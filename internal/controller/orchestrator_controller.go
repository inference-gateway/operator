/*
Copyright (c) 2025 Inference Gateway

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	handler "sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

// OrchestratorReconciler reconciles an Orchestrator object.
type OrchestratorReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=orchestrators,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=orchestrators/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=orchestrators/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=agents,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=mcps,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile drives an Orchestrator resource toward its desired state.
//
// The controller manages a singleton Deployment (replicas=1, strategy=Recreate)
// that runs the Inference Gateway CLI's `infer channels-manager` daemon.
// No Service is created because the orchestrator is outbound-only (Telegram long-poll).
// When spec.a2a.serviceDiscovery.enabled is true, the controller also discovers Agent CRs
// matching the configured selector and writes them into a ConfigMap mounted as
// ~/.infer/agents.yaml inside the orchestrator pod for hot-reload without restarts.
func (r *OrchestratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var orch v1alpha1.Orchestrator
	if err := r.Get(ctx, req.NamespacedName, &orch); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !orch.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Reconcile the agents ConfigMap (handles both static and discovered agents).
	discoveredAgentURLs, agentsYAML, err := r.reconcileAgentsConfigMap(ctx, &orch)
	if err != nil {
		if apiErrors.IsConflict(err) {
			logger.V(1).Info("agents configmap reconciliation conflict, requeueing", "error", err)
			return ctrl.Result{RequeueAfter: time.Second * 1}, nil
		}
		logger.Error(err, "failed to reconcile agents configmap")
		return ctrl.Result{}, err
	}

	// Reconcile the MCPs ConfigMap (handles both static and discovered MCP servers).
	discoveredMCPURLs, mcpYAML, err := r.reconcileMCPsConfigMap(ctx, &orch)
	if err != nil {
		if apiErrors.IsConflict(err) {
			logger.V(1).Info("mcps configmap reconciliation conflict, requeueing", "error", err)
			return ctrl.Result{RequeueAfter: time.Second * 1}, nil
		}
		logger.Error(err, "failed to reconcile mcps configmap")
		return ctrl.Result{}, err
	}

	deployment, err := r.reconcileDeployment(ctx, &orch, agentsYAML, mcpYAML)
	if err != nil {
		if apiErrors.IsConflict(err) {
			logger.V(1).Info("deployment reconciliation conflict, requeueing", "error", err)
			return ctrl.Result{RequeueAfter: time.Second * 1}, nil
		}
		logger.Error(err, "failed to reconcile deployment")
		return ctrl.Result{}, err
	}

	if err := r.updateStatus(ctx, &orch, deployment, discoveredAgentURLs, discoveredMCPURLs); err != nil {
		logger.Error(err, "failed to update orchestrator status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileAgentsConfigMap discovers Agent CRs matching the service discovery selector,
// builds an agents.yaml combining static and discovered agents, and writes it to a ConfigMap
// owned by the Orchestrator. Returns the sorted list of discovered agent URLs and the
// rendered agents.yaml content (used to stamp a hash annotation on the Deployment so the
// pod is rolled when the content changes — required because the ConfigMap is mounted with
// subPath, which Kubernetes does not propagate live updates for).
//
// Note on INFER_A2A_AGENTS: the env var is retained for backward compatibility when
// service discovery is disabled. When service discovery is enabled the ConfigMap mount
// is the source of truth for all agents (static + discovered), so INFER_A2A_AGENTS
// becomes redundant; it will be removed in a future version.
func (r *OrchestratorReconciler) reconcileAgentsConfigMap(ctx context.Context, orch *v1alpha1.Orchestrator) ([]string, string, error) {
	logger := logf.FromContext(ctx)

	if !orch.Spec.A2A.Enabled || !orch.Spec.A2A.ServiceDiscovery.Enabled {
		return nil, "", nil
	}

	// Discover matching Agent CRs.
	discoveredAgents, err := r.discoverAgents(ctx, orch)
	if err != nil {
		return nil, "", fmt.Errorf("failed to discover agents: %w", err)
	}

	// Derive sorted, unique URL list for status.
	discoveredURLs := make([]string, 0, len(discoveredAgents))
	for _, agent := range discoveredAgents {
		port := agent.Spec.Port
		if port == 0 {
			port = 8080
		}
		url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", agent.Name, agent.Namespace, port)
		discoveredURLs = append(discoveredURLs, url)
	}
	sort.Strings(discoveredURLs)

	// Build agents.yaml content.
	agentsYAML := buildAgentsYAML(orch.Spec.A2A.Agents, discoveredAgents)

	// Create or update the ConfigMap.
	cmName := orch.Name + "-agents"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: orch.Namespace,
			Labels:    map[string]string{"app": orch.Name},
		},
		Data: map[string]string{
			"agents.yaml": agentsYAML,
		},
	}

	if err := controllerutil.SetControllerReference(orch, cm, r.Scheme); err != nil {
		return nil, "", err
	}

	found := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: orch.Namespace}, found)
	if err != nil && apiErrors.IsNotFound(err) {
		logger.Info("creating agents configmap", "ConfigMap.Name", cmName)
		if err = r.Create(ctx, cm); err != nil {
			return nil, "", err
		}
		return discoveredURLs, agentsYAML, nil
	} else if err != nil {
		return nil, "", err
	}

	// Update only when content changes to avoid spurious updates.
	if found.Data == nil || found.Data["agents.yaml"] != agentsYAML {
		found.Data = cm.Data
		logger.Info("updating agents configmap", "ConfigMap.Name", cmName, "agentCount", len(discoveredAgents))
		if err = r.Update(ctx, found); err != nil {
			return nil, "", err
		}
	}

	return discoveredURLs, agentsYAML, nil
}

// discoverAgents lists Agent CRs in the configured namespace filtered by the label selector.
func (r *OrchestratorReconciler) discoverAgents(ctx context.Context, orch *v1alpha1.Orchestrator) ([]v1alpha1.Agent, error) {
	ns := orch.Spec.A2A.ServiceDiscovery.Namespace
	if ns == "" {
		ns = orch.Namespace
	}

	listOpts := []client.ListOption{client.InNamespace(ns)}

	if orch.Spec.A2A.ServiceDiscovery.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(orch.Spec.A2A.ServiceDiscovery.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %w", err)
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: selector})
	}

	var agentList v1alpha1.AgentList
	if err := r.List(ctx, &agentList, listOpts...); err != nil {
		return nil, err
	}

	return agentList.Items, nil
}

// buildAgentsYAML constructs an agents.yaml document combining static (from spec.a2a.agents)
// and discovered Agent CRs. Discovered entries take precedence on URL collision.
// The format matches the CLI's AgentsConfig schema.
func buildAgentsYAML(staticAgents []string, discoveredAgents []v1alpha1.Agent) string {
	var sb strings.Builder
	sb.WriteString("agents:\n")

	// Static agents with synthetic names (kept for backward compat).
	for i, url := range staticAgents {
		fmt.Fprintf(&sb, "  - name: static-agent-%d\n", i)
		fmt.Fprintf(&sb, "    url: %s\n", url)
		sb.WriteString("    enabled: true\n")
		sb.WriteString("    run: false\n")
	}

	// Discovered agents, sorted by name for determinism.
	sorted := make([]v1alpha1.Agent, len(discoveredAgents))
	copy(sorted, discoveredAgents)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	for _, agent := range sorted {
		port := agent.Spec.Port
		if port == 0 {
			port = 8080
		}
		url := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", agent.Name, agent.Namespace, port)
		fmt.Fprintf(&sb, "  - name: %s\n", agent.Name)
		fmt.Fprintf(&sb, "    url: %s\n", url)
		sb.WriteString("    enabled: true\n")
		sb.WriteString("    run: false\n")
	}

	return sb.String()
}

// reconcileMCPsConfigMap discovers MCP CRs matching the service discovery selector,
// builds an mcp.yaml combining static and discovered MCP servers, and writes it to a ConfigMap
// owned by the Orchestrator. Returns the sorted list of discovered MCP URLs and the
// rendered mcp.yaml content (used to stamp a hash annotation on the Deployment so the
// pod is rolled when the content changes — required because the ConfigMap is mounted with
// subPath, which Kubernetes does not propagate live updates for).
func (r *OrchestratorReconciler) reconcileMCPsConfigMap(ctx context.Context, orch *v1alpha1.Orchestrator) ([]string, string, error) {
	logger := logf.FromContext(ctx)

	if !orch.Spec.MCP.Enabled || !orch.Spec.MCP.ServiceDiscovery.Enabled {
		return nil, "", nil
	}

	discoveredMCPs, err := r.discoverMCPs(ctx, orch)
	if err != nil {
		return nil, "", fmt.Errorf("failed to discover mcps: %w", err)
	}

	discoveredURLs := make([]string, 0, len(discoveredMCPs))
	for _, mcp := range discoveredMCPs {
		discoveredURLs = append(discoveredURLs, mcpURL(&mcp))
	}
	sort.Strings(discoveredURLs)

	mcpYAML := buildMCPsYAML(orch.Spec.MCP.Servers, discoveredMCPs)

	cmName := orch.Name + "-mcps"
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cmName,
			Namespace: orch.Namespace,
			Labels:    map[string]string{"app": orch.Name},
		},
		Data: map[string]string{
			"mcp.yaml": mcpYAML,
		},
	}

	if err := controllerutil.SetControllerReference(orch, cm, r.Scheme); err != nil {
		return nil, "", err
	}

	found := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: cmName, Namespace: orch.Namespace}, found)
	if err != nil && apiErrors.IsNotFound(err) {
		logger.Info("creating mcps configmap", "ConfigMap.Name", cmName)
		if err = r.Create(ctx, cm); err != nil {
			return nil, "", err
		}
		return discoveredURLs, mcpYAML, nil
	} else if err != nil {
		return nil, "", err
	}

	if found.Data == nil || found.Data["mcp.yaml"] != mcpYAML {
		found.Data = cm.Data
		logger.Info("updating mcps configmap", "ConfigMap.Name", cmName, "mcpCount", len(discoveredMCPs))
		if err = r.Update(ctx, found); err != nil {
			return nil, "", err
		}
	}

	return discoveredURLs, mcpYAML, nil
}

// discoverMCPs lists MCP CRs in the configured namespace filtered by the label selector.
func (r *OrchestratorReconciler) discoverMCPs(ctx context.Context, orch *v1alpha1.Orchestrator) ([]v1alpha1.MCP, error) {
	ns := orch.Spec.MCP.ServiceDiscovery.Namespace
	if ns == "" {
		ns = orch.Namespace
	}

	listOpts := []client.ListOption{client.InNamespace(ns)}

	if orch.Spec.MCP.ServiceDiscovery.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(orch.Spec.MCP.ServiceDiscovery.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %w", err)
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: selector})
	}

	var mcpList v1alpha1.MCPList
	if err := r.List(ctx, &mcpList, listOpts...); err != nil {
		return nil, err
	}

	return mcpList.Items, nil
}

// buildMCPsYAML constructs an mcp.yaml document combining static and discovered MCP servers.
// Discovered entries are sorted by name for determinism.
func buildMCPsYAML(staticServers []string, discoveredMCPs []v1alpha1.MCP) string {
	var sb strings.Builder
	sb.WriteString("mcpServers:\n")

	for i, url := range staticServers {
		fmt.Fprintf(&sb, "  - name: static-mcp-%d\n", i)
		fmt.Fprintf(&sb, "    url: %s\n", url)
		sb.WriteString("    enabled: true\n")
	}

	sorted := make([]v1alpha1.MCP, len(discoveredMCPs))
	copy(sorted, discoveredMCPs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	for _, mcp := range sorted {
		fmt.Fprintf(&sb, "  - name: %s\n", mcp.Name)
		fmt.Fprintf(&sb, "    url: %s\n", mcpURL(&mcp))
		sb.WriteString("    enabled: true\n")
	}

	return sb.String()
}

// mcpURL returns the URL for an MCP CR. It prefers Status.URL (already TLS-aware,
// populated by the MCP controller) and falls back to a deterministic construction
// when status has not been populated yet.
func mcpURL(mcp *v1alpha1.MCP) string {
	if mcp.Status.URL != "" {
		return mcp.Status.URL
	}
	scheme := "http"
	var port int32 = 8080
	if mcp.Spec.Server != nil {
		if mcp.Spec.Server.Port != 0 {
			port = mcp.Spec.Server.Port
		}
		if mcp.Spec.Server.TLS != nil && mcp.Spec.Server.TLS.Enabled {
			scheme = "https"
		}
	}
	return fmt.Sprintf("%s://%s-service.%s.svc.cluster.local:%d", scheme, mcp.Name, mcp.Namespace, port)
}

// reconcileDeployment ensures the Orchestrator's singleton Deployment exists and matches the spec.
// agentsYAML and mcpYAML are the rendered ConfigMap contents; their hashes are stamped as
// pod template annotations so the Deployment rolls when the contents change (subPath mounts
// do not propagate updates).
func (r *OrchestratorReconciler) reconcileDeployment(ctx context.Context, orch *v1alpha1.Orchestrator, agentsYAML, mcpYAML string) (*appsv1.Deployment, error) {
	deployment := r.buildOrchestratorDeployment(orch, agentsYAML, mcpYAML)

	if err := controllerutil.SetControllerReference(orch, deployment, r.Scheme); err != nil {
		return nil, err
	}

	return r.createOrUpdateOrchestratorDeployment(ctx, deployment)
}

// buildOrchestratorDeployment returns a singleton Deployment for the given Orchestrator.
// When service discovery is enabled the agents/mcps ConfigMaps are mounted at
// /home/infer/.infer/agents.yaml and /home/infer/.infer/mcp.yaml so the CLI picks up the
// discovered sets on each invocation. The hashes of the YAML contents are stamped as
// pod template annotations so the Deployment rolls when either set changes —
// Kubernetes does not propagate live updates to ConfigMap volumes that use subPath.
func (r *OrchestratorReconciler) buildOrchestratorDeployment(orch *v1alpha1.Orchestrator, agentsYAML, mcpYAML string) *appsv1.Deployment {
	orchLabels := map[string]string{"app": orch.Name}

	container := corev1.Container{
		Name:      "orchestrator",
		Image:     orch.Spec.Image,
		Command:   []string{"infer"},
		Args:      []string{"channels-manager"},
		Env:       buildOrchestratorEnvironmentVars(orch),
		Resources: orch.Spec.Resources,
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}

	podAnnotations := map[string]string{}

	// Mount the agents ConfigMap when service discovery is enabled.
	if orch.Spec.A2A.Enabled && orch.Spec.A2A.ServiceDiscovery.Enabled {
		cmName := orch.Name + "-agents"
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "agents-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		})
		podSpec.Containers[0].VolumeMounts = append(
			podSpec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "agents-config",
				MountPath: "/home/infer/.infer/agents.yaml",
				SubPath:   "agents.yaml",
			},
		)
		sum := sha256.Sum256([]byte(agentsYAML))
		podAnnotations["inference-gateway.com/agents-config-hash"] = hex.EncodeToString(sum[:])
	}

	// Mount the mcps ConfigMap when MCP service discovery is enabled.
	if orch.Spec.MCP.Enabled && orch.Spec.MCP.ServiceDiscovery.Enabled {
		cmName := orch.Name + "-mcps"
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "mcps-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: cmName},
				},
			},
		})
		podSpec.Containers[0].VolumeMounts = append(
			podSpec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "mcps-config",
				MountPath: "/home/infer/.infer/mcp.yaml",
				SubPath:   "mcp.yaml",
			},
		)
		sum := sha256.Sum256([]byte(mcpYAML))
		podAnnotations["inference-gateway.com/mcps-config-hash"] = hex.EncodeToString(sum[:])
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      orch.Name,
			Namespace: orch.Namespace,
			Labels:    orchLabels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{MatchLabels: orchLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: orchLabels, Annotations: podAnnotations},
				Spec:       podSpec,
			},
		},
	}
}

// buildOrchestratorEnvironmentVars translates OrchestratorSpec into INFER_* environment variables
// per the spec ↔ CLI config mapping documented in the Orchestrator CRD.
//
// Note: INFER_A2A_AGENTS is retained for backward compatibility. When service discovery is
// enabled, the agents ConfigMap mount (~/.infer/agents.yaml) is the source of truth for all
// agents (static + discovered). INFER_A2A_AGENTS will be removed in a future release.
func buildOrchestratorEnvironmentVars(orch *v1alpha1.Orchestrator) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	if orch.Spec.Env != nil {
		envVars = append(envVars, *orch.Spec.Env...)
	}

	envVars = append(envVars,
		corev1.EnvVar{Name: "INFER_CHANNELS_ENABLED", Value: "true"},
		corev1.EnvVar{Name: "INFER_LOGGING_STDOUT", Value: "true"},
		corev1.EnvVar{Name: "INFER_GATEWAY_RUN", Value: "false"},
	)

	if orch.Spec.Channels.MaxWorkers != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "INFER_CHANNELS_MAX_WORKERS",
			Value: strconv.Itoa(int(*orch.Spec.Channels.MaxWorkers)),
		})
	}
	if orch.Spec.Channels.ImageRetention != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "INFER_CHANNELS_IMAGE_RETENTION",
			Value: strconv.Itoa(int(*orch.Spec.Channels.ImageRetention)),
		})
	}
	if orch.Spec.Channels.RequireApproval != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "INFER_CHANNELS_REQUIRE_APPROVAL",
			Value: strconv.FormatBool(*orch.Spec.Channels.RequireApproval),
		})
	}

	tg := orch.Spec.Channels.Telegram
	envVars = append(envVars,
		corev1.EnvVar{
			Name:  "INFER_CHANNELS_TELEGRAM_ENABLED",
			Value: strconv.FormatBool(tg.Enabled),
		},
		corev1.EnvVar{
			Name: "INFER_CHANNELS_TELEGRAM_BOT_TOKEN",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: tg.TokenSecretRef.DeepCopy(),
			},
		},
	)
	if tg.AllowedUsersSecretRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "INFER_CHANNELS_TELEGRAM_ALLOWED_USERS",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: tg.AllowedUsersSecretRef.DeepCopy(),
			},
		})
	}
	if tg.PollTimeout != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "INFER_CHANNELS_TELEGRAM_POLL_TIMEOUT",
			Value: strconv.Itoa(int(tg.PollTimeout.Seconds())),
		})
	}

	envVars = append(envVars, corev1.EnvVar{
		Name:  "INFER_GATEWAY_URL",
		Value: orch.Spec.Gateway.URL,
	})
	if orch.Spec.Gateway.APIKeySecretRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "INFER_GATEWAY_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: orch.Spec.Gateway.APIKeySecretRef.DeepCopy(),
			},
		})
	}

	envVars = append(envVars,
		corev1.EnvVar{Name: "INFER_AGENT_MODEL", Value: orch.Spec.Agent.Model},
		corev1.EnvVar{Name: "INFER_AGENT_SYSTEM_PROMPT", Value: orch.Spec.Agent.SystemPrompt},
		corev1.EnvVar{
			Name:  "INFER_TOOLS_ENABLED",
			Value: strconv.FormatBool(orch.Spec.Tools.Enabled),
		},
		corev1.EnvVar{
			Name:  "INFER_TOOLS_SCHEDULE_ENABLED",
			Value: strconv.FormatBool(orch.Spec.Tools.Schedule),
		},
		corev1.EnvVar{
			Name:  "INFER_A2A_ENABLED",
			Value: strconv.FormatBool(orch.Spec.A2A.Enabled),
		},
		corev1.EnvVar{
			Name:  "INFER_A2A_AGENTS",
			Value: strings.Join(orch.Spec.A2A.Agents, ","),
		},
	)

	return envVars
}

// createOrUpdateOrchestratorDeployment creates the Deployment if missing, otherwise reconciles drift.
func (r *OrchestratorReconciler) createOrUpdateOrchestratorDeployment(ctx context.Context, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && apiErrors.IsNotFound(err) {
		logger.Info("creating orchestrator deployment", "Deployment.Name", deployment.Name)
		if err = r.Create(ctx, deployment); err != nil {
			return nil, err
		}
		return deployment, nil
	} else if err != nil {
		return nil, err
	}

	return r.updateOrchestratorDeploymentIfNeeded(ctx, deployment, found)
}

// updateOrchestratorDeploymentIfNeeded reconciles drift on replicas, strategy, selector, and pod template.
func (r *OrchestratorReconciler) updateOrchestratorDeploymentIfNeeded(ctx context.Context, desired, found *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)

	for retries := 0; retries < 3; retries++ {
		latest := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: found.Name, Namespace: found.Namespace}, latest); err != nil {
			return nil, err
		}

		needsUpdate := false
		var changes []string

		desiredReplicas := int32(1)
		if latest.Spec.Replicas == nil || *latest.Spec.Replicas != desiredReplicas {
			latest.Spec.Replicas = &desiredReplicas
			needsUpdate = true
			changes = append(changes, "replicas")
		}

		if latest.Spec.Strategy.Type != appsv1.RecreateDeploymentStrategyType {
			latest.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
			needsUpdate = true
			changes = append(changes, "strategy")
		}

		desiredTemplate := desired.Spec.Template.DeepCopy()
		if desiredTemplate.Annotations == nil {
			desiredTemplate.Annotations = map[string]string{}
		}
		for k, v := range latest.Spec.Template.Annotations {
			if k == "kubectl.kubernetes.io/restartedAt" || k == "deployment.kubernetes.io/revision" {
				desiredTemplate.Annotations[k] = v
			}
		}

		if !reflect.DeepEqual(latest.Spec.Template, *desiredTemplate) {
			latest.Spec.Template = *desiredTemplate
			needsUpdate = true
			changes = append(changes, "pod template")
		}

		if !reflect.DeepEqual(latest.Spec.Selector, desired.Spec.Selector) {
			latest.Spec.Selector = desired.Spec.Selector
			needsUpdate = true
			changes = append(changes, "selector")
		}

		if !needsUpdate {
			return latest, nil
		}

		logger.Info("updating orchestrator deployment", "Deployment.Name", desired.Name, "changes", changes)
		if err := r.Update(ctx, latest); err != nil {
			if apiErrors.IsConflict(err) && retries < 2 {
				logger.Info("deployment update conflict, retrying", "retry", retries+1)
				time.Sleep(time.Millisecond * 100)
				continue
			}
			return nil, err
		}
		return latest, nil
	}

	return nil, fmt.Errorf("failed to update orchestrator deployment after 3 retries due to conflicts")
}

// updateStatus reflects Deployment availability and discovered agents/MCPs into Orchestrator status.
func (r *OrchestratorReconciler) updateStatus(ctx context.Context, orch *v1alpha1.Orchestrator, deployment *appsv1.Deployment, discoveredAgentURLs, discoveredMCPURLs []string) error {
	patch := client.MergeFrom(orch.DeepCopy())

	ready := deployment.Status.AvailableReplicas >= 1
	orch.Status.Ready = ready
	orch.Status.ObservedGeneration = orch.Generation
	orch.Status.DiscoveredAgents = discoveredAgentURLs
	orch.Status.DiscoveredAgentCount = int32(len(discoveredAgentURLs))
	orch.Status.DiscoveredMCPs = discoveredMCPURLs
	orch.Status.DiscoveredMCPCount = int32(len(discoveredMCPURLs))

	readyCondition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "DeploymentNotAvailable",
		Message:            "orchestrator deployment has no available replicas yet",
		ObservedGeneration: orch.Generation,
		LastTransitionTime: metav1.Now(),
	}
	if ready {
		readyCondition.Status = metav1.ConditionTrue
		readyCondition.Reason = "DeploymentAvailable"
		readyCondition.Message = "orchestrator deployment is available"
	}
	setCondition(&orch.Status.Conditions, readyCondition)

	// Reflect the outcome of the last agent discovery pass.
	if orch.Spec.A2A.Enabled && orch.Spec.A2A.ServiceDiscovery.Enabled {
		discoveredCondition := metav1.Condition{
			Type:               "Discovered",
			Status:             metav1.ConditionTrue,
			Reason:             "AgentsDiscovered",
			Message:            fmt.Sprintf("discovered %d agent(s)", len(discoveredAgentURLs)),
			ObservedGeneration: orch.Generation,
			LastTransitionTime: metav1.Now(),
		}
		setCondition(&orch.Status.Conditions, discoveredCondition)
	}

	// Reflect the outcome of the last MCP discovery pass.
	if orch.Spec.MCP.Enabled && orch.Spec.MCP.ServiceDiscovery.Enabled {
		discoveredMCPCondition := metav1.Condition{
			Type:               "MCPsDiscovered",
			Status:             metav1.ConditionTrue,
			Reason:             "MCPsDiscovered",
			Message:            fmt.Sprintf("discovered %d mcp(s)", len(discoveredMCPURLs)),
			ObservedGeneration: orch.Generation,
			LastTransitionTime: metav1.Now(),
		}
		setCondition(&orch.Status.Conditions, discoveredMCPCondition)
	}

	return r.Status().Patch(ctx, orch, patch)
}

// setCondition upserts a condition by Type, preserving LastTransitionTime when Status is unchanged.
func setCondition(conditions *[]metav1.Condition, newCond metav1.Condition) {
	for i, c := range *conditions {
		if c.Type != newCond.Type {
			continue
		}
		if c.Status == newCond.Status {
			newCond.LastTransitionTime = c.LastTransitionTime
		}
		(*conditions)[i] = newCond
		return
	}
	*conditions = append(*conditions, newCond)
}

// agentToOrchestratorRequests maps an Agent event to the set of Orchestrator reconcile requests
// whose service discovery configuration selects that Agent.
//
// hurt readability for marginal LOC savings.
//
//nolint:dupl // structurally parallel to mcpToOrchestratorRequests; merging via generics would
func (r *OrchestratorReconciler) agentToOrchestratorRequests(ctx context.Context, obj client.Object) []ctrl.Request {
	agent, ok := obj.(*v1alpha1.Agent)
	if !ok {
		return nil
	}

	var orchList v1alpha1.OrchestratorList
	if err := r.List(ctx, &orchList); err != nil {
		return nil
	}

	var requests []ctrl.Request
	for _, orch := range orchList.Items {
		if !orch.Spec.A2A.Enabled || !orch.Spec.A2A.ServiceDiscovery.Enabled {
			continue
		}

		ns := orch.Spec.A2A.ServiceDiscovery.Namespace
		if ns == "" {
			ns = orch.Namespace
		}

		if ns != agent.Namespace {
			continue
		}

		// nil / empty selector matches all Agents in the namespace.
		if orch.Spec.A2A.ServiceDiscovery.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(orch.Spec.A2A.ServiceDiscovery.Selector)
			if err != nil {
				continue
			}
			if !selector.Matches(labels.Set(agent.Labels)) {
				continue
			}
		}

		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      orch.Name,
				Namespace: orch.Namespace,
			},
		})
	}
	return requests
}

// mcpToOrchestratorRequests maps an MCP event to the set of Orchestrator reconcile requests
// whose MCP service discovery configuration selects that MCP.
//
// hurt readability for marginal LOC savings.
//
//nolint:dupl // structurally parallel to agentToOrchestratorRequests; merging via generics would
func (r *OrchestratorReconciler) mcpToOrchestratorRequests(ctx context.Context, obj client.Object) []ctrl.Request {
	mcp, ok := obj.(*v1alpha1.MCP)
	if !ok {
		return nil
	}

	var orchList v1alpha1.OrchestratorList
	if err := r.List(ctx, &orchList); err != nil {
		return nil
	}

	var requests []ctrl.Request
	for _, orch := range orchList.Items {
		if !orch.Spec.MCP.Enabled || !orch.Spec.MCP.ServiceDiscovery.Enabled {
			continue
		}

		ns := orch.Spec.MCP.ServiceDiscovery.Namespace
		if ns == "" {
			ns = orch.Namespace
		}

		if ns != mcp.Namespace {
			continue
		}

		if orch.Spec.MCP.ServiceDiscovery.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(orch.Spec.MCP.ServiceDiscovery.Selector)
			if err != nil {
				continue
			}
			if !selector.Matches(labels.Set(mcp.Labels)) {
				continue
			}
		}

		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      orch.Name,
				Namespace: orch.Namespace,
			},
		})
	}
	return requests
}

// SetupWithManager registers the Orchestrator controller with the manager.
// It watches Agent and MCP CRs across all namespaces and triggers Orchestrator
// reconciliations when one matching an Orchestrator's service discovery selector changes.
func (r *OrchestratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Orchestrator{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Watches(
			&v1alpha1.Agent{},
			handler.EnqueueRequestsFromMapFunc(r.agentToOrchestratorRequests),
		).
		Watches(
			&v1alpha1.MCP{},
			handler.EnqueueRequestsFromMapFunc(r.mcpToOrchestratorRequests),
		).
		Named("orchestrator").
		Complete(r)
}
