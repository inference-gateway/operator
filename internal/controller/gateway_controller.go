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
	"strconv"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
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
	log "sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=a2as,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete

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

	err = r.reconcileIngress(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to reconcile Ingress")
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
		ingressSpec := gw.Spec.Ingress
		var host, scheme string

		if ingressSpec != nil && ingressSpec.Enabled {
			ingress := &networkingv1.Ingress{}
			err := r.Get(ctx, types.NamespacedName{Name: gw.Name, Namespace: gw.Namespace}, ingress)
			if err != nil || len(ingress.Spec.Rules) == 0 {
				logger.V(1).Info("ingress not found or has no rules, falling back to service fqdn", "gateway", gw.Name)
				host = fmt.Sprintf("%s.%s.svc.cluster.local", gw.Name, gw.Namespace)
			} else {
				host = ingress.Spec.Rules[0].Host
			}
			switch {
			case ingressSpec.TLS != nil && ingressSpec.TLS.Enabled:
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
		if latest.Status.URL == newURL || newURL == "" {
			return nil
		}

		latest.Status.URL = newURL
		if err := r.Status().Update(ctx, latest); err != nil {
			if errors.IsConflict(err) {
				lastErr = err
				continue
			}
			logger.Error(err, "failed to update gateway url in status")
			return err
		}
		logger.V(1).Info("updated gateway url in status", "gateway", latest.Name, "url", newURL)
		return nil
	}
	return lastErr
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
				Name: "MCP_SERVERS",
				Value: strings.Join(func() []string {
					var servers []string
					for _, s := range gateway.Spec.MCP.Servers {
						servers = append(servers, s.URL)
					}
					return servers
				}(), ","),
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

	if gateway.Spec.A2A != nil && gateway.Spec.A2A.Enabled {
		envVars = append(envVars,
			corev1.EnvVar{
				Name:  "A2A_ENABLE",
				Value: fmt.Sprintf("%t", gateway.Spec.A2A.Enabled),
			},
			corev1.EnvVar{
				Name:  "A2A_EXPOSE",
				Value: fmt.Sprintf("%t", gateway.Spec.A2A.Expose),
			},
			corev1.EnvVar{
				Name: "A2A_AGENTS",
				Value: strings.Join(func() []string {
					var agents []string
					for _, a := range gateway.Spec.A2A.Agents {
						agents = append(agents, a.URL)
					}
					return agents
				}(), ","),
			},
			corev1.EnvVar{
				Name: "A2A_CLIENT_TIMEOUT",
				Value: func() string {
					if gateway.Spec.A2A.Timeouts != nil {
						return gateway.Spec.A2A.Timeouts.Client
					}
					return "5s"
				}(),
			},
		)

		if gateway.Spec.A2A.ServiceDiscovery != nil && gateway.Spec.A2A.ServiceDiscovery.Enabled {
			namespace := gateway.Spec.A2A.ServiceDiscovery.Namespace
			if namespace == "" {
				namespace = "default"
			}

			envVars = append(envVars,
				corev1.EnvVar{
					Name:  "A2A_SERVICE_DISCOVERY_ENABLE",
					Value: fmt.Sprintf("%t", gateway.Spec.A2A.ServiceDiscovery.Enabled),
				},
				corev1.EnvVar{
					Name:  "A2A_SERVICE_DISCOVERY_NAMESPACE",
					Value: namespace,
				},
				corev1.EnvVar{
					Name: "A2A_SERVICE_DISCOVERY_POLLING_INTERVAL",
					Value: func() string {
						if gateway.Spec.A2A.ServiceDiscovery.PollingInterval != "" {
							return gateway.Spec.A2A.ServiceDiscovery.PollingInterval
						}
						return "30s"
					}(),
				},
			)
		}
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

// reconcileIngress ensures the Ingress exists with the correct configuration
func (r *GatewayReconciler) reconcileIngress(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)
	logger.V(1).Info("Reconciling Ingress", "Gateway.Name", gateway.Name, "Gateway.Namespace", gateway.Namespace)

	if gateway.Spec.Ingress == nil || !gateway.Spec.Ingress.Enabled {
		ingress := &networkingv1.Ingress{}
		err := r.Get(ctx, types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}, ingress)
		if err == nil {
			logger.Info("Deleting Ingress (ingress disabled)", "Ingress.Name", ingress.Name)
			return r.Delete(ctx, ingress)
		} else if !errors.IsNotFound(err) {
			return err
		}
		return nil
	}

	ingress := r.buildIngress(gateway)
	if err := controllerutil.SetControllerReference(gateway, ingress, r.Scheme); err != nil {
		return err
	}

	found := &networkingv1.Ingress{}
	err := r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: ingress.Namespace}, found)

	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating Ingress", "Ingress.Name", ingress.Name)
		if err = r.Create(ctx, ingress); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if !reflect.DeepEqual(found.Spec, ingress.Spec) || !reflect.DeepEqual(found.Annotations, ingress.Annotations) {
			found.Spec = ingress.Spec
			found.Annotations = ingress.Annotations
			logger.Info("Updating Ingress", "Ingress.Name", ingress.Name)
			if err = r.Update(ctx, found); err != nil {
				return err
			}
		}
	}

	return nil
}

