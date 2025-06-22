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
// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=get;list;watch;create;update;patch;delete

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

	logger.Info("About to reconcile Service", "gateway", gateway.Name)
	err = r.reconcileService(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to reconcile Service")
		return ctrl.Result{}, err
	}
	logger.Info("Service reconciliation completed", "gateway", gateway.Name)

	logger.Info("About to reconcile HPA", "gateway", gateway.Name, "hpaConfig", gateway.Spec.HPA)
	err = r.reconcileHPA(ctx, gateway, deployment)
	if err != nil {
		logger.Error(err, "Failed to reconcile HPA")
		return ctrl.Result{}, err
	}
	logger.Info("HPA reconciliation completed", "gateway", gateway.Name)

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
						r.buildContainer(gateway, envVars, containerPorts),
					},
					Volumes: r.buildVolumes(gateway, configMap),
				},
			},
		},
	}

	r.setDeploymentReplicas(gateway, deployment)
	return deployment
}

// buildEnvVars creates environment variables for the container
func (r *GatewayReconciler) buildEnvVars(gateway *corev1alpha1.Gateway) []corev1.EnvVar {
	var envVars []corev1.EnvVar

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
			envVars = append(envVars, corev1.EnvVar{
				Name: fmt.Sprintf("%s_API_KEY", toUpperSnakeCase(provider.Name)),
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

// buildContainerPorts creates container ports based on Gateway configuration
func (r *GatewayReconciler) buildContainerPorts(gateway *corev1alpha1.Gateway) []corev1.ContainerPort {
	port := int32(8080)
	if gateway.Spec.Server != nil && gateway.Spec.Server.Port > 0 {
		port = gateway.Spec.Server.Port
	}

	containerPorts := []corev1.ContainerPort{
		{
			ContainerPort: port,
			Name:          "http",
		},
	}

	if gateway.Spec.Telemetry != nil && gateway.Spec.Telemetry.Enabled && gateway.Spec.Telemetry.Metrics != nil && gateway.Spec.Telemetry.Metrics.Enabled {
		metricsPort := int32(9464)
		if gateway.Spec.Telemetry.Metrics.Port > 0 {
			metricsPort = gateway.Spec.Telemetry.Metrics.Port
		}
		containerPorts = append(containerPorts, corev1.ContainerPort{
			ContainerPort: metricsPort,
			Name:          "metrics",
		})
	}

	return containerPorts
}

// buildContainer creates the main container specification
func (r *GatewayReconciler) buildContainer(gateway *corev1alpha1.Gateway, envVars []corev1.EnvVar, containerPorts []corev1.ContainerPort) corev1.Container {
	port := int32(8080)
	if gateway.Spec.Server != nil && gateway.Spec.Server.Port > 0 {
		port = gateway.Spec.Server.Port
	}

	return corev1.Container{
		Name:  "inference-gateway",
		Image: "ghcr.io/inference-gateway/inference-gateway:latest",
		Ports: containerPorts,
		Env:   envVars,
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      gateway.Name + "-config-volume",
				MountPath: "/app/config",
			},
		},
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
				logger.Info("HPA is enabled, not modifying replicas field",
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

	port := int32(8080)
	if gateway.Spec.Server != nil && gateway.Spec.Server.Port > 0 {
		port = gateway.Spec.Server.Port
	}

	servicePorts := []corev1.ServicePort{
		{
			Port:       port,
			TargetPort: intstr.FromInt(int(port)),
			Protocol:   corev1.ProtocolTCP,
			Name:       "http",
		},
	}

	if gateway.Spec.Telemetry != nil && gateway.Spec.Telemetry.Enabled && gateway.Spec.Telemetry.Metrics != nil && gateway.Spec.Telemetry.Metrics.Enabled {
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

// toUpperSnakeCase converts a camelCase or kebab-case string to UPPER_SNAKE_CASE
func toUpperSnakeCase(s string) string {
	result := ""
	for i, c := range s {
		if i > 0 && ((c >= 'A' && c <= 'Z') || c == '-') {
			if c == '-' {
				result += "_"
			} else {
				result += "_" + string(c)
			}
		} else {
			if c >= 'a' && c <= 'z' {
				result += string(c - 32)
			} else {
				result += string(c)
			}
		}
	}
	return result
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
	logger.Info("Starting HPA reconciliation", "gateway", gateway.Name, "hpaEnabled", hpaEnabled)

	hpaName := fmt.Sprintf("%s-hpa", gateway.Name)
	existingHPA := &autoscalingv2.HorizontalPodAutoscaler{}
	err := r.Get(ctx, types.NamespacedName{Name: hpaName, Namespace: gateway.Namespace}, existingHPA)
	hpaExists := err == nil

	logger.Info("HPA existence check", "hpaName", hpaName, "hpaExists", hpaExists, "getError", err)

	if gateway.Spec.HPA == nil || !gateway.Spec.HPA.Enabled {
		logger.Info("HPA is disabled or not configured", "hpaConfigured", gateway.Spec.HPA != nil)
		if hpaExists {
			logger.Info("Deleting HPA as it's disabled", "hpa", hpaName)
			if err := r.Delete(ctx, existingHPA); err != nil {
				return fmt.Errorf("failed to delete HPA: %w", err)
			}
		}
		return nil
	}

	logger.Info("HPA is enabled, proceeding with creation/update", "enabled", gateway.Spec.HPA.Enabled)

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
	} else {
		if !reflect.DeepEqual(existingHPA.Spec, hpa.Spec) {
			logger.Info("Updating HPA", "hpa", hpaName)
			existingHPA.Spec = hpa.Spec
			if err := r.Update(ctx, existingHPA); err != nil {
				logger.Error(err, "Failed to update HPA", "hpa", hpaName)
				return fmt.Errorf("failed to update HPA: %w", err)
			}
			logger.Info("Successfully updated HPA", "hpa", hpaName)
		} else {
			logger.Info("HPA already up to date", "hpa", hpaName)
		}
	}

	logger.Info("HPA reconciliation completed successfully", "hpa", hpaName)
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

	// If no metrics are specified, default to CPU
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

	// Build behavior configuration
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

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Gateway{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Complete(r)
}
