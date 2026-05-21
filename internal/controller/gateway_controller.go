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
	"sort"
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	handler "sigs.k8s.io/controller-runtime/pkg/handler"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=mcps,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=gateways/status,verbs=get
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=gateway.networking.k8s.io,resources=httproutes/status,verbs=get
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GatewayReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	if !r.shouldWatchNamespace(ctx, req.Namespace) {
		logger.V(1).Info("Skipping Gateway in namespace not matching watch criteria", "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	gateway := &corev1alpha1.Gateway{}
	err := r.Get(ctx, req.NamespacedName, gateway)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Failed to get Gateway")
		return ctrl.Result{}, err
	}

	deployment, err := r.reconcileDeployment(ctx, gateway)
	if err != nil {
		if errors.IsConflict(err) {
			logger.V(1).Info("Deployment reconciliation conflict, requeueing", "error", err)
			return ctrl.Result{RequeueAfter: time.Second * 1}, nil
		}
		logger.Error(err, "Failed to reconcile Deployment")
		return ctrl.Result{}, err
	}

	err = r.reconcileService(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to reconcile Service")
		return ctrl.Result{}, err
	}

	err = r.reconcileRouting(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to reconcile routing")
		return ctrl.Result{}, err
	}

	err = r.reconcileHPA(ctx, gateway, deployment)
	if err != nil {
		logger.Error(err, "Failed to reconcile HPA")
		return ctrl.Result{}, err
	}

	err = r.reconcileRBAC(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to reconcile RBAC")
		return ctrl.Result{}, err
	}

	err = r.reconcileGatewayStatus(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to reconcile Gateway status")
		return ctrl.Result{}, err
	}

	logger.Info("Reconciled Gateway successfully",
		"gateway", gateway.Name,
		"deployment", deployment.Name)

	return ctrl.Result{}, nil
}

// reconcileGatewayStatus updates the Gateway status with the current URL
func (r *GatewayReconciler) reconcileGatewayStatus(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)

	err := r.updateProvidersSummary(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to update ProvidersSummary")
		return err
	}

	getHostAndScheme := func(gw *corev1alpha1.Gateway) (string, string) {
		routing := gw.Spec.Routing
		var host, scheme string

		if routing != nil && routing.Enabled {
			httpRoute := &gwapiv1.HTTPRoute{}
			err := r.Get(ctx, types.NamespacedName{Name: gw.Name, Namespace: gw.Namespace}, httpRoute)
			if err != nil || len(httpRoute.Spec.Hostnames) == 0 {
				logger.V(1).Info("HTTPRoute not found or has no hostnames, falling back to service fqdn", "gateway", gw.Name)
				host = fmt.Sprintf("%s.%s.svc.cluster.local", gw.Name, gw.Namespace)
			} else {
				host = string(httpRoute.Spec.Hostnames[0])
			}
			switch {
			case tlsEnabled(gw):
				scheme = "https"
			default:
				scheme = "http"
			}
		} else {
			host = fmt.Sprintf("%s.%s.svc.cluster.local", gw.Name, gw.Namespace)
			switch {
			case gw.Spec.Server != nil && gw.Spec.Server.TLS != nil && gw.Spec.Server.TLS.Enabled:
				scheme = "https"
			default:
				scheme = "http"
			}
		}
		return host, scheme
	}

	const maxRetries = 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		latest := &corev1alpha1.Gateway{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(gateway), latest); err != nil {
			return err
		}

		host, scheme := getHostAndScheme(latest)
		newURL := fmt.Sprintf("%s://%s", scheme, host)

		var mcpURLs []string
		if latest.Spec.MCP != nil && latest.Spec.MCP.Enabled {
			mcpURLs = r.assembleMCPServerURLs(ctx, latest)
		}

		urlChanged := newURL != "" && latest.Status.URL != newURL
		mcpChanged := !stringSlicesEqual(latest.Status.MCPServers, mcpURLs) ||
			latest.Status.MCPServerCount != int32(len(mcpURLs))
		if !urlChanged && !mcpChanged {
			return nil
		}

		if urlChanged {
			latest.Status.URL = newURL
		}
		if mcpChanged {
			latest.Status.MCPServers = mcpURLs
			latest.Status.MCPServerCount = int32(len(mcpURLs))
		}

		if err := r.Status().Update(ctx, latest); err != nil {
			if errors.IsConflict(err) {
				lastErr = err
				continue
			}
			logger.Error(err, "failed to update gateway status")
			return err
		}
		logger.V(1).Info("updated gateway status", "gateway", latest.Name, "url", newURL, "mcpServerCount", len(mcpURLs))
		return nil
	}
	return lastErr
}

