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
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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

// GatewayReconciler reconciles a Gateway object
type GatewayReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gateways,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gateways/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gateways/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list

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

	configMap, err := r.reconcileConfigMap(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to reconcile ConfigMap")
		return ctrl.Result{}, err
	}

	deployment, err := r.reconcileDeployment(ctx, gateway, configMap)
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

	var configuredProviderNames []string
	for _, p := range gateway.Spec.Providers {
		if p.Config != nil && p.Config.TokenRef != nil && p.Config.TokenRef.Name != "" && p.Config.TokenRef.Key != "" {
			secret := &corev1.Secret{}
			secretNamespacedName := types.NamespacedName{Name: p.Config.TokenRef.Name, Namespace: gateway.Namespace}
			err := r.Get(ctx, secretNamespacedName, secret)
			if err != nil {
				continue
			}
			apiKeyBytes, ok := secret.Data[p.Config.TokenRef.Key]
			if !ok || len(apiKeyBytes) == 0 {
				continue
			}
			configuredProviderNames = append(configuredProviderNames, p.Name)
		}
	}
	summary := strings.Join(configuredProviderNames, ",")
	if gateway.Status.ProviderSummary != summary {
		gateway.Status.ProviderSummary = summary
		if err := r.Status().Update(ctx, gateway); err != nil {
			logger.Error(err, "Failed to update ProviderSummary in status")
			return ctrl.Result{}, err
		}
	}

	logger.Info("Reconciled Gateway successfully",
		"gateway", gateway.Name,
		"deployment", deployment.Name,
		"configMap", configMap.Name)

	return ctrl.Result{}, nil
}

// generateConfig converts the Gateway spec to a YAML configuration
func (r *GatewayReconciler) generateConfig(gateway *corev1alpha1.Gateway) (string, error) { // nolint:unparam
	builder := strings.Builder{}

	// Core configuration
	r.addCoreConfig(&builder, gateway)

	// Telemetry configuration
	r.addTelemetryConfig(&builder, gateway)

	// Authentication configuration
	r.addAuthConfig(&builder, gateway)

	// Server configuration
	r.addServerConfig(&builder, gateway)

	// Provider configuration
	r.addProviderConfig(&builder, gateway)

	// MCP configuration
	r.addMCPConfig(&builder, gateway)

	// A2A configuration
	r.addA2AConfig(&builder, gateway)

	return builder.String(), nil
}

// addCoreConfig adds basic gateway configuration
func (r *GatewayReconciler) addCoreConfig(builder *strings.Builder, gateway *corev1alpha1.Gateway) {
	fmt.Fprintf(builder, "environment: %s\n", gateway.Spec.Environment)
}

// addTelemetryConfig adds telemetry configuration
func (r *GatewayReconciler) addTelemetryConfig(builder *strings.Builder, gateway *corev1alpha1.Gateway) {
	if gateway.Spec.Telemetry != nil {
		builder.WriteString("telemetry:\n")
		fmt.Fprintf(builder, "  enabled: %t\n", gateway.Spec.Telemetry.Enabled)
		if gateway.Spec.Telemetry.Metrics != nil {
			builder.WriteString("  metrics:\n")
			fmt.Fprintf(builder, "    enabled: %t\n", gateway.Spec.Telemetry.Metrics.Enabled)
			fmt.Fprintf(builder, "    port: %d\n", gateway.Spec.Telemetry.Metrics.Port)
		}
		if gateway.Spec.Telemetry.Tracing != nil {
			builder.WriteString("  tracing:\n")
			fmt.Fprintf(builder, "    enabled: %t\n", gateway.Spec.Telemetry.Tracing.Enabled)
			if gateway.Spec.Telemetry.Tracing.Endpoint != "" {
				fmt.Fprintf(builder, "    endpoint: %s\n", gateway.Spec.Telemetry.Tracing.Endpoint)
			}
		}
	}
}

// addAuthConfig adds authentication configuration
func (r *GatewayReconciler) addAuthConfig(builder *strings.Builder, gateway *corev1alpha1.Gateway) {
	if gateway.Spec.Auth != nil {
		builder.WriteString("auth:\n")
		fmt.Fprintf(builder, "  enabled: %t\n", gateway.Spec.Auth.Enabled)
		fmt.Fprintf(builder, "  provider: %s\n", gateway.Spec.Auth.Provider)
		if gateway.Spec.Auth.OIDC != nil {
			builder.WriteString("  oidc:\n")
			fmt.Fprintf(builder, "    issuerUrl: %s\n", gateway.Spec.Auth.OIDC.IssuerURL)
			fmt.Fprintf(builder, "    clientId: %s\n", gateway.Spec.Auth.OIDC.ClientID)
		}
	}
}