// buildIngress creates an Ingress resource based on Gateway spec
func (r *GatewayReconciler) buildIngress(gateway *corev1alpha1.Gateway) *networkingv1.Ingress {
	ingressSpec := gateway.Spec.Ingress

	var servicePort int32
	if gateway.Spec.Service != nil && gateway.Spec.Service.Port > 0 {
		servicePort = gateway.Spec.Service.Port
	} else if gateway.Spec.Server != nil && gateway.Spec.Server.Port > 0 {
		servicePort = gateway.Spec.Server.Port
	} else if (gateway.Spec.Server != nil && gateway.Spec.Server.TLS != nil && gateway.Spec.Server.TLS.Enabled) ||
		(ingressSpec != nil && ingressSpec.TLS != nil && ingressSpec.TLS.Enabled) {
		servicePort = 8443
	} else {
		servicePort = 8080
	}

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
			Labels: map[string]string{
				"app": gateway.Name,
			},
			Annotations: r.buildIngressAnnotations(gateway),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: r.getIngressClassName(ingressSpec),
			Rules:            r.buildIngressRules(gateway, servicePort),
			TLS:              r.buildIngressTLS(gateway),
		},
	}

	return ingress
}

// buildIngressAnnotations builds annotations for the ingress
func (r *GatewayReconciler) buildIngressAnnotations(gateway *corev1alpha1.Gateway) map[string]string {
	annotations := make(map[string]string)
	ingressSpec := gateway.Spec.Ingress

	if ingressSpec.Annotations != nil {
		for k, v := range ingressSpec.Annotations {
			annotations[k] = v
		}
	}

	if ingressSpec.TLS != nil && ingressSpec.TLS.Issuer != "" {
		annotations["cert-manager.io/cluster-issuer"] = ingressSpec.TLS.Issuer
	}

	className := r.getIngressClassName(ingressSpec)
	if className != nil && *className != "nginx" {
		return annotations
	}

	// Currently supporting only NGINX Ingress Controller, to be extended later
	if gateway.Spec.Server != nil && gateway.Spec.Server.TLS != nil && !gateway.Spec.Server.TLS.Enabled {
		if _, exists := annotations["nginx.ingress.kubernetes.io/ssl-redirect"]; !exists {
			annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
		}
		if _, exists := annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"]; !exists {
			annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "true"
		}
		if _, exists := annotations["nginx.ingress.kubernetes.io/backend-protocol"]; !exists {
			annotations["nginx.ingress.kubernetes.io/backend-protocol"] = "HTTP"
		}
	} else if gateway.Spec.Server != nil && gateway.Spec.Server.TLS != nil && gateway.Spec.Server.TLS.Enabled {
		if _, exists := annotations["nginx.ingress.kubernetes.io/backend-protocol"]; !exists {
			annotations["nginx.ingress.kubernetes.io/backend-protocol"] = "HTTPS"
		}
	}

	return annotations
}

// getIngressClassName returns the ingress class name
func (r *GatewayReconciler) getIngressClassName(ingressSpec *corev1alpha1.IngressSpec) *string {
	if ingressSpec.ClassName != "" {
		return &ingressSpec.ClassName
	}

	defaultClass := "nginx"
	return &defaultClass
}