// stringSlicesEqual reports whether a and b have identical, in-order contents.
func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (r *GatewayReconciler) updateProvidersSummary(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)

	logger.V(1).Info("Updating ProvidersSummary", "Gateway.Name", gateway.Name)

	allowedProviders := map[string]string{
		"custom": "", "anthropic": "", "cloudflare": "", "cohere": "", "groq": "", "ollama": "", "openai": "", "deepseek": "",
	}
	configuredProviderNames := []string{}
	for _, p := range gateway.Spec.Providers {
		if _, ok := allowedProviders[strings.ToLower(p.Name)]; !ok {
			logger.V(1).Info("Skipping unsupported provider", "provider", p)
			continue
		}

		secret := &corev1.Secret{}
		var secretNamespacedName types.NamespacedName

		for _, envVar := range *p.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
				secretNamespacedName = types.NamespacedName{
					Name:      envVar.ValueFrom.SecretKeyRef.Name,
					Namespace: gateway.Namespace,
				}
				break
			}
		}

		err := r.Get(ctx, secretNamespacedName, secret)
		if err != nil {
			if errors.IsNotFound(err) {
				logger.V(1).Info("Skipping provider with missing secret", "provider", p, "secret", secretNamespacedName)
			} else {
				logger.Error(err, "Failed to get secret for provider", "provider", p, "secret", secretNamespacedName)
			}
			continue
		}
		apiKeyFound := false
		for _, envVar := range *p.Env {
			if envVar.ValueFrom != nil && envVar.ValueFrom.SecretKeyRef != nil {
				apiKeyBytes, ok := secret.Data[envVar.ValueFrom.SecretKeyRef.Key]
				if ok && len(apiKeyBytes) > 0 {
					apiKeyFound = true
					break
				}
			}
		}
		if !p.Enabled {
			logger.V(1).Info("Skipping provider that is not enabled", "provider", p)
			continue
		}

		if !apiKeyFound {
			logger.V(1).Info("Skipping provider with missing or empty API key", "provider", p, "secret", secretNamespacedName)
			continue
		}

		configuredProviderNames = append(configuredProviderNames, p.Name)
	}
	summary := strings.Join(configuredProviderNames, ",")
	if gateway.Status.ProviderSummary != summary {
		gateway.Status.ProviderSummary = summary
		if err := r.Status().Update(ctx, gateway); err != nil {
			logger.Error(err, "Failed to update ProviderSummary in status")
			return err
		}
	}

	logger.V(1).Info("Updated ProvidersSummary successfully", "Gateway.Name", gateway.Name, "summary", summary)

	return nil
}

// reconcileDeployment ensures the Deployment exists with the correct configuration
func (r *GatewayReconciler) reconcileDeployment(ctx context.Context, gateway *corev1alpha1.Gateway) (*appsv1.Deployment, error) {
	deployment := r.buildDeployment(ctx, gateway)

	if err := controllerutil.SetControllerReference(gateway, deployment, r.Scheme); err != nil {
		return nil, err
	}

	return r.createOrUpdateDeployment(ctx, gateway, deployment)
}

// buildDeployment creates a Deployment resource based on Gateway spec
func (r *GatewayReconciler) buildDeployment(ctx context.Context, gateway *corev1alpha1.Gateway) *appsv1.Deployment {
	containerPorts := r.buildContainerPorts(gateway)

	volumes := []corev1.Volume{}
	volumeMounts := []corev1.VolumeMount{}

	if gateway.Spec.Server != nil && gateway.Spec.Server.TLS != nil && gateway.Spec.Server.TLS.Enabled {
		volumes = append(volumes, corev1.Volume{
			Name: "inference-gateway-tls",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "inference-gateway-tls",
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "inference-gateway-tls",
			MountPath: "/app/tls",
			ReadOnly:  true,
		})
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": gateway.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": gateway.Name,
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: r.getServiceAccountName(gateway),
					Containers: []corev1.Container{
						r.buildContainer(ctx, gateway, containerPorts, volumeMounts),
					},
					Volumes: volumes,
				},
			},
		},
	}

	r.setDeploymentReplicas(gateway, deployment)
	return deployment
}

// buildContainer creates the main container specification with custom volume mounts
func (r *GatewayReconciler) buildContainer(ctx context.Context, gateway *corev1alpha1.Gateway, containerPorts []corev1.ContainerPort, volumeMounts []corev1.VolumeMount) corev1.Container { //nolint gocyclo
	port := int32(8080)
	if gateway.Spec.Server != nil && gateway.Spec.Server.Port > 0 {
		port = gateway.Spec.Server.Port
	}

	image := gateway.Spec.Image
	if image == "" {
		image = "ghcr.io/inference-gateway/inference-gateway:latest"
	}

	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(256*1024*1024, resource.BinarySI),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
			corev1.ResourceMemory: *resource.NewQuantity(1024*1024*1024, resource.BinarySI),
		},
	}
	if gateway.Spec.Resources != nil {
		if gateway.Spec.Resources.Requests != nil {
			if gateway.Spec.Resources.Requests.CPU != "" {
				if cpuQty, err := resource.ParseQuantity(gateway.Spec.Resources.Requests.CPU); err == nil {
					resources.Requests[corev1.ResourceCPU] = cpuQty
				}
			}
			if gateway.Spec.Resources.Requests.Memory != "" {
				if memQty, err := resource.ParseQuantity(gateway.Spec.Resources.Requests.Memory); err == nil {
					resources.Requests[corev1.ResourceMemory] = memQty
				}
			}
		}
		if gateway.Spec.Resources.Limits != nil {
			if gateway.Spec.Resources.Limits.CPU != "" {
				if cpuQty, err := resource.ParseQuantity(gateway.Spec.Resources.Limits.CPU); err == nil {
					resources.Limits[corev1.ResourceCPU] = cpuQty
				}
			}
			if gateway.Spec.Resources.Limits.Memory != "" {
				if memQty, err := resource.ParseQuantity(gateway.Spec.Resources.Limits.Memory); err == nil {
					resources.Limits[corev1.ResourceMemory] = memQty
				}
			}
		}
	}

	envVars := []corev1.EnvVar{
		{
			Name:  "ENVIRONMENT",
			Value: gateway.Spec.Environment,
		},
		{
			Name:  "TELEMETRY_ENABLE",
			Value: strconv.FormatBool(gateway.Spec.Telemetry != nil && gateway.Spec.Telemetry.Enabled),
		},
		{
			Name:  "AUTH_ENABLE",
			Value: strconv.FormatBool(gateway.Spec.Auth != nil && gateway.Spec.Auth.Enabled),
		},
	}

	if gateway.Spec.Auth != nil && gateway.Spec.Auth.Enabled && gateway.Spec.Auth.OIDC != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "OIDC_CLIENT_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: gateway.Spec.Auth.OIDC.ClientSecretRef.Name,
					},
					Key: gateway.Spec.Auth.OIDC.ClientSecretRef.Key,
				},
			},
		})
	}

	if gateway.Spec.MCP != nil && gateway.Spec.MCP.Enabled {
		envVars = append(envVars,
			corev1.EnvVar{
				Name:  "MCP_ENABLE",
				Value: fmt.Sprintf("%t", gateway.Spec.MCP.Enabled),
			},
			corev1.EnvVar{
				Name:  "MCP_EXPOSE",
				Value: fmt.Sprintf("%t", gateway.Spec.MCP.Expose),
			},
			corev1.EnvVar{
				Name:  "MCP_SERVERS",
				Value: strings.Join(r.assembleMCPServerURLs(ctx, gateway), ","),
			},
			corev1.EnvVar{
				Name: "MCP_CLIENT_TIMEOUT",
				Value: func() string {
					if gateway.Spec.MCP.Timeouts != nil {
						return gateway.Spec.MCP.Timeouts.Client
					}
					return "5s"
				}(),
			},
			corev1.EnvVar{
				Name: "MCP_DIAL_TIMEOUT",
				Value: func() string {
					if gateway.Spec.MCP.Timeouts != nil {
						return gateway.Spec.MCP.Timeouts.Dial
					}
					return "3s"
				}(),
			},
			corev1.EnvVar{
				Name: "MCP_TLS_HANDSHAKE_TIMEOUT",
				Value: func() string {
					if gateway.Spec.MCP.Timeouts != nil {
						return gateway.Spec.MCP.Timeouts.TLSHandshake
					}
					return "3s"
				}(),
			},
			corev1.EnvVar{
				Name: "MCP_RESPONSE_HEADER_TIMEOUT",
				Value: func() string {
					if gateway.Spec.MCP.Timeouts != nil {
						return gateway.Spec.MCP.Timeouts.ResponseHeader
					}
					return "3s"
				}(),
			},
		)
	}

	providerEnvVars := []corev1.EnvVar{}
	for _, provider := range gateway.Spec.Providers {
		if !provider.Enabled {
			continue
		}

		if provider.Env != nil {
			providerEnvVars = append(providerEnvVars, *provider.Env...)
		}
	}

	return corev1.Container{
		Name:         "inference-gateway",
		Image:        image,
		Ports:        containerPorts,
		Env:          append(envVars, providerEnvVars...),
		VolumeMounts: volumeMounts,
		Resources:    resources,
		LivenessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(int(port)),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/health",
					Port: intstr.FromInt(int(port)),
				},
			},
			InitialDelaySeconds: 5,
			PeriodSeconds:       10,
		},
	}
}

