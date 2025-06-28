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
	"os"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

const (
	// Default values
	defaultPort        = int32(3000)
	defaultMaxReplicas = int32(10)

	// Container and volume names
	mcpContainerName = "mcp"
	defaultCommand   = "/mcp-server"
	tlsVolumeSuffix  = "-volume"
	tlsMountPath     = "/etc/tls"

	// Environment variables
	envServerHost = "SERVER_HOST"
	envServerPort = "SERVER_PORT"

	// Default server host
	defaultServerHost = "0.0.0.0"

	// Kubernetes resource kinds
	deploymentKind       = "Deployment"
	deploymentAPIVersion = "apps/v1"

	// Labels
	appLabel = "app"

	// Port names
	httpPortName = "http"

	// Service names
	serviceSuffix = "-service"

	// URL schemes
	httpScheme  = "http"
	httpsScheme = "https"
)

var (
	// Variables for values that need addresses
	defaultMinReplicas = int32(1)
)

// MCPReconciler reconciles a MCP object
type MCPReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=mcps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=mcps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=mcps/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MCPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("reconciling mcp", "mcp", req.NamespacedName)

	if !r.shouldWatchNamespace(ctx, req.Namespace) {
		logger.V(1).Info("skipping mcp in namespace not matching watch criteria", "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	var mcp v1alpha1.MCP
	if err := r.Get(ctx, req.NamespacedName, &mcp); err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.V(1).Info("mcp not found, probably deleted")
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !mcp.DeletionTimestamp.IsZero() {
		logger.V(1).Info("mcp is being deleted, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	deployment, err := r.reconcileDeployment(ctx, &mcp)
	if err != nil {
		logger.Error(err, "failed to reconcile mcp deployment", "mcp", req.NamespacedName)
		return ctrl.Result{}, fmt.Errorf("failed to reconcile deployment for mcp %s: %w", req.NamespacedName, err)
	}

	service, err := r.reconcileService(ctx, &mcp)
	if err != nil {
		logger.Error(err, "failed to reconcile mcp service", "mcp", req.NamespacedName)
		return ctrl.Result{}, fmt.Errorf("failed to reconcile service for mcp %s: %w", req.NamespacedName, err)
	}

	_, err = r.reconcileHPA(ctx, &mcp, deployment)
	if err != nil {
		logger.Error(err, "failed to reconcile hpa for mcp", "mcp", req.NamespacedName)
		return ctrl.Result{}, fmt.Errorf("failed to reconcile hpa for mcp %s: %w", req.NamespacedName, err)
	}

	err = r.updateStatus(ctx, &mcp, service, deployment)
	if err != nil {
		logger.Error(err, "failed to update mcp status", "mcp", req.NamespacedName)
		return ctrl.Result{}, fmt.Errorf("failed to update status for mcp %s: %w", req.NamespacedName, err)
	}

	logger.V(1).Info("successfully reconciled mcp", "mcp", req.NamespacedName)
	return ctrl.Result{}, nil
}

// reconcileHPA reconciles the Horizontal Pod Autoscaler for the MCP.
func (r *MCPReconciler) reconcileHPA(ctx context.Context, mcp *v1alpha1.MCP, deployment *appsv1.Deployment) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	logger := logf.FromContext(ctx)

	switch {
	case mcp.Spec.HPA == nil:
		logger.V(1).Info("no hpa spec defined, skipping hpa reconciliation", "mcp", mcp.Name)
		return nil, nil
	case !mcp.Spec.HPA.Enabled:
		logger.V(1).Info("hpa is disabled, skipping hpa reconciliation", "mcp", mcp.Name)
		return nil, nil
	case deployment == nil:
		return nil, fmt.Errorf("deployment is nil for mcp %s/%s", mcp.Namespace, mcp.Name)
	case r.Scheme == nil:
		return nil, fmt.Errorf("scheme is nil in MCPReconciler")
	}

	hpaConfig := mcp.Spec.HPA.Config
	if hpaConfig == nil {
		logger.V(1).Info("hpa config is nil, using default values", "mcp", mcp.Name)
		hpaConfig = &v1alpha1.CustomHorizontalPodAutoscalerSpec{
			MinReplicas: &defaultMinReplicas,
			MaxReplicas: defaultMaxReplicas,
			Metrics:     []autoscalingv2.MetricSpec{},
			Behavior:    nil,
		}
	}

	desiredHPA := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mcp.Name,
			Namespace: mcp.Namespace,
			Labels: map[string]string{
				appLabel: mcp.Name,
			},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Name:       deployment.Name,
				Kind:       deploymentKind,
				APIVersion: deploymentAPIVersion,
			},
			MinReplicas: hpaConfig.MinReplicas,
			MaxReplicas: hpaConfig.MaxReplicas,
			Metrics:     hpaConfig.Metrics,
			Behavior:    hpaConfig.Behavior,
		},
	}

	if err := controllerutil.SetControllerReference(mcp, desiredHPA, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference for hpa: %w", err)
	}

	return r.createOrUpdateHPA(ctx, desiredHPA)
}

