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

	appsv1 "k8s.io/api/apps/v1"
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
		logger.Error(err, "Failed to reconcile Deployment")
		return ctrl.Result{}, err
	}

	err = r.reconcileService(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to reconcile Service")
		return ctrl.Result{}, err
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
	logger := log.FromContext(ctx)

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

	if len(gateway.Spec.Providers) > 0 {
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
	}

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

	replicas := int32(1)
	if gateway.Spec.Replicas != nil {
		replicas = *gateway.Spec.Replicas
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gateway.Name,
			Namespace: gateway.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
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
						{
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
						},
					},
					Volumes: []corev1.Volume{
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
					},
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(gateway, deployment, r.Scheme); err != nil {
		return nil, err
	}

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("Creating Deployment", "Deployment.Name", deployment.Name)
		if err = r.Create(ctx, deployment); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		if !reflect.DeepEqual(found.Spec, deployment.Spec) {
			found.Spec = deployment.Spec
			logger.Info("Updating Deployment", "Deployment.Name", deployment.Name)
			if err = r.Update(ctx, found); err != nil {
				return nil, err
			}
		}
		deployment = found
	}

	return deployment, nil
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

// SetupWithManager sets up the controller with the Manager.
func (r *GatewayReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1alpha1.Gateway{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