// setDeploymentReplicas sets the replica count based on HPA configuration
func (r *GatewayReconciler) setDeploymentReplicas(gateway *corev1alpha1.Gateway, deployment *appsv1.Deployment) {
	if gateway.Spec.HPA != nil && gateway.Spec.HPA.Enabled {
		deployment.Spec.Replicas = nil
	} else {
		replicas := int32(1)
		if gateway.Spec.Replicas != nil {
			replicas = *gateway.Spec.Replicas
		}
		deployment.Spec.Replicas = &replicas
	}
}

// createOrUpdateDeployment handles deployment creation and updates
func (r *GatewayReconciler) createOrUpdateDeployment(ctx context.Context, gateway *corev1alpha1.Gateway, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := log.FromContext(ctx)

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating deployment", "Deployment.Name", deployment.Name)
		if err = r.Create(ctx, deployment); err != nil {
			return nil, err
		}
		return deployment, nil
	} else if err != nil {
		return nil, err
	}

	return r.updateDeploymentIfNeeded(ctx, gateway, deployment, found)
}

// updateDeploymentIfNeeded updates deployment if changes are detected
func (r *GatewayReconciler) updateDeploymentIfNeeded(ctx context.Context, gateway *corev1alpha1.Gateway, desired, found *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := log.FromContext(ctx)

	for retries := 0; retries < 3; retries++ {
		latestDeployment := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: found.Name, Namespace: found.Namespace}, latestDeployment); err != nil {
			return nil, err
		}

		needsUpdate := false
		var changes []string

		if gateway.Spec.HPA == nil || !gateway.Spec.HPA.Enabled {
			desiredReplicas := int32(1)
			if gateway.Spec.Replicas != nil {
				desiredReplicas = *gateway.Spec.Replicas
			}

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
		} else {
			if latestDeployment.Spec.Replicas != nil {
				logger.V(1).Info("HPA is enabled, not modifying replicas field",
					"current", *latestDeployment.Spec.Replicas)
			}
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

		logger.Info("Updating Deployment", "Deployment.Name", desired.Name, "changes", strings.Join(changes, ", "))
		if err := r.Update(ctx, latestDeployment); err != nil {
			if errors.IsConflict(err) && retries < 2 {
				logger.Info("Deployment update conflict, retrying", "retry", retries+1, "error", err)
				time.Sleep(time.Millisecond * 100)
				continue
			}
			return nil, err
		}
		logger.Info("Deployment updated successfully")
		return latestDeployment, nil
	}

	return nil, fmt.Errorf("failed to update deployment after 3 retries due to conflicts")
}

