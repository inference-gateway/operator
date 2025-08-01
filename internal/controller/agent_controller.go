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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
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

	card, err := fetchAgentCard(svc)
	if err != nil {
		logger.Info("failed to fetch agent card", "error", err.Error())
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	card.SkillsNames = card.Skills.SkillsNames()

	patch := client.MergeFrom(agent.DeepCopy())
	agent.Status.Card = *card
	if err := r.Status().Patch(ctx, &agent, patch); err != nil {
		logger.Error(err, "unable to update agent status.card")
		return ctrl.Result{}, err
	}
	logger.Info("updated agent status.card", "version", card.Version)
	return ctrl.Result{}, nil
}

// buildAgentService returns a Service for the given Agent resource.
func buildAgentService(agent *v1alpha1.Agent) *corev1.Service {
	labels := map[string]string{
		"app": agent.Name,
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
				Port:       agent.Spec.Port,
				TargetPort: intstrFromInt(int(agent.Spec.Port)),
			}},
		},
	}
}

// fetchAgentCard retrieves the agent card from the given base URL and unmarshals it into an AgentCard.
func fetchAgentCard(svc *corev1.Service) (*v1alpha1.Card, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + svc.Name + "." + svc.Namespace + ".svc.cluster.local:8080" + "/.well-known/agent.json")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
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

	port := int32(8080)
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
						Env:   env,
						Name:  "agent",
						Image: agent.Spec.Image,
						Ports: []corev1.ContainerPort{{
							ContainerPort: port,
						}},
					}},
				},
			},
		},
	}
}

// buildAgentEnvironmentVars creates comprehensive environment variables from Agent spec
func (r *AgentReconciler) buildAgentEnvironmentVars(agent *v1alpha1.Agent) []corev1.EnvVar {
	envVars := []corev1.EnvVar{}

	if agent.Spec.Env != nil {
		envVars = append(envVars, *agent.Spec.Env...)
	}

	envVars = append(envVars,
		corev1.EnvVar{
			Name:  "TIMEZONE",
			Value: agent.Spec.Timezone,
		},
		corev1.EnvVar{
			Name:  "PORT",
			Value: strconv.Itoa(int(agent.Spec.Port)),
		},
		corev1.EnvVar{
			Name:  "HOST",
			Value: agent.Spec.Host,
		},
		corev1.EnvVar{
			Name:  "READ_TIMEOUT",
			Value: agent.Spec.ReadTimeout,
		},
		corev1.EnvVar{
			Name:  "WRITE_TIMEOUT",
			Value: agent.Spec.WriteTimeout,
		},
		corev1.EnvVar{
			Name:  "IDLE_TIMEOUT",
			Value: agent.Spec.IdleTimeout,
		},
		// Logging configuration
		corev1.EnvVar{
			Name:  "LOG_LEVEL",
			Value: agent.Spec.Logging.Level,
		},
		corev1.EnvVar{
			Name:  "LOG_FORMAT",
			Value: agent.Spec.Logging.Format,
		},
		// Telemetry configuration
		corev1.EnvVar{
			Name:  "TELEMETRY_ENABLED",
			Value: strconv.FormatBool(agent.Spec.Telemetry.Enabled),
		},
		corev1.EnvVar{
			Name:  "QUEUE_ENABLED",
			Value: strconv.FormatBool(agent.Spec.Queue.Enabled),
		},
		corev1.EnvVar{
			Name:  "QUEUE_MAX_SIZE",
			Value: strconv.Itoa(int(agent.Spec.Queue.MaxSize)),
		},
		corev1.EnvVar{
			Name:  "QUEUE_CLEANUP_INTERVAL",
			Value: agent.Spec.Queue.CleanupInterval,
		},
		corev1.EnvVar{
			Name:  "TLS_ENABLED",
			Value: strconv.FormatBool(agent.Spec.TLS.Enabled),
		},
		corev1.EnvVar{
			Name:  "TLS_SECRET_REF",
			Value: agent.Spec.TLS.SecretRef,
		},
		corev1.EnvVar{
			Name:  "AGENT_ENABLED",
			Value: strconv.FormatBool(agent.Spec.Agent.Enabled),
		},
		corev1.EnvVar{
			Name:  "AGENT_MAX_CONVERSATION_HISTORY",
			Value: strconv.Itoa(int(agent.Spec.Agent.MaxConversationHistory)),
		},
		corev1.EnvVar{
			Name:  "AGENT_MAX_CHAT_COMPLETION_ITERATIONS",
			Value: strconv.Itoa(int(agent.Spec.Agent.MaxChatCompletionIterations)),
		},
		corev1.EnvVar{
			Name:  "AGENT_MAX_RETRIES",
			Value: strconv.Itoa(int(agent.Spec.Agent.MaxRetries)),
		},
		corev1.EnvVar{
			Name:  "AGENT_API_KEY_SECRET_REF",
			Value: agent.Spec.Agent.APIKey.SecretRef,
		},
		corev1.EnvVar{
			Name:  "AGENT_LLM_MODEL",
			Value: agent.Spec.Agent.LLM.Model,
		},
		corev1.EnvVar{
			Name:  "AGENT_LLM_SYSTEM_PROMPT",
			Value: agent.Spec.Agent.LLM.SystemPrompt,
		},
	)

	if agent.Spec.Agent.LLM.MaxTokens != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "AGENT_LLM_MAX_TOKENS",
			Value: strconv.Itoa(int(*agent.Spec.Agent.LLM.MaxTokens)),
		})
	}

	if agent.Spec.Agent.LLM.Temperature != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name:  "AGENT_LLM_TEMPERATURE",
			Value: *agent.Spec.Agent.LLM.Temperature,
		})
	}

	if agent.Spec.Agent.LLM.CustomHeaders != nil {
		for i, header := range *agent.Spec.Agent.LLM.CustomHeaders {
			envVars = append(envVars,
				corev1.EnvVar{
					Name:  fmt.Sprintf("AGENT_LLM_CUSTOM_HEADER_%d_NAME", i),
					Value: header.Name,
				},
				corev1.EnvVar{
					Name:  fmt.Sprintf("AGENT_LLM_CUSTOM_HEADER_%d_VALUE", i),
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