// createOrUpdateHPA creates or updates the HPA resource
func (r *MCPReconciler) createOrUpdateHPA(ctx context.Context, desiredHPA *autoscalingv2.HorizontalPodAutoscaler) (*autoscalingv2.HorizontalPodAutoscaler, error) {
	logger := logf.FromContext(ctx)

	found := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Get(ctx, types.NamespacedName{Name: desiredHPA.Name, Namespace: desiredHPA.Namespace}, found)

	switch {
	case errors.IsNotFound(err):
		logger.Info("creating hpa", "hpa", desiredHPA.Name)
		if err := r.Create(ctx, desiredHPA); err != nil {
			return nil, fmt.Errorf("failed to create hpa %s: %w", desiredHPA.Name, err)
		}
		return desiredHPA, nil
	case err != nil:
		return nil, fmt.Errorf("failed to get hpa %s: %w", desiredHPA.Name, err)
	}

	if !reflect.DeepEqual(found.Spec, desiredHPA.Spec) {
		logger.Info("updating hpa", "hpa", found.Name)
		found.Spec = desiredHPA.Spec
		if err := r.Update(ctx, found); err != nil {
			return nil, fmt.Errorf("failed to update hpa %s: %w", found.Name, err)
		}
		return found, nil
	}

	logger.V(1).Info("hpa up-to-date, no update needed", "hpa", found.Name)
	return found, nil
}

// reconcileDeployment reconciles the Deployment for the MCP.
func (r *MCPReconciler) reconcileDeployment(ctx context.Context, mcp *v1alpha1.MCP) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)
	if mcp == nil {
		logger.Error(nil, "MCP is nil, cannot reconcile deployment")
		return nil, fmt.Errorf("MCP is nil")
	}

	deployment := r.buildDeployment(mcp)
	if deployment == nil {
		return nil, fmt.Errorf("failed to build deployment for MCP %s/%s", mcp.Namespace, mcp.Name)
	}

	if err := controllerutil.SetControllerReference(mcp, deployment, r.Scheme); err != nil {
		return nil, err
	}

	return r.createOrUpdateDeployment(ctx, mcp, deployment)
}