// reconcileService ensures the Service exists with the correct configuration
func (r *GatewayReconciler) reconcileService(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)

	var mainPort int32
	if gateway.Spec.Service != nil && gateway.Spec.Service.Port > 0 {
		mainPort = gateway.Spec.Service.Port
	} else if gateway.Spec.Server != nil && gateway.Spec.Server.Port > 0 {
		mainPort = gateway.Spec.Server.Port
	} else if gateway.Spec.Server != nil && gateway.Spec.Server.TLS != nil && gateway.Spec.Server.TLS.Enabled {
		mainPort = 8443
	} else {
		mainPort = 8080
	}

	servicePorts := []corev1.ServicePort{
		{
			Port:       mainPort,
			TargetPort: intstr.FromInt(int(mainPort)),
			Protocol:   corev1.ProtocolTCP,
			Name:       "http",
		},
	}

	if gateway.Spec.Telemetry != nil && gateway.Spec.Telemetry.Enabled && gateway.Spec.Telemetry.Metrics != nil && gateway.Spec.Telemetry.Metrics.Enabled {
		logger.V(1).Info("Adding metrics port to Service", "port", gateway.Spec.Telemetry.Metrics.Port)
		metricsPort := int32(9464)
		if gateway.Spec.Telemetry.Metrics.Port > 0 {
			metricsPort = gateway.Spec.Telemetry.Metrics.Port
		}
		servicePorts = append(servicePorts, corev1.ServicePort{
			Port:       metricsPort,
			TargetPort: intstr.FromInt(int(metricsPort)),
			Protocol:   corev1.ProtocolTCP,
			Name:       "metrics",
		})
	}

	if gateway.Spec.Server != nil && gateway.Spec.Server.TLS != nil && gateway.Spec.Server.TLS.Enabled && mainPort != 8443 {
		logger.V(1).Info("Adding HTTPS port to Service", "port", 8443)
		servicePorts = append(servicePorts, corev1.ServicePort{
			Port:       8443,
			TargetPort: intstr.FromInt(8443),
			Protocol:   corev1.ProtocolTCP,
			Name:       "https",
		})
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": gateway.Name,
			},
			Ports: servicePorts,
		},
	}

	if err := controllerutil.SetControllerReference(gateway, service, r.Scheme); err != nil {
		return err
	}

	found := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating Service", "Service.Name", service.Name)
		if err = r.Create(ctx, service); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		foundSpec := found.Spec
		serviceSpec := service.Spec
		serviceSpec.ClusterIP = foundSpec.ClusterIP

		if !reflect.DeepEqual(foundSpec.Ports, serviceSpec.Ports) || !reflect.DeepEqual(foundSpec.Selector, serviceSpec.Selector) {
			found.Spec = serviceSpec
			logger.Info("Updating Service", "Service.Name", service.Name)
			if err = r.Update(ctx, found); err != nil {
				return err
			}
		}
	}

	return nil
}

// reconcileRouting orchestrates creation and deletion of the upstream
// Gateway and HTTPRoute resources implementing north-south routing.
// gateway here is the project CR; gw* helpers below construct upstream
// gateway.networking.k8s.io types.
func (r *GatewayReconciler) reconcileRouting(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling routing", "Gateway.Name", gateway.Name, "Gateway.Namespace", gateway.Namespace)

	if gateway.Spec.Routing == nil || !gateway.Spec.Routing.Enabled {
		return r.deleteOwnedRouting(ctx, gateway)
	}

	if err := r.reconcileUpstreamGateway(ctx, gateway); err != nil {
		return err
	}

	return r.reconcileHTTPRoute(ctx, gateway)
}

// deleteOwnedRouting removes any operator-owned Gateway/HTTPRoute when
// routing is disabled or the CR is being torn down.
func (r *GatewayReconciler) deleteOwnedRouting(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)
	key := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}

	httpRoute := &gwapiv1.HTTPRoute{}
	err := r.Get(ctx, key, httpRoute)
	switch {
	case err == nil:
		logger.Info("Deleting HTTPRoute (routing disabled)", "HTTPRoute.Name", httpRoute.Name)
		if delErr := r.Delete(ctx, httpRoute); delErr != nil && !errors.IsNotFound(delErr) {
			return delErr
		}
	case !errors.IsNotFound(err):
		return err
	}

	gw := &gwapiv1.Gateway{}
	err = r.Get(ctx, key, gw)
	switch {
	case err == nil:
		logger.Info("Deleting Gateway (routing disabled)", "Gateway.Name", gw.Name)
		if delErr := r.Delete(ctx, gw); delErr != nil && !errors.IsNotFound(delErr) {
			return delErr
		}
	case !errors.IsNotFound(err):
		return err
	}

	return nil
}

// reconcileUpstreamGateway creates or updates the operator-owned
// upstream Gateway. Skipped in advanced mode (when ParentRefs is set).
func (r *GatewayReconciler) reconcileUpstreamGateway(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)

	if isAdvancedRoutingMode(gateway) {
		gw := &gwapiv1.Gateway{}
		err := r.Get(ctx, types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}, gw)
		switch {
		case err == nil:
			logger.Info("Deleting operator-owned Gateway (advanced mode uses parentRefs)", "Gateway.Name", gw.Name)
			if delErr := r.Delete(ctx, gw); delErr != nil && !errors.IsNotFound(delErr) {
				return delErr
			}
		case !errors.IsNotFound(err):
			return err
		}
		return nil
	}

	desired := r.buildUpstreamGateway(gateway)
	if err := controllerutil.SetControllerReference(gateway, desired, r.Scheme); err != nil {
		return err
	}

	found := &gwapiv1.Gateway{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	switch {
	case errors.IsNotFound(err):
		logger.Info("Creating Gateway", "Gateway.Name", desired.Name)
		return r.Create(ctx, desired)
	case err != nil:
		return err
	}

	if !reflect.DeepEqual(found.Spec, desired.Spec) || !reflect.DeepEqual(found.Annotations, desired.Annotations) {
		found.Spec = desired.Spec
		found.Annotations = desired.Annotations
		logger.Info("Updating Gateway", "Gateway.Name", desired.Name)
		return r.Update(ctx, found)
	}

	return nil
}