// buildIngressRules builds the ingress rules
func (r *GatewayReconciler) buildIngressRules(gateway *corev1alpha1.Gateway, servicePort int32) []networkingv1.IngressRule {
	ingressSpec := gateway.Spec.Ingress
	var rules []networkingv1.IngressRule

	if ingressSpec.Host != "" {
		rule := networkingv1.IngressRule{
			Host: ingressSpec.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: (*networkingv1.PathType)(stringPtr("Prefix")),
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: gateway.Name,
									Port: networkingv1.ServiceBackendPort{
										Number: servicePort,
									},
								},
							},
						},
					},
				},
			},
		}
		rules = append(rules, rule)
	} else if len(ingressSpec.Hosts) > 0 {
		for _, host := range ingressSpec.Hosts {
			paths := host.Paths
			if len(paths) == 0 {
				paths = []corev1alpha1.IngressPath{
					{
						Path:     "/",
						PathType: "Prefix",
					},
				}
			}

			var httpPaths []networkingv1.HTTPIngressPath
			for _, path := range paths {
				pathType := path.PathType
				if pathType == "" {
					pathType = "Prefix"
				}
				httpPaths = append(httpPaths, networkingv1.HTTPIngressPath{
					Path:     path.Path,
					PathType: (*networkingv1.PathType)(&pathType),
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: gateway.Name,
							Port: networkingv1.ServiceBackendPort{
								Number: servicePort,
							},
						},
					},
				})
			}

			rule := networkingv1.IngressRule{
				Host: host.Host,
				IngressRuleValue: networkingv1.IngressRuleValue{
					HTTP: &networkingv1.HTTPIngressRuleValue{
						Paths: httpPaths,
					},
				},
			}
			rules = append(rules, rule)
		}
	}

	return rules
}

