/*
Copyright 2026 Inference Gateway

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
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
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

// AgentReconciler reconciles a Agent object
type AgentReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=agents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=agents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=agents/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *AgentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	if !r.shouldWatchNamespace(ctx, req.Namespace) {
		logger.V(1).Info("Skipping Agent in namespace not matching watch criteria", "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	var agent v1alpha1.Agent
	if err := r.Get(ctx, req.NamespacedName, &agent); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !agent.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	_, err := r.reconcileDeployment(ctx, &agent)
	if err != nil {
		if apiErrors.IsConflict(err) {
			logger.V(1).Info("Deployment reconciliation conflict, requeueing", "error", err)
			return ctrl.Result{RequeueAfter: time.Second * 1}, nil
		}
		logger.Error(err, "Failed to reconcile Deployment")
		return ctrl.Result{}, err
	}

	svc := &corev1.Service{}
	svcName := agent.Name
	err = r.Get(ctx, client.ObjectKey{Namespace: agent.Namespace, Name: svcName}, svc)
	if err != nil {
		svc = buildAgentService(&agent)
		if err := r.Create(ctx, svc); err != nil {
			logger.Error(err, "failed to create service")
			return ctrl.Result{}, err
		}
		logger.Info("created service", "name", svcName)
	}

	card, fetchErr := fetchAgentCard(&agent, svc)
	if fetchErr != nil {
		logger.Info("failed to fetch agent card, will retry", "error", fetchErr.Error())
		if statusErr := r.patchStatusNotReady(ctx, &agent, fetchErr); statusErr != nil {
			logger.Error(statusErr, "unable to update agent status with fetch failure")
		}
		return ctrl.Result{RequeueAfter: agentCardRetryInterval}, nil
	}
	card.SkillsNames = card.Skills.SkillsNames()

	if statusErr := r.patchStatusReady(ctx, &agent, card); statusErr != nil {
		logger.Error(statusErr, "unable to update agent status.card")
		return ctrl.Result{}, statusErr
	}
	logger.Info("updated agent status.card", "version", card.Version)
	return ctrl.Result{RequeueAfter: agentCardRefreshInterval}, nil
}

// patchStatusReady writes the Agent status with the fetched card and a Ready=True condition.
// Uses Status().Update rather than a merge patch so that false-valued bool fields (e.g.
// capabilities.pushNotifications) actually land on the API object: a JSON merge patch
// would diff the in-memory base (zero-valued bools marshal as false) against the modified
// object (same false), produce an empty diff for those fields, and leave them absent from
// the stored status — meaning printer columns sourced from them render blank.
func (r *AgentReconciler) patchStatusReady(ctx context.Context, agent *v1alpha1.Agent, card *v1alpha1.Card) error {
	agent.Status.Card = *card
	agent.Status.Ready = true
	agent.Status.ObservedGeneration = agent.Generation
	setReadyCondition(&agent.Status.Conditions, metav1.ConditionTrue, "CardFetched", "agent card retrieved successfully")
	return r.Status().Update(ctx, agent)
}

// patchStatusNotReady writes the Agent status with a Ready=False condition explaining
// why the card could not be fetched. The previously cached card (if any) is preserved.
func (r *AgentReconciler) patchStatusNotReady(ctx context.Context, agent *v1alpha1.Agent, fetchErr error) error {
	agent.Status.Ready = false
	agent.Status.ObservedGeneration = agent.Generation
	setReadyCondition(&agent.Status.Conditions, metav1.ConditionFalse, "CardFetchFailed", fetchErr.Error())
	return r.Status().Update(ctx, agent)
}

// setReadyCondition upserts a "Ready" condition on the slice.
func setReadyCondition(conditions *[]metav1.Condition, status metav1.ConditionStatus, reason, message string) {
	now := metav1.Now()
	for i := range *conditions {
		if (*conditions)[i].Type != "Ready" {
			continue
		}
		if (*conditions)[i].Status != status {
			(*conditions)[i].LastTransitionTime = now
		}
		(*conditions)[i].Status = status
		(*conditions)[i].Reason = reason
		(*conditions)[i].Message = message
		return
	}
	*conditions = append(*conditions, metav1.Condition{
		Type:               "Ready",
		Status:             status,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: now,
	})
}

// buildAgentService returns a Service for the given Agent resource.
func buildAgentService(agent *v1alpha1.Agent) *corev1.Service {
	labels := map[string]string{
		"app": agent.Name,
	}
	port := agent.Spec.Port
	if port <= 0 {
		port = defaultAgentPort
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agent.Name,
			Namespace: agent.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       port,
				TargetPort: intstrFromInt(int(port)),
			}},
		},
	}
}

// agentCardPaths are the well-known paths probed in order to retrieve the agent card.
// The first entry is the current A2A protocol path; the second is the legacy fallback.
var agentCardPaths = []string{
	"/.well-known/agent-card.json",
	"/.well-known/agent.json",
}

const (
	// agentCardFetchTimeout bounds a single agent card HTTP request.
	agentCardFetchTimeout = 5 * time.Second

	// agentCardRetryInterval is how soon to retry after a failed card fetch.
	agentCardRetryInterval = 5 * time.Second

	// agentCardRefreshInterval is how often a successfully fetched card is re-validated.
	agentCardRefreshInterval = 30 * time.Second

	// defaultAgentPort is the port used when an agent's spec.port is unset.
	defaultAgentPort = int32(8080)
)

// agentAdvertisedURL returns the URL the agent should report in its agent-card.
// spec.card.url wins when set, otherwise it's the in-cluster Service URL.
func agentAdvertisedURL(agent *v1alpha1.Agent) string {
	if agent.Spec.Card.URL != "" {
		return agent.Spec.Card.URL
	}
	port := agent.Spec.Port
	if port <= 0 {
		port = defaultAgentPort
	}
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", agent.Name, agent.Namespace, port)
}

// agentCardPort returns the port to query for the agent's well-known card, preferring
// the agent spec but falling back to the service port, then the package default.
func agentCardPort(agent *v1alpha1.Agent, svc *corev1.Service) int32 {
	if agent != nil && agent.Spec.Port > 0 {
		return agent.Spec.Port
	}
	if svc != nil {
		for _, p := range svc.Spec.Ports {
			if p.Port > 0 {
				return p.Port
			}
		}
	}
	return defaultAgentPort
}

// agentCardURLs returns the ordered list of URLs to probe for the agent card.
// Exposed as a separate function so unit tests can verify URL construction.
func agentCardURLs(agent *v1alpha1.Agent, svc *corev1.Service) []string {
	if svc == nil {
		return nil
	}
	port := agentCardPort(agent, svc)
	base := fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", svc.Name, svc.Namespace, port)
	urls := make([]string, 0, len(agentCardPaths))
	for _, path := range agentCardPaths {
		urls = append(urls, base+path)
	}
	return urls
}

// fetchAgentCard retrieves the agent card by probing the well-known paths on the
// agent's service. It tries the current A2A path first and falls back to the
// legacy path. A non-2xx response on a probe is treated as a miss for that path.
func fetchAgentCard(agent *v1alpha1.Agent, svc *corev1.Service) (*v1alpha1.Card, error) {
	if svc == nil {
		return nil, errors.New("service is nil")
	}
	httpClient := &http.Client{Timeout: agentCardFetchTimeout}

	var lastErr error
	for _, url := range agentCardURLs(agent, svc) {
		card, err := getAgentCard(httpClient, url)
		if err == nil {
			return card, nil
		}
		lastErr = fmt.Errorf("%s: %w", url, err)
	}
	return nil, lastErr
}

// getAgentCard performs a single HTTP GET and JSON-decodes the response into a Card.
func getAgentCard(httpClient *http.Client, url string) (*v1alpha1.Card, error) {
	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("unexpected status: " + resp.Status)
	}
	var card v1alpha1.Card
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, err
	}
	return &card, nil
}

// reconcileDeployment ensures the Deployment exists with the correct configuration
func (r *AgentReconciler) reconcileDeployment(ctx context.Context, agent *v1alpha1.Agent) (*appsv1.Deployment, error) {
	deployment := r.buildAgentDeployment(agent)

	if err := controllerutil.SetControllerReference(agent, deployment, r.Scheme); err != nil {
		return nil, err
	}

	return r.createOrUpdateDeployment(ctx, deployment)
}

// buildAgentDeployment returns a Deployment for the given Agent resource with comprehensive configuration.
func (r *AgentReconciler) buildAgentDeployment(agent *v1alpha1.Agent) *appsv1.Deployment {
	labels := map[string]string{
		"app": agent.Name,
	}

	env := r.buildAgentEnvironmentVars(agent)

	port := defaultAgentPort
	if agent.Spec.Port > 0 {
		port = agent.Spec.Port
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      agent.Name,
			Namespace: agent.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Env:       env,
						Name:      "agent",
						Image:     agent.Spec.Image,
						Resources: agent.Spec.Resources,
						Ports: []corev1.ContainerPort{{
							ContainerPort: port,
						}},
					}},
				},
			},
		},
	}
}

// buildAgentEnvironmentVars creates environment variables from Agent spec.
// LLM-related vars use the A2A_AGENT_CLIENT_* prefix that agent images actually read.
func (r *AgentReconciler) buildAgentEnvironmentVars(agent *v1alpha1.Agent) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	// User-supplied env vars take the lowest precedence (prepended so operator vars win on conflict).
	if agent.Spec.Env != nil {
		envVars = append(envVars, *agent.Spec.Env...)
	}

	// Server / infrastructure vars
	envVars = append(envVars,
		corev1.EnvVar{Name: "TIMEZONE", Value: agent.Spec.Timezone},
		corev1.EnvVar{Name: "PORT", Value: strconv.Itoa(int(agent.Spec.Port))},
		corev1.EnvVar{Name: "HOST", Value: agent.Spec.Host},
		// A2A_AGENT_URL is what the agent reports as its own URL in /.well-known/agent-card.json.
		// The agent can't derive this itself, so the operator supplies a URL: spec.card.url
		// takes precedence (for agents fronted by an Ingress/Gateway), falling back to the
		// in-cluster Service URL.
		corev1.EnvVar{Name: "A2A_AGENT_URL", Value: agentAdvertisedURL(agent)},
		corev1.EnvVar{Name: "READ_TIMEOUT", Value: agent.Spec.ReadTimeout},
		corev1.EnvVar{Name: "WRITE_TIMEOUT", Value: agent.Spec.WriteTimeout},
		corev1.EnvVar{Name: "IDLE_TIMEOUT", Value: agent.Spec.IdleTimeout},
		// Logging
		corev1.EnvVar{Name: "LOG_LEVEL", Value: agent.Spec.Logging.Level},
		corev1.EnvVar{Name: "LOG_FORMAT", Value: agent.Spec.Logging.Format},
		// Telemetry
		corev1.EnvVar{Name: "TELEMETRY_ENABLED", Value: strconv.FormatBool(agent.Spec.Telemetry.Enabled)},
		// Queue
		corev1.EnvVar{Name: "QUEUE_ENABLED", Value: strconv.FormatBool(agent.Spec.Queue.Enabled)},
		corev1.EnvVar{Name: "QUEUE_MAX_SIZE", Value: strconv.Itoa(int(agent.Spec.Queue.MaxSize))},
		corev1.EnvVar{Name: "QUEUE_CLEANUP_INTERVAL", Value: agent.Spec.Queue.CleanupInterval},
		// TLS
		corev1.EnvVar{Name: "TLS_ENABLED", Value: strconv.FormatBool(agent.Spec.TLS.Enabled)},
		corev1.EnvVar{Name: "TLS_SECRET_REF", Value: agent.Spec.TLS.SecretRef},
		// Agent loop
		corev1.EnvVar{Name: "AGENT_ENABLED", Value: strconv.FormatBool(agent.Spec.Agent.Enabled)},
		corev1.EnvVar{Name: "AGENT_MAX_CONVERSATION_HISTORY", Value: strconv.Itoa(int(agent.Spec.Agent.MaxConversationHistory))},
	)

	llm := agent.Spec.Agent.LLM

	if llm.Model != "" {
		parts := strings.SplitN(llm.Model, "/", 2)
		if len(parts) == 2 {
			envVars = append(envVars,
				corev1.EnvVar{Name: "A2A_AGENT_CLIENT_PROVIDER", Value: parts[0]},
				corev1.EnvVar{Name: "A2A_AGENT_CLIENT_MODEL", Value: parts[1]},
			)
		} else {
			envVars = append(envVars,
				corev1.EnvVar{Name: "A2A_AGENT_CLIENT_MODEL", Value: llm.Model},
			)
		}
	}

	if llm.BaseURL != "" {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "A2A_AGENT_CLIENT_BASE_URL",
			Value: llm.BaseURL,
		})
	}

	if llm.APIKeySecretRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "A2A_AGENT_CLIENT_API_KEY",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: llm.APIKeySecretRef.DeepCopy(),
			},
		})
	}

	if llm.MaxTokens != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "A2A_AGENT_CLIENT_MAX_TOKENS",
			Value: strconv.Itoa(int(*llm.MaxTokens)),
		})
	}

	if llm.Temperature != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "A2A_AGENT_CLIENT_TEMPERATURE",
			Value: *llm.Temperature,
		})
	}

	envVars = append(envVars,
		corev1.EnvVar{
			Name:  "A2A_AGENT_CLIENT_MAX_CHAT_COMPLETION_ITERATIONS",
			Value: strconv.Itoa(int(agent.Spec.Agent.MaxChatCompletionIterations)),
		},
		corev1.EnvVar{
			Name:  "A2A_AGENT_CLIENT_MAX_RETRIES",
			Value: strconv.Itoa(int(agent.Spec.Agent.MaxRetries)),
		},
	)

	if llm.CustomHeaders != nil {
		for i, header := range *llm.CustomHeaders {
			envVars = append(envVars,
				corev1.EnvVar{
					Name:  fmt.Sprintf("A2A_AGENT_CLIENT_CUSTOM_HEADER_%d_NAME", i),
					Value: header.Name,
				},
				corev1.EnvVar{
					Name:  fmt.Sprintf("A2A_AGENT_CLIENT_CUSTOM_HEADER_%d_VALUE", i),
					Value: header.Value,
				},
			)
		}
	}

	return envVars
}

// createOrUpdateDeployment handles deployment creation and updates
func (r *AgentReconciler) createOrUpdateDeployment(ctx context.Context, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && apiErrors.IsNotFound(err) {
		logger.Info("creating deployment", "Deployment.Name", deployment.Name)
		if err = r.Create(ctx, deployment); err != nil {
			return nil, err
		}
		return deployment, nil
	} else if err != nil {
		return nil, err
	}

	return r.updateDeploymentIfNeeded(ctx, deployment, found)
}

// updateDeploymentIfNeeded updates deployment if changes are detected
func (r *AgentReconciler) updateDeploymentIfNeeded(ctx context.Context, desired, found *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)

	for retries := 0; retries < 3; retries++ {
		latestDeployment := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: found.Name, Namespace: found.Namespace}, latestDeployment); err != nil {
			return nil, err
		}

		needsUpdate := false
		var changes []string

		desiredReplicas := int32(1)
		if latestDeployment.Spec.Replicas == nil || *latestDeployment.Spec.Replicas != desiredReplicas {
			latestDeployment.Spec.Replicas = &desiredReplicas
			needsUpdate = true
			changes = append(changes, fmt.Sprintf("replicas: %v -> %v",
				func() interface{} {
					if latestDeployment.Spec.Replicas == nil {
						return "nil"
					}
					return *latestDeployment.Spec.Replicas
				}(), desiredReplicas))
		}

		desiredTemplate := desired.Spec.Template.DeepCopy()
		if desiredTemplate.Annotations == nil {
			desiredTemplate.Annotations = map[string]string{}
		}

		existingAnnotations := latestDeployment.Spec.Template.Annotations
		for k, v := range existingAnnotations {
			if k == "kubectl.kubernetes.io/restartedAt" ||
				k == "deployment.kubernetes.io/revision" {
				desiredTemplate.Annotations[k] = v
			}
		}

		if !reflect.DeepEqual(latestDeployment.Spec.Template, *desiredTemplate) {
			latestDeployment.Spec.Template = *desiredTemplate
			needsUpdate = true
			changes = append(changes, "pod template")
		}

		if !reflect.DeepEqual(latestDeployment.Spec.Selector, desired.Spec.Selector) {
			latestDeployment.Spec.Selector = desired.Spec.Selector
			needsUpdate = true
			changes = append(changes, "selector")
		}

		if !needsUpdate {
			logger.Info("No deployment changes needed")
			return latestDeployment, nil
		}

		logger.Info("Updating Agent Deployment", "Deployment.Name", desired.Name, "changes", fmt.Sprintf("[%s]", fmt.Sprintf("%v", changes)))
		if err := r.Update(ctx, latestDeployment); err != nil {
			if apiErrors.IsConflict(err) && retries < 2 {
				logger.Info("Deployment update conflict, retrying", "retry", retries+1, "error", err)
				time.Sleep(time.Millisecond * 100)
				continue
			}
			return nil, err
		}
		logger.Info("Deployment updated successfully - pods will restart automatically")
		return latestDeployment, nil
	}

	return nil, fmt.Errorf("failed to update deployment after 3 retries due to conflicts")
}

func int32Ptr(i int32) *int32 { return &i }

// intstrFromInt returns an IntOrString for a port.
func intstrFromInt(i int) intstr.IntOrString {
	return intstr.IntOrString{Type: intstr.Int, IntVal: int32(i)}
}

// shouldWatchNamespace checks if the operator should watch resources in the given namespace
// based on WATCH_NAMESPACE_SELECTOR environment variable
func (r *AgentReconciler) shouldWatchNamespace(ctx context.Context, namespace string) bool {
	watchNamespaceSelector := os.Getenv("WATCH_NAMESPACE_SELECTOR")

	if watchNamespaceSelector == "" {
		return true
	}

	labelSelector, err := labels.Parse(watchNamespaceSelector)
	if err != nil {
		return true
	}

	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
		return false
	}

	return labelSelector.Matches(labels.Set(ns.Labels))
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Agent{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("agent").
		Complete(r)
}