// reconcileHTTPRoute creates or updates the operator-owned HTTPRoute.
func (r *GatewayReconciler) reconcileHTTPRoute(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)

	desired := r.buildHTTPRoute(gateway)
	if err := controllerutil.SetControllerReference(gateway, desired, r.Scheme); err != nil {
		return err
	}

	found := &gwapiv1.HTTPRoute{}
	err := r.Get(ctx, types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}, found)
	switch {
	case errors.IsNotFound(err):
		logger.Info("Creating HTTPRoute", "HTTPRoute.Name", desired.Name)
		return r.Create(ctx, desired)
	case err != nil:
		return err
	}

	if !reflect.DeepEqual(found.Spec, desired.Spec) {
		found.Spec = desired.Spec
		logger.Info("Updating HTTPRoute", "HTTPRoute.Name", desired.Name)
		return r.Update(ctx, found)
	}

	return nil
}

// buildUpstreamGateway constructs the gateway.networking.k8s.io Gateway
// resource the operator owns in default mode.
func (r *GatewayReconciler) buildUpstreamGateway(gateway *corev1alpha1.Gateway) *gwapiv1.Gateway {
	routing := gateway.Spec.Routing
	gwSpec := routing.Gateway

	className := gwapiv1.ObjectName("envoy")
	if gwSpec != nil && gwSpec.GatewayClassName != "" {
		className = gwapiv1.ObjectName(gwSpec.GatewayClassName)
	}

	listeners := []gwapiv1.Listener{buildDefaultListener(gateway)}

	annotations := map[string]string{}
	if gwSpec != nil && gwSpec.TLS != nil && gwSpec.TLS.Issuer != "" {
		annotations["cert-manager.io/cluster-issuer"] = gwSpec.TLS.Issuer
	}

	return &gwapiv1.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
			Labels: map[string]string{
				"app": gateway.Name,
			},
			Annotations: annotations,
		},
		Spec: gwapiv1.GatewaySpec{
			GatewayClassName: className,
			Listeners:        listeners,
		},
	}
}

// buildDefaultListener synthesizes the default listener (HTTP/80 or
// HTTPS/443 depending on TLS.Enabled) from RoutingHTTPRouteSpec.Hostnames.
func buildDefaultListener(gateway *corev1alpha1.Gateway) gwapiv1.Listener {
	tls := tlsSpec(gateway)
	httpRoute := gateway.Spec.Routing.HTTPRoute

	var hostname *gwapiv1.Hostname
	if httpRoute != nil && len(httpRoute.Hostnames) > 0 {
		h := httpRoute.Hostnames[0]
		hostname = &h
	}

	if tls != nil && tls.Enabled {
		secretName := gwapiv1.ObjectName(tlsSecretName(gateway))
		return gwapiv1.Listener{
			Name:     "https",
			Protocol: gwapiv1.HTTPSProtocolType,
			Port:     gwapiv1.PortNumber(443),
			Hostname: hostname,
			TLS: &gwapiv1.GatewayTLSConfig{
				CertificateRefs: []gwapiv1.SecretObjectReference{
					{Name: secretName},
				},
			},
		}
	}

	return gwapiv1.Listener{
		Name:     "http",
		Protocol: gwapiv1.HTTPProtocolType,
		Port:     gwapiv1.PortNumber(80),
		Hostname: hostname,
	}
}

// buildHTTPRoute constructs the operator-owned HTTPRoute. ParentRefs come
// from Routing.Gateway.ParentRefs (advanced) or default to the
// operator-managed Gateway in the same namespace.
func (r *GatewayReconciler) buildHTTPRoute(gateway *corev1alpha1.Gateway) *gwapiv1.HTTPRoute {
	routing := gateway.Spec.Routing
	httpRouteSpec := routing.HTTPRoute

	parentRefs := defaultParentRefs(gateway)
	if isAdvancedRoutingMode(gateway) {
		parentRefs = routing.Gateway.ParentRefs
	}

	rules := defaultHTTPRouteRules(gateway)

	var hostnames []gwapiv1.Hostname
	if httpRouteSpec != nil {
		hostnames = httpRouteSpec.Hostnames
	}

	return &gwapiv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
			Labels: map[string]string{
				"app": gateway.Name,
			},
		},
		Spec: gwapiv1.HTTPRouteSpec{
			CommonRouteSpec: gwapiv1.CommonRouteSpec{
				ParentRefs: parentRefs,
			},
			Hostnames: hostnames,
			Rules:     rules,
		},
	}
}

func defaultParentRefs(gateway *corev1alpha1.Gateway) []gwapiv1.ParentReference {
	return []gwapiv1.ParentReference{
		{Name: gwapiv1.ObjectName(gateway.Name)},
	}
}

func defaultHTTPRouteRules(gateway *corev1alpha1.Gateway) []gwapiv1.HTTPRouteRule {
	pathPrefix := gwapiv1.PathMatchPathPrefix
	rootPath := "/"
	port := gwapiv1.PortNumber(gatewayServicePort(gateway))

	return []gwapiv1.HTTPRouteRule{
		{
			Matches: []gwapiv1.HTTPRouteMatch{
				{
					Path: &gwapiv1.HTTPPathMatch{
						Type:  &pathPrefix,
						Value: &rootPath,
					},
				},
			},
			BackendRefs: []gwapiv1.HTTPBackendRef{
				{
					BackendRef: gwapiv1.BackendRef{
						BackendObjectReference: gwapiv1.BackendObjectReference{
							Name: gwapiv1.ObjectName(gateway.Name),
							Port: &port,
						},
					},
				},
			},
		},
	}
}