// addServerConfig adds server configuration
func (r *GatewayReconciler) addServerConfig(builder *strings.Builder, gateway *corev1alpha1.Gateway) {
	if gateway.Spec.Server != nil {
		builder.WriteString("server:\n")
		fmt.Fprintf(builder, "  host: %s\n", gateway.Spec.Server.Host)
		fmt.Fprintf(builder, "  port: %d\n", gateway.Spec.Server.Port)
		if gateway.Spec.Server.Timeouts != nil {
			builder.WriteString("  timeouts:\n")
			if gateway.Spec.Server.Timeouts.Read != "" {
				fmt.Fprintf(builder, "    read: %s\n", gateway.Spec.Server.Timeouts.Read)
			}
			if gateway.Spec.Server.Timeouts.Write != "" {
				fmt.Fprintf(builder, "    write: %s\n", gateway.Spec.Server.Timeouts.Write)
			}
			if gateway.Spec.Server.Timeouts.Idle != "" {
				fmt.Fprintf(builder, "    idle: %s\n", gateway.Spec.Server.Timeouts.Idle)
			}
		}
		if gateway.Spec.Server.TLS != nil && gateway.Spec.Server.TLS.Enabled {
			builder.WriteString("  tls:\n")
			builder.WriteString("    enabled: true\n")
		}
	}
}

// addProviderConfig adds provider configuration
func (r *GatewayReconciler) addProviderConfig(builder *strings.Builder, gateway *corev1alpha1.Gateway) {
	if len(gateway.Spec.Providers) > 0 {
		builder.WriteString("providers:\n")
		for _, provider := range gateway.Spec.Providers {
			fmt.Fprintf(builder, "  - name: %s\n", provider.Name)
			fmt.Fprintf(builder, "    type: %s\n", provider.Type)
			if provider.Config != nil {
				builder.WriteString("    config:\n")
				if provider.Config.BaseURL != "" {
					fmt.Fprintf(builder, "      baseUrl: %s\n", provider.Config.BaseURL)
				}
				if provider.Config.AuthType != "" {
					fmt.Fprintf(builder, "      authType: %s\n", provider.Config.AuthType)
				}
			}
		}
	}
}

// addMCPConfig adds MCP configuration
func (r *GatewayReconciler) addMCPConfig(builder *strings.Builder, gateway *corev1alpha1.Gateway) {
	if gateway.Spec.MCP != nil && gateway.Spec.MCP.Enabled {
		builder.WriteString("mcp:\n")
		fmt.Fprintf(builder, "  enabled: %t\n", gateway.Spec.MCP.Enabled)
		fmt.Fprintf(builder, "  expose: %t\n", gateway.Spec.MCP.Expose)
		r.addTimeouts(builder, "  ", gateway.Spec.MCP.Timeouts)
		r.addServers(builder, "  ", gateway.Spec.MCP.Servers)
	}
}

// addA2AConfig adds A2A configuration
func (r *GatewayReconciler) addA2AConfig(builder *strings.Builder, gateway *corev1alpha1.Gateway) {
	if gateway.Spec.A2A != nil && gateway.Spec.A2A.Enabled {
		builder.WriteString("a2a:\n")
		fmt.Fprintf(builder, "  enabled: %t\n", gateway.Spec.A2A.Enabled)
		fmt.Fprintf(builder, "  expose: %t\n", gateway.Spec.A2A.Expose)
		if gateway.Spec.A2A.Timeouts != nil {
			builder.WriteString("  timeouts:\n")
			if gateway.Spec.A2A.Timeouts.Client != "" {
				fmt.Fprintf(builder, "    client: %s\n", gateway.Spec.A2A.Timeouts.Client)
			}
		}
		r.addAgents(builder, "  ", gateway.Spec.A2A.Agents)
	}
}

// addTimeouts adds MCP timeout configuration
func (r *GatewayReconciler) addTimeouts(builder *strings.Builder, indent string, timeouts *corev1alpha1.MCPTimeouts) {
	if timeouts != nil {
		builder.WriteString(indent + "timeouts:\n")
		if timeouts.Client != "" {
			fmt.Fprintf(builder, "%s  client: %s\n", indent, timeouts.Client)
		}
	}
}

// addServers adds MCP server configuration
func (r *GatewayReconciler) addServers(builder *strings.Builder, indent string, servers []corev1alpha1.MCPServer) {
	if len(servers) > 0 {
		builder.WriteString(indent + "servers:\n")
		for _, server := range servers {
			fmt.Fprintf(builder, "%s  - name: %s\n", indent, server.Name)
			fmt.Fprintf(builder, "%s    url: %s\n", indent, server.URL)
		}
	}
}