// buildIngressTLS builds the TLS configuration for ingress
func (r *GatewayReconciler) buildIngressTLS(gateway *corev1alpha1.Gateway) []networkingv1.IngressTLS {
	ingressSpec := gateway.Spec.Ingress
	tlsConfigs := make([]networkingv1.IngressTLS, 0, 1)

	if ingressSpec.TLS == nil {
		return tlsConfigs
	}

	if ingressSpec.TLS.Enabled {
		secretName := ingressSpec.TLS.SecretName
		if secretName == "" {
			host := ingressSpec.Host
			if host == "" && len(ingressSpec.Hosts) > 0 {
				host = ingressSpec.Hosts[0].Host
			}
			if host != "" {
				secretName = fmt.Sprintf("%s-tls", gateway.Name)
			}
		}

		if secretName != "" {
			var hosts []string
			if ingressSpec.Host != "" {
				hosts = append(hosts, ingressSpec.Host)
			} else {
				for _, host := range ingressSpec.Hosts {
					hosts = append(hosts, host.Host)
				}
			}

			if len(hosts) > 0 {
				tlsConfig := networkingv1.IngressTLS{
					SecretName: secretName,
					Hosts:      hosts,
				}
				tlsConfigs = append(tlsConfigs, tlsConfig)
			}
		}
	}

	for _, tlsConfig := range ingressSpec.TLS.Config {
		tlsConfigs = append(tlsConfigs, networkingv1.IngressTLS{
			SecretName: tlsConfig.SecretName,
			Hosts:      tlsConfig.Hosts,
		})
	}

	return tlsConfigs
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
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

// reconcileRBAC manages RBAC resources (ServiceAccount, Role, RoleBinding) for the Gateway
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

	err = r.reconcileRole(ctx, gateway, serviceAccountName)
	if err != nil {
		return fmt.Errorf("failed to reconcile Role: %w", err)
	}

	err = r.reconcileRoleBinding(ctx, gateway, serviceAccountName)
	if err != nil {
		return fmt.Errorf("failed to reconcile RoleBinding: %w", err)
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

// reconcileRole creates or updates the Role for A2A discovery permissions
func (r *GatewayReconciler) reconcileRole(ctx context.Context, gateway *corev1alpha1.Gateway, serviceAccountName string) error {
	logger := log.FromContext(ctx)

	a2aNamespace := gateway.Namespace
	if gateway.Spec.A2A != nil && gateway.Spec.A2A.ServiceDiscovery != nil && gateway.Spec.A2A.ServiceDiscovery.Namespace != "" {
		a2aNamespace = gateway.Spec.A2A.ServiceDiscovery.Namespace
	}

	roleName := fmt.Sprintf("%s-a2a-discovery", serviceAccountName)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: a2aNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "inference-gateway",
				"app.kubernetes.io/instance":   gateway.Name,
				"app.kubernetes.io/component":  "gateway-rbac",
				"app.kubernetes.io/managed-by": "inference-gateway-operator",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"core.inference-gateway.com"},
				Resources: []string{"a2as"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}

	if a2aNamespace == gateway.Namespace {
		err := controllerutil.SetControllerReference(gateway, role, r.Scheme)
		if err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	existing := &rbacv1.Role{}
	err := r.Get(ctx, types.NamespacedName{Name: roleName, Namespace: a2aNamespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Create(ctx, role)
			if err != nil {
				logger.Error(err, "Failed to create Role", "role", roleName, "namespace", a2aNamespace)
				return fmt.Errorf("failed to create Role: %w", err)
			}
			logger.Info("Successfully created Role", "role", roleName, "namespace", a2aNamespace)
		} else {
			return fmt.Errorf("failed to get Role: %w", err)
		}
	} else {
		existing.Rules = role.Rules
		existing.Labels = role.Labels
		err = r.Update(ctx, existing)
		if err != nil {
			logger.Error(err, "Failed to update Role", "role", roleName, "namespace", a2aNamespace)
			return fmt.Errorf("failed to update Role: %w", err)
		}
		logger.V(1).Info("Successfully updated Role", "role", roleName, "namespace", a2aNamespace)
	}

	return nil
}

// reconcileRoleBinding creates or updates the RoleBinding
func (r *GatewayReconciler) reconcileRoleBinding(ctx context.Context, gateway *corev1alpha1.Gateway, serviceAccountName string) error {
	logger := log.FromContext(ctx)

	a2aNamespace := gateway.Namespace
	if gateway.Spec.A2A != nil && gateway.Spec.A2A.ServiceDiscovery != nil && gateway.Spec.A2A.ServiceDiscovery.Namespace != "" {
		a2aNamespace = gateway.Spec.A2A.ServiceDiscovery.Namespace
	}

	roleName := fmt.Sprintf("%s-a2a-discovery", serviceAccountName)
	roleBindingName := fmt.Sprintf("%s-a2a-discovery", serviceAccountName)

	roleBinding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleBindingName,
			Namespace: a2aNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "inference-gateway",
				"app.kubernetes.io/instance":   gateway.Name,
				"app.kubernetes.io/component":  "gateway-rbac",
				"app.kubernetes.io/managed-by": "inference-gateway-operator",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: gateway.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "Role",
			Name:     roleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	if a2aNamespace == gateway.Namespace {
		err := controllerutil.SetControllerReference(gateway, roleBinding, r.Scheme)
		if err != nil {
			return fmt.Errorf("failed to set controller reference: %w", err)
		}
	}

	existing := &rbacv1.RoleBinding{}
	err := r.Get(ctx, types.NamespacedName{Name: roleBindingName, Namespace: a2aNamespace}, existing)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.Create(ctx, roleBinding)
			if err != nil {
				logger.Error(err, "Failed to create RoleBinding", "roleBinding", roleBindingName, "namespace", a2aNamespace)
				return fmt.Errorf("failed to create RoleBinding: %w", err)
			}
			logger.Info("Successfully created RoleBinding", "roleBinding", roleBindingName, "namespace", a2aNamespace)
		} else {
			return fmt.Errorf("failed to get RoleBinding: %w", err)
		}
	} else {
		existing.Subjects = roleBinding.Subjects
		existing.RoleRef = roleBinding.RoleRef
		existing.Labels = roleBinding.Labels
		err = r.Update(ctx, existing)
		if err != nil {
			logger.Error(err, "Failed to update RoleBinding", "roleBinding", roleBindingName, "namespace", a2aNamespace)
			return fmt.Errorf("failed to update RoleBinding: %w", err)
		}
		logger.V(1).Info("Successfully updated RoleBinding", "roleBinding", roleBindingName, "namespace", a2aNamespace)
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
		Owns(&networkingv1.Ingress{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Complete(r)
}