// gatewayServicePort returns the port the upstream HTTPRoute backend
// targets on the gateway Service.
func gatewayServicePort(gateway *corev1alpha1.Gateway) int32 {
	switch {
	case gateway.Spec.Service != nil && gateway.Spec.Service.Port > 0:
		return gateway.Spec.Service.Port
	case gateway.Spec.Server != nil && gateway.Spec.Server.Port > 0:
		return gateway.Spec.Server.Port
	case (gateway.Spec.Server != nil && gateway.Spec.Server.TLS != nil && gateway.Spec.Server.TLS.Enabled) ||
		tlsEnabled(gateway):
		return 8443
	default:
		return 8080
	}
}

func tlsSpec(gateway *corev1alpha1.Gateway) *corev1alpha1.RoutingTLSSpec {
	if gateway.Spec.Routing == nil || gateway.Spec.Routing.Gateway == nil {
		return nil
	}
	return gateway.Spec.Routing.Gateway.TLS
}

func tlsEnabled(gateway *corev1alpha1.Gateway) bool {
	t := tlsSpec(gateway)
	return t != nil && t.Enabled
}

func tlsSecretName(gateway *corev1alpha1.Gateway) string {
	t := tlsSpec(gateway)
	if t != nil && t.SecretName != "" {
		return t.SecretName
	}
	return fmt.Sprintf("%s-tls", gateway.Name)
}

func isAdvancedRoutingMode(gateway *corev1alpha1.Gateway) bool {
	return gateway.Spec.Routing != nil &&
		gateway.Spec.Routing.Gateway != nil &&
		len(gateway.Spec.Routing.Gateway.ParentRefs) > 0
}

// shouldWatchNamespace checks if the operator should watch resources in the given namespace
// based on WATCH_NAMESPACE_SELECTOR environment variable
func (r *GatewayReconciler) shouldWatchNamespace(ctx context.Context, namespace string) bool {
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

// reconcileHPA handles HPA creation, update, or deletion based on Gateway spec
func (r *GatewayReconciler) reconcileHPA(ctx context.Context, gateway *corev1alpha1.Gateway, deployment *appsv1.Deployment) error {
	logger := log.FromContext(ctx)

	var hpaEnabled bool
	if gateway.Spec.HPA != nil {
		hpaEnabled = gateway.Spec.HPA.Enabled
	}
	logger.V(1).Info("Starting HPA reconciliation", "gateway", gateway.Name, "hpaEnabled", hpaEnabled)

	hpaName := fmt.Sprintf("%s-hpa", gateway.Name)
	existingHPA := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Get(ctx, types.NamespacedName{Name: hpaName, Namespace: gateway.Namespace}, existingHPA)
	hpaExists := err == nil

	logger.V(1).Info("HPA existence check", "hpaName", hpaName, "hpaExists", hpaExists, "getError", err)

	if gateway.Spec.HPA == nil || !gateway.Spec.HPA.Enabled {
		logger.V(1).Info("HPA is disabled or not configured", "hpaConfigured", gateway.Spec.HPA != nil)
		if !hpaExists {
			return nil
		}

		logger.V(1).Info("Deleting HPA as it's disabled", "hpa", hpaName)
		if err := r.Delete(ctx, existingHPA); err != nil {
			return fmt.Errorf("failed to delete HPA: %w", err)
		}
		return nil
	}

	logger.V(1).Info("HPA is enabled, proceeding with creation/update", "enabled", gateway.Spec.HPA.Enabled)

	hpa := r.buildHPA(gateway, deployment)

	if !hpaExists {
		logger.Info("Creating HPA", "hpa", hpaName, "hpaSpec", hpa.Spec)
		if err := controllerutil.SetControllerReference(gateway, hpa, r.Scheme); err != nil {
			logger.Error(err, "Failed to set controller reference for HPA", "hpa", hpaName)
			return fmt.Errorf("failed to set controller reference: %w", err)
		}

		if err := r.Create(ctx, hpa); err != nil {
			logger.Error(err, "Failed to create HPA", "hpa", hpaName)
			return fmt.Errorf("failed to create HPA: %w", err)
		}
		logger.Info("Successfully created HPA", "hpa", hpaName)
		logger.V(1).Info("HPA reconciliation completed successfully", "hpa", hpaName)
		return nil
	}

	if reflect.DeepEqual(existingHPA.Spec, hpa.Spec) {
		logger.V(1).Info("HPA already up to date", "hpa", hpaName)
		logger.V(1).Info("HPA reconciliation completed successfully", "hpa", hpaName)
		return nil
	}

	logger.Info("Updating HPA", "hpa", hpaName)
	existingHPA.Spec = hpa.Spec
	if err := r.Update(ctx, existingHPA); err != nil {
		logger.Error(err, "Failed to update HPA", "hpa", hpaName)
		return fmt.Errorf("failed to update HPA: %w", err)
	}
	logger.Info("Successfully updated HPA", "hpa", hpaName)
	logger.V(1).Info("HPA reconciliation completed successfully", "hpa", hpaName)
	return nil
}

// buildHPA creates an HPA resource based on Gateway spec
func (r *GatewayReconciler) buildHPA(gateway *corev1alpha1.Gateway, deployment *appsv1.Deployment) *autoscalingv2.HorizontalPodAutoscaler {
	if gateway.Spec.HPA == nil || !gateway.Spec.HPA.Enabled {
		return nil
	}

	var minReplicas *int32
	var maxReplicas int32
	var metrics []autoscalingv2.MetricSpec
	var behavior *autoscalingv2.HorizontalPodAutoscalerBehavior

	if gateway.Spec.HPA.Config == nil {
		defaultMin := int32(1)
		defaultMax := int32(10)
		defaultAvgUtil := int32(80)
		minReplicas = &defaultMin
		maxReplicas = defaultMax
		metrics = []autoscalingv2.MetricSpec{
			{
				Type: autoscalingv2.ResourceMetricSourceType,
				Resource: &autoscalingv2.ResourceMetricSource{
					Name: corev1.ResourceCPU,
					Target: autoscalingv2.MetricTarget{
						Type:               autoscalingv2.UtilizationMetricType,
						AverageUtilization: &defaultAvgUtil,
					},
				},
			},
		}
		behavior = nil
	} else {
		minReplicas = gateway.Spec.HPA.Config.MinReplicas
		maxReplicas = gateway.Spec.HPA.Config.MaxReplicas
		metrics = gateway.Spec.HPA.Config.Metrics
		behavior = gateway.Spec.HPA.Config.Behavior
	}

	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-hpa", gateway.Name),
			Namespace: gateway.Namespace,
			Labels: map[string]string{
				"app": gateway.Name,
			},
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       deployment.Name,
			},
			MinReplicas: minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
			Behavior:    behavior,
		},
	}

	return hpa
}