// addAgents adds A2A agent configuration
func (r *GatewayReconciler) addAgents(builder *strings.Builder, indent string, agents []corev1alpha1.A2AAgent) {
	if len(agents) > 0 {
		builder.WriteString(indent + "agents:\n")
		for _, agent := range agents {
			fmt.Fprintf(builder, "%s  - name: %s\n", indent, agent.Name)
			fmt.Fprintf(builder, "%s    url: %s\n", indent, agent.URL)
		}
	}
}

// reconcileConfigMap ensures the ConfigMap exists and has the correct configuration
func (r *GatewayReconciler) reconcileConfigMap(ctx context.Context, gateway *corev1alpha1.Gateway) (*corev1.ConfigMap, error) {
	logger := log.FromContext(ctx)
	configMapName := gateway.Name + "-config"

	configYaml, err := r.generateConfig(gateway)
	if err != nil {
		return nil, err
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: gateway.Namespace,
		},
		Data: map[string]string{
			"config.yaml": configYaml,
		},
	}

	if err := controllerutil.SetControllerReference(gateway, configMap, r.Scheme); err != nil {
		return nil, err
	}

	found := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating ConfigMap", "ConfigMap.Name", configMap.Name)
		if err = r.Create(ctx, configMap); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if !reflect.DeepEqual(found.Data, configMap.Data) {
			found.Data = configMap.Data
			logger.Info("Updating ConfigMap", "ConfigMap.Name", configMap.Name)
			if err = r.Update(ctx, found); err != nil {
				return nil, err
			}
		}
		configMap = found
	}

	return configMap, nil
}

// reconcileDeployment ensures the Deployment exists with the correct configuration
func (r *GatewayReconciler) reconcileDeployment(ctx context.Context, gateway *corev1alpha1.Gateway, configMap *corev1.ConfigMap) (*appsv1.Deployment, error) {
	deployment := r.buildDeployment(gateway, configMap)

	if err := controllerutil.SetControllerReference(gateway, deployment, r.Scheme); err != nil {
		return nil, err
	}

	return r.createOrUpdateDeployment(ctx, gateway, deployment)
}

// buildDeployment creates a Deployment resource based on Gateway spec
func (r *GatewayReconciler) buildDeployment(gateway *corev1alpha1.Gateway, configMap *corev1.ConfigMap) *appsv1.Deployment {
	envVars := r.buildEnvVars(gateway)
	containerPorts := r.buildContainerPorts(gateway)

	volumes := r.buildVolumes(gateway, configMap)
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      gateway.Name + "-config-volume",
			MountPath: "/app/config",
		},
	}

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
					Containers: []corev1.Container{
						r.buildContainerWithMounts(gateway, envVars, containerPorts, volumeMounts),
					},
					Volumes: volumes,
				},
			},
		},
	}

	r.setDeploymentReplicas(gateway, deployment)
	return deployment
}

