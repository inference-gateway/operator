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
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

// Reconcile drives an Orchestrator resource toward its desired state.
//
// The controller manages a singleton Deployment (replicas=1, strategy=Recreate)
// that runs the Inference Gateway CLI's `infer channels-manager` daemon.
// No Service is created because the orchestrator is outbound-only (Telegram long-poll).
func (r *OrchestratorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	var orch v1alpha1.Orchestrator
	if err := r.Get(ctx, req.NamespacedName, &orch); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !orch.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	deployment, err := r.reconcileDeployment(ctx, &orch)
	if err != nil {
		if apiErrors.IsConflict(err) {
			logger.V(1).Info("deployment reconciliation conflict, requeueing", "error", err)
			return ctrl.Result{RequeueAfter: time.Second * 1}, nil
		}
		logger.Error(err, "failed to reconcile deployment")
		return ctrl.Result{}, err
	}

	if err := r.updateStatus(ctx, &orch, deployment); err != nil {
		logger.Error(err, "failed to update orchestrator status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileDeployment ensures the Orchestrator's singleton Deployment exists and matches the spec.
func (r *OrchestratorReconciler) reconcileDeployment(ctx context.Context, orch *v1alpha1.Orchestrator) (*appsv1.Deployment, error) {
	deployment := r.buildOrchestratorDeployment(orch)

	if err := controllerutil.SetControllerReference(orch, deployment, r.Scheme); err != nil {
		return nil, err
	}

	return r.createOrUpdateOrchestratorDeployment(ctx, deployment)
}

// buildOrchestratorDeployment returns a singleton Deployment for the given Orchestrator.
func (r *OrchestratorReconciler) buildOrchestratorDeployment(orch *v1alpha1.Orchestrator) *appsv1.Deployment {
	labels := map[string]string{"app": orch.Name}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      orch.Name,
			Namespace: orch.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:      "orchestrator",
						Image:     orch.Spec.Image,
						Command:   []string{"infer"},
						Args:      []string{"channels-manager"},
						Env:       buildOrchestratorEnvironmentVars(orch),
						Resources: orch.Spec.Resources,
					}},
				},
			},
		},
	}
}

// buildOrchestratorEnvironmentVars translates OrchestratorSpec into INFER_* environment variables
// per the spec ↔ CLI config mapping documented in the Orchestrator CRD.
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

// updateStatus reflects Deployment availability into Orchestrator status.
func (r *OrchestratorReconciler) updateStatus(ctx context.Context, orch *v1alpha1.Orchestrator, deployment *appsv1.Deployment) error {
	patch := client.MergeFrom(orch.DeepCopy())

	ready := deployment.Status.AvailableReplicas >= 1
	orch.Status.Ready = ready
	orch.Status.ObservedGeneration = orch.Generation

	condition := metav1.Condition{
		Type:               "Ready",
		Status:             metav1.ConditionFalse,
		Reason:             "DeploymentNotAvailable",
		Message:            "orchestrator deployment has no available replicas yet",
		ObservedGeneration: orch.Generation,
		LastTransitionTime: metav1.Now(),
	}
	if ready {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "DeploymentAvailable"
		condition.Message = "orchestrator deployment is available"
	}
	setCondition(&orch.Status.Conditions, condition)

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

// SetupWithManager registers the Orchestrator controller with the manager.
func (r *OrchestratorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Orchestrator{}).
		Owns(&appsv1.Deployment{}).
		Named("orchestrator").
		Complete(r)
}