// buildContainerPorts returns the container ports for the gateway container
func (r *GatewayReconciler) buildContainerPorts(gateway *corev1alpha1.Gateway) []corev1.ContainerPort {
	if gateway == nil || gateway.Spec.Server == nil {
		return []corev1.ContainerPort{
			{
				ContainerPort: 8080,
				Name:          "http",
			},
		}
	}

	ports := []corev1.ContainerPort{
		{
			ContainerPort: gateway.Spec.Server.Port,
			Name:          "http",
		},
	}

	if gateway.Spec.Server.TLS != nil && gateway.Spec.Server.TLS.Enabled {
		ports = append(ports, corev1.ContainerPort{
			ContainerPort: 8443,
			Name:          "https",
		})
	}

	if gateway.Spec.Telemetry != nil && gateway.Spec.Telemetry.Enabled &&
		gateway.Spec.Telemetry.Metrics != nil && gateway.Spec.Telemetry.Metrics.Enabled {
		metricsPort := int32(9464)
		if gateway.Spec.Telemetry.Metrics.Port > 0 {
			metricsPort = gateway.Spec.Telemetry.Metrics.Port
		}
		ports = append(ports, corev1.ContainerPort{
			ContainerPort: metricsPort,
			Name:          "metrics",
		})
	}

	return ports
}

// reconcileRBAC manages RBAC resources (ServiceAccount) for the Gateway
func (r *GatewayReconciler) reconcileRBAC(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)

	shouldCreate := gateway.Spec.ServiceAccount == nil || gateway.Spec.ServiceAccount.Create

	if !shouldCreate {
		logger.V(1).Info("RBAC creation disabled for Gateway", "gateway", gateway.Name)
		return nil
	}

	serviceAccountName := gateway.Name
	if gateway.Spec.ServiceAccount != nil && gateway.Spec.ServiceAccount.Name != "" {
		serviceAccountName = gateway.Spec.ServiceAccount.Name
	}

	err := r.reconcileServiceAccount(ctx, gateway, serviceAccountName)
	if err != nil {
		return fmt.Errorf("failed to reconcile ServiceAccount: %w", err)
	}

	err = r.updateServiceAccountStatus(ctx, gateway, serviceAccountName)
	if err != nil {
		return fmt.Errorf("failed to update service account status: %w", err)
	}

	logger.Info("Successfully reconciled RBAC resources", "gateway", gateway.Name, "serviceAccount", serviceAccountName)
	return nil
}

// reconcileServiceAccount creates or updates the ServiceAccount for the Gateway
func (r *GatewayReconciler) reconcileServiceAccount(ctx context.Context, gateway *corev1alpha1.Gateway, serviceAccountName string) error {
	logger := log.FromContext(ctx)

	serviceAccount := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceAccountName,
			Namespace: gateway.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "inference-gateway",
				"app.kubernetes.io/instance":   gateway.Name,
				"app.kubernetes.io/component":  "gateway",
				"app.kubernetes.io/managed-by": "inference-gateway-operator",
			},
		},
	}

	// Set owner reference
	err := controllerutil.SetControllerReference(gateway, serviceAccount, r.Scheme)
	if err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Check if ServiceAccount already exists
	existing := &corev1.ServiceAccount{}
	err = r.Get(ctx, types.NamespacedName{Name: serviceAccountName, Namespace: gateway.Namespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			// Create new ServiceAccount
			err = r.Create(ctx, serviceAccount)
			if err != nil {
				logger.Error(err, "Failed to create ServiceAccount", "serviceAccount", serviceAccountName)
				return fmt.Errorf("failed to create ServiceAccount: %w", err)
			}
			logger.Info("Successfully created ServiceAccount", "serviceAccount", serviceAccountName)
		} else {
			return fmt.Errorf("failed to get ServiceAccount: %w", err)
		}
	} else {
		// Update existing ServiceAccount if needed
		existing.Labels = serviceAccount.Labels
		err = r.Update(ctx, existing)
		if err != nil {
			logger.Error(err, "Failed to update ServiceAccount", "serviceAccount", serviceAccountName)
			return fmt.Errorf("failed to update ServiceAccount: %w", err)
		}
		logger.V(1).Info("Successfully updated ServiceAccount", "serviceAccount", serviceAccountName)
	}

	return nil
}

// updateServiceAccountStatus updates the Gateway status with the service account name
func (r *GatewayReconciler) updateServiceAccountStatus(ctx context.Context, gateway *corev1alpha1.Gateway, serviceAccountName string) error {
	const maxRetries = 3
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		latest := &corev1alpha1.Gateway{}
		if err := r.Get(ctx, client.ObjectKeyFromObject(gateway), latest); err != nil {
			return err
		}

		if latest.Status.ServiceAccountName == serviceAccountName {
			return nil
		}

		latest.Status.ServiceAccountName = serviceAccountName
		if err := r.Status().Update(ctx, latest); err != nil {
			if errors.IsConflict(err) {
				lastErr = err
				continue
			}
			return err
		}
		return nil
	}
	return lastErr
}