func (r *MCPReconciler) buildDeployment(mcp *v1alpha1.MCP) *appsv1.Deployment {
	if mcp == nil {
		return nil
	}

	if mcp.Spec.Image == "" {
		return nil
	}

	port := defaultPort
	command := []string{defaultCommand}

	envVars := []corev1.EnvVar{
		{
			Name:  envServerHost,
			Value: defaultServerHost,
		},
	}

	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	if mcp.Spec.Server != nil {
		if mcp.Spec.Server.Port != 0 {
			port = mcp.Spec.Server.Port
		}

		envVars = append(envVars, corev1.EnvVar{
			Name:  envServerPort,
			Value: fmt.Sprintf("%d", port),
		})

		if len(mcp.Spec.Server.Command) > 0 {
			command = mcp.Spec.Server.Command
		}

		if mcp.Spec.Server.TLS != nil && mcp.Spec.Server.TLS.Enabled && mcp.Spec.Server.TLS.SecretName != "" {
			volumes = append(volumes, corev1.Volume{
				Name: mcp.Spec.Server.TLS.SecretName + tlsVolumeSuffix,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: mcp.Spec.Server.TLS.SecretName,
					},
				},
			})
			volumeMounts = append(volumeMounts, corev1.VolumeMount{
				Name:      mcp.Spec.Server.TLS.SecretName + tlsVolumeSuffix,
				MountPath: tlsMountPath,
				ReadOnly:  true,
			})
		}
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mcp.Name,
			Namespace: mcp.Namespace,
			Labels: map[string]string{
				appLabel: mcp.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					appLabel: mcp.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						appLabel: mcp.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  mcpContainerName,
							Image: mcp.Spec.Image,
							Ports: []corev1.ContainerPort{
								{
									Name:          httpPortName,
									ContainerPort: port,
								},
							},
							VolumeMounts: volumeMounts,
							Env:          envVars,
							Command:      command,
						},
					},
					Volumes: volumes,
				},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
		},
	}

	if mcp.Spec.Replicas != nil && (mcp.Spec.HPA == nil || !mcp.Spec.HPA.Enabled) {
		deployment.Spec.Replicas = mcp.Spec.Replicas
	}

	return deployment
}

// createOrUpdateDeployment creates or updates the Deployment resource
func (r *MCPReconciler) createOrUpdateDeployment(ctx context.Context, mcp *v1alpha1.MCP, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)

	if deployment == nil {
		logger.V(1).Info("deployment spec is nil, skipping deployment creation", "mcp", mcp.Name)
		return nil, nil
	}

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)

	switch {
	case errors.IsNotFound(err):
		logger.Info("creating deployment", "deployment", deployment.Name)
		if err = r.Create(ctx, deployment); err != nil {
			return nil, fmt.Errorf("failed to create deployment %s: %w", deployment.Name, err)
		}
		return deployment, nil
	case err != nil:
		return nil, fmt.Errorf("failed to get deployment %s: %w", deployment.Name, err)
	}

	return r.updateDeploymentIfNeeded(ctx, mcp, deployment, found)
}

func (r *MCPReconciler) updateDeploymentIfNeeded(ctx context.Context, _ *v1alpha1.MCP, deployment *appsv1.Deployment, found *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)

	if reflect.DeepEqual(&deployment.Spec, &found.Spec) {
		logger.V(1).Info("deployment up-to-date, no update needed", "deployment", found.Name)
		return found, nil
	}

	found.Spec = deployment.Spec

	if err := r.Update(ctx, found); err != nil {
		return nil, fmt.Errorf("failed to update deployment %s: %w", found.Name, err)
	}

	logger.Info("updated deployment", "deployment", found.Name)
	return found, nil
}

// reconcileService reconciles the Service for the MCP.
func (r *MCPReconciler) reconcileService(ctx context.Context, mcp *v1alpha1.MCP) (*corev1.Service, error) {
	if mcp == nil {
		return nil, fmt.Errorf("mcp is nil")
	}

	service := r.buildService(mcp)
	if service == nil {
		return nil, fmt.Errorf("failed to build service for mcp %s/%s", mcp.Namespace, mcp.Name)
	}

	if err := controllerutil.SetControllerReference(mcp, service, r.Scheme); err != nil {
		return nil, fmt.Errorf("failed to set controller reference for service: %w", err)
	}

	return r.createOrUpdateService(ctx, service)
}