// buildContainerWithMounts creates the main container specification with custom volume mounts
func (r *GatewayReconciler) buildContainerWithMounts(gateway *corev1alpha1.Gateway, envVars []corev1.EnvVar, containerPorts []corev1.ContainerPort, volumeMounts []corev1.VolumeMount) corev1.Container {
	port := int32(8080)
	if gateway.Spec.Server != nil && gateway.Spec.Server.Port > 0 {
		port = gateway.Spec.Server.Port
	}

	return corev1.Container{
		Name:         "inference-gateway",
		Image:        "ghcr.io/inference-gateway/inference-gateway:latest",
		Ports:        containerPorts,
		Env:          envVars,
		VolumeMounts: volumeMounts,
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewMilliQuantity(100, resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(256*1024*1024, resource.BinarySI),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    *resource.NewQuantity(2, resource.DecimalSI),
				corev1.ResourceMemory: *resource.NewQuantity(1024*1024*1024, resource.BinarySI),
			},
		},
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

// buildVolumes creates volumes for the deployment
func (r *GatewayReconciler) buildVolumes(gateway *corev1alpha1.Gateway, configMap *corev1.ConfigMap) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: gateway.Name + "-config-volume",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMap.Name,
					},
				},
			},
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
		logger.Info("Creating Deployment", "Deployment.Name", deployment.Name)
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

		if !reflect.DeepEqual(latestDeployment.Spec.Template, desired.Spec.Template) {
			latestDeployment.Spec.Template = desired.Spec.Template
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
	hpaSpec := gateway.Spec.HPA

	minReplicas := int32(1)
	if hpaSpec.MinReplicas != nil {
		minReplicas = *hpaSpec.MinReplicas
	}

	maxReplicas := int32(10)
	if hpaSpec.MaxReplicas > 0 {
		maxReplicas = hpaSpec.MaxReplicas
	}

	targetCPUPercent := int32(80)
	if hpaSpec.TargetCPUUtilizationPercentage != nil {
		targetCPUPercent = *hpaSpec.TargetCPUUtilizationPercentage
	}

	metrics := []autoscalingv2.MetricSpec{}

	if hpaSpec.TargetCPUUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: hpaSpec.TargetCPUUtilizationPercentage,
				},
			},
		})
	}

	if hpaSpec.TargetMemoryUtilizationPercentage != nil {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceMemory,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: hpaSpec.TargetMemoryUtilizationPercentage,
				},
			},
		})
	}

	for _, customMetric := range hpaSpec.CustomMetrics {
		metricSpec := autoscalingv2.MetricSpec{}

		switch customMetric.Type {
		case "Pods":
			metricSpec.Type = autoscalingv2.PodsMetricSourceType
			metricSpec.Pods = &autoscalingv2.PodsMetricSource{
				Metric: autoscalingv2.MetricIdentifier{Name: customMetric.Name},
				Target: r.buildMetricTarget(customMetric.Target),
			}
		case "Object":
			metricSpec.Type = autoscalingv2.ObjectMetricSourceType
			// Note: Object metrics need more configuration, this is a basic structure
		case "External":
			metricSpec.Type = autoscalingv2.ExternalMetricSourceType
			metricSpec.External = &autoscalingv2.ExternalMetricSource{
				Metric: autoscalingv2.MetricIdentifier{Name: customMetric.Name},
				Target: r.buildMetricTarget(customMetric.Target),
			}
		}

		if metricSpec.Type != "" {
			metrics = append(metrics, metricSpec)
		}
	}

	if len(metrics) == 0 {
		metrics = append(metrics, autoscalingv2.MetricSpec{
			Type: autoscalingv2.ResourceMetricSourceType,
			Resource: &autoscalingv2.ResourceMetricSource{
				Name: corev1.ResourceCPU,
				Target: autoscalingv2.MetricTarget{
					Type:               autoscalingv2.UtilizationMetricType,
					AverageUtilization: &targetCPUPercent,
				},
			},
		})
	}

	behavior := &autoscalingv2.HorizontalPodAutoscalerBehavior{}

	if hpaSpec.ScaleDownStabilizationWindowSeconds != nil {
		behavior.ScaleDown = &autoscalingv2.HPAScalingRules{
			StabilizationWindowSeconds: hpaSpec.ScaleDownStabilizationWindowSeconds,
		}
	}

	if hpaSpec.ScaleUpStabilizationWindowSeconds != nil {
		behavior.ScaleUp = &autoscalingv2.HPAScalingRules{
			StabilizationWindowSeconds: hpaSpec.ScaleUpStabilizationWindowSeconds,
		}
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
			MinReplicas: &minReplicas,
			MaxReplicas: maxReplicas,
			Metrics:     metrics,
			Behavior:    behavior,
		},
	}

	return hpa
}

// buildMetricTarget converts HPAMetricTarget to autoscalingv2.MetricTarget
func (r *GatewayReconciler) buildMetricTarget(target corev1alpha1.HPAMetricTarget) autoscalingv2.MetricTarget {
	metricTarget := autoscalingv2.MetricTarget{}

	switch target.Type {
	case "Utilization":
		metricTarget.Type = autoscalingv2.UtilizationMetricType
		metricTarget.AverageUtilization = target.AverageUtilization
	case "Value":
		metricTarget.Type = autoscalingv2.ValueMetricType
		if target.Value != "" {
			if value, err := resource.ParseQuantity(target.Value); err == nil {
				metricTarget.Value = &value
			}
		}
	case "AverageValue":
		metricTarget.Type = autoscalingv2.AverageValueMetricType
		if target.Value != "" {
			if value, err := resource.ParseQuantity(target.Value); err == nil {
				metricTarget.AverageValue = &value
			}
		}
	}

	return metricTarget
}

// buildEnvVars returns the environment variables for the gateway container
func (r *GatewayReconciler) buildEnvVars(gateway *corev1alpha1.Gateway) []corev1.EnvVar {
	if gateway == nil {
		return nil
	}

	envVars := []corev1.EnvVar{
		{
			Name:  "ENVIRONMENT",
			Value: gateway.Spec.Environment,
		},
	}

	if gateway.Spec.Auth != nil && gateway.Spec.Auth.Enabled && gateway.Spec.Auth.OIDC != nil && gateway.Spec.Auth.OIDC.ClientSecretRef != nil {
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

	for _, provider := range gateway.Spec.Providers {
		if provider.Config != nil && provider.Config.TokenRef != nil {
			name := strings.ToUpper(provider.Name) + "_API_KEY"
			envVars = append(envVars, corev1.EnvVar{
				Name: name,
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: provider.Config.TokenRef.Name,
						},
						Key: provider.Config.TokenRef.Key,
					},
				},
			})
		}
	}

	return envVars
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

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Gateway{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.Ingress{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Complete(r)
}