// getServiceAccountName returns the service account name for the Gateway
func (r *GatewayReconciler) getServiceAccountName(gateway *corev1alpha1.Gateway) string {
	if gateway.Spec.ServiceAccount != nil && !gateway.Spec.ServiceAccount.Create {
		if gateway.Spec.ServiceAccount.Name != "" {
			return gateway.Spec.ServiceAccount.Name
		}
		return gateway.Name
	}

	if gateway.Spec.ServiceAccount != nil && gateway.Spec.ServiceAccount.Name != "" {
		return gateway.Spec.ServiceAccount.Name
	}

	return gateway.Name
}

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Gateway{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&gwapiv1.Gateway{}).
		Owns(&gwapiv1.HTTPRoute{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&corev1.ServiceAccount{}).
		Watches(
			&corev1alpha1.MCP{},
			handler.EnqueueRequestsFromMapFunc(r.mcpToGatewayRequests),
		).
		Complete(r)
}

// assembleMCPServerURLs returns the union of static spec.mcp.servers[].url entries
// and URLs of MCP CRs discovered via spec.mcp.serviceDiscovery, deduped on URL and
// sorted for determinism. Discovery errors are logged and ignored so a transient
// API failure does not blank out the static list.
func (r *GatewayReconciler) assembleMCPServerURLs(ctx context.Context, gateway *corev1alpha1.Gateway) []string {
	logger := log.FromContext(ctx)
	seen := map[string]struct{}{}
	urls := make([]string, 0, len(gateway.Spec.MCP.Servers))

	for _, s := range gateway.Spec.MCP.Servers {
		if s.URL == "" {
			continue
		}
		if _, ok := seen[s.URL]; ok {
			continue
		}
		seen[s.URL] = struct{}{}
		urls = append(urls, s.URL)
	}

	if gateway.Spec.MCP.ServiceDiscovery != nil && gateway.Spec.MCP.ServiceDiscovery.Enabled {
		discovered, err := r.discoverMCPs(ctx, gateway)
		if err != nil {
			logger.Error(err, "failed to discover MCPs; using static MCP_SERVERS only")
		}
		for _, mcp := range discovered {
			url := gatewayMCPURL(&mcp)
			if url == "" {
				continue
			}
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			urls = append(urls, url)
		}
	}

	sort.Strings(urls)
	return urls
}

// discoverMCPs lists MCP CRs in the configured namespace filtered by the label selector.
func (r *GatewayReconciler) discoverMCPs(ctx context.Context, gateway *corev1alpha1.Gateway) ([]corev1alpha1.MCP, error) {
	sd := gateway.Spec.MCP.ServiceDiscovery
	ns := sd.Namespace
	if ns == "" {
		ns = gateway.Namespace
	}

	listOpts := []client.ListOption{client.InNamespace(ns)}

	if sd.Selector != nil {
		selector, err := metav1.LabelSelectorAsSelector(sd.Selector)
		if err != nil {
			return nil, fmt.Errorf("invalid label selector: %w", err)
		}
		listOpts = append(listOpts, client.MatchingLabelsSelector{Selector: selector})
	}

	var mcpList corev1alpha1.MCPList
	if err := r.List(ctx, &mcpList, listOpts...); err != nil {
		return nil, err
	}

	return mcpList.Items, nil
}

// gatewayMCPURL returns the URL for an MCP CR. It prefers Status.URL (already TLS-
// and path-aware, populated by the MCP controller) and falls back to a deterministic
// construction when status has not been populated yet.
func gatewayMCPURL(mcp *corev1alpha1.MCP) string {
	if mcp.Status.URL != "" {
		return mcp.Status.URL
	}
	scheme := "http"
	var port int32 = 8080
	path := "/mcp"
	if mcp.Spec.Server != nil {
		if mcp.Spec.Server.Port != 0 {
			port = mcp.Spec.Server.Port
		}
		if mcp.Spec.Server.TLS != nil && mcp.Spec.Server.TLS.Enabled {
			scheme = "https"
		}
		if mcp.Spec.Server.Path != "" {
			path = mcp.Spec.Server.Path
		}
	}
	return fmt.Sprintf("%s://%s-service.%s.svc.cluster.local:%d%s", scheme, mcp.Name, mcp.Namespace, port, path)
}

// mcpToGatewayRequests maps an MCP event to the set of Gateway reconcile requests
// whose MCP service discovery configuration selects that MCP.
func (r *GatewayReconciler) mcpToGatewayRequests(ctx context.Context, obj client.Object) []ctrl.Request {
	mcp, ok := obj.(*corev1alpha1.MCP)
	if !ok {
		return nil
	}

	var gwList corev1alpha1.GatewayList
	if err := r.List(ctx, &gwList); err != nil {
		return nil
	}

	var requests []ctrl.Request
	for _, gateway := range gwList.Items {
		if gateway.Spec.MCP == nil || !gateway.Spec.MCP.Enabled {
			continue
		}
		if gateway.Spec.MCP.ServiceDiscovery == nil || !gateway.Spec.MCP.ServiceDiscovery.Enabled {
			continue
		}

		ns := gateway.Spec.MCP.ServiceDiscovery.Namespace
		if ns == "" {
			ns = gateway.Namespace
		}
		if ns != mcp.Namespace {
			continue
		}

		if gateway.Spec.MCP.ServiceDiscovery.Selector != nil {
			selector, err := metav1.LabelSelectorAsSelector(gateway.Spec.MCP.ServiceDiscovery.Selector)
			if err != nil {
				continue
			}
			if !selector.Matches(labels.Set(mcp.Labels)) {
				continue
			}
		}

		requests = append(requests, ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      gateway.Name,
				Namespace: gateway.Namespace,
			},
		})
	}
	return requests
}