// buildService builds the Service resource for the MCP.
func (r *MCPReconciler) buildService(mcp *v1alpha1.MCP) *corev1.Service {
	if mcp == nil {
		return nil
	}

	port := defaultPort
	if mcp.Spec.Server != nil && mcp.Spec.Server.Port != 0 {
		port = mcp.Spec.Server.Port
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mcp.Name + serviceSuffix,
			Namespace: mcp.Namespace,
			Labels: map[string]string{
				appLabel: mcp.Name,
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				appLabel: mcp.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:     httpPortName,
					Port:     port,
					Protocol: corev1.ProtocolTCP,
				},
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	return service
}

// createOrUpdateService creates or updates the Service resource
func (r *MCPReconciler) createOrUpdateService(ctx context.Context, desiredService *corev1.Service) (*corev1.Service, error) {
	logger := logf.FromContext(ctx)

	found := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: desiredService.Name, Namespace: desiredService.Namespace}, found)

	switch {
	case errors.IsNotFound(err):
		logger.Info("creating service", "service", desiredService.Name)
		if err := r.Create(ctx, desiredService); err != nil {
			return nil, fmt.Errorf("failed to create service %s: %w", desiredService.Name, err)
		}
		return desiredService, nil
	case err != nil:
		return nil, fmt.Errorf("failed to get service %s: %w", desiredService.Name, err)
	}

	if !reflect.DeepEqual(found.Spec.Ports, desiredService.Spec.Ports) ||
		!reflect.DeepEqual(found.Spec.Selector, desiredService.Spec.Selector) {
		logger.Info("updating service", "service", found.Name)
		found.Spec.Ports = desiredService.Spec.Ports
		found.Spec.Selector = desiredService.Spec.Selector
		found.Spec.Type = desiredService.Spec.Type
		if err := r.Update(ctx, found); err != nil {
			return nil, fmt.Errorf("failed to update service %s: %w", found.Name, err)
		}
		return found, nil
	}

	logger.V(1).Info("service up-to-date, no update needed", "service", found.Name)
	return found, nil
}

// updateStatus updates the MCP status with URL and ready state
func (r *MCPReconciler) updateStatus(ctx context.Context, mcp *v1alpha1.MCP, service *corev1.Service, deployment *appsv1.Deployment) error {
	logger := logf.FromContext(ctx)

	url := r.buildServiceURL(mcp, service)

	isReady := r.isDeploymentReady(deployment)

	statusChanged := false
	if mcp.Status.URL != url {
		mcp.Status.URL = url
		statusChanged = true
	}
	if mcp.Status.Ready != isReady {
		mcp.Status.Ready = isReady
		statusChanged = true
	}
	if mcp.Status.ObservedGeneration != mcp.Generation {
		mcp.Status.ObservedGeneration = mcp.Generation
		statusChanged = true
	}

	if !statusChanged {
		logger.V(1).Info("status unchanged, skipping update", "mcp", mcp.Name)
		return nil
	}

	logger.Info("updating mcp status", "mcp", mcp.Name, "url", url, "ready", isReady)
	if err := r.Status().Update(ctx, mcp); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

// buildServiceURL constructs the URL for the MCP service
func (r *MCPReconciler) buildServiceURL(mcp *v1alpha1.MCP, service *corev1.Service) string {
	if service == nil {
		return ""
	}

	scheme := httpScheme
	if mcp.Spec.Server != nil && mcp.Spec.Server.TLS != nil && mcp.Spec.Server.TLS.Enabled {
		scheme = httpsScheme
	}

	port := defaultPort
	if mcp.Spec.Server != nil && mcp.Spec.Server.Port != 0 {
		port = mcp.Spec.Server.Port
	}

	return fmt.Sprintf("%s://%s.%s.svc.cluster.local:%d", scheme, service.Name, service.Namespace, port)
}

// isDeploymentReady checks if the deployment is ready
func (r *MCPReconciler) isDeploymentReady(deployment *appsv1.Deployment) bool {
	if deployment == nil {
		return false
	}

	return deployment.Status.ReadyReplicas > 0 &&
		deployment.Status.ReadyReplicas == deployment.Status.Replicas &&
		deployment.Status.Replicas == *deployment.Spec.Replicas
}

// shouldWatchNamespace checks if the operator should watch resources in the given namespace
// based on WATCH_NAMESPACE_SELECTOR environment variable
func (r *MCPReconciler) shouldWatchNamespace(ctx context.Context, namespace string) bool {
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
func (r *MCPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MCP{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Named("mcp").
		Complete(r)
}
