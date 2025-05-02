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
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

	gateway := &corev1alpha1.Gateway{}
	err := r.Get(ctx, req.NamespacedName, gateway)
	if err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request
		logger.Error(err, "Failed to get Gateway")
		return ctrl.Result{}, err
	}

	// Create or update the ConfigMap
	configMap, err := r.reconcileConfigMap(ctx, gateway)
	if err != nil {
		logger.Error(err, "Failed to reconcile ConfigMap")
		return ctrl.Result{}, err
	}

	// Create or update the Deployment
	deployment, err := r.reconcileDeployment(ctx, gateway, configMap)
	if err != nil {
		logger.Error(err, "Failed to reconcile Deployment")
		return ctrl.Result{}, err
	}

	// Create or update the Service
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

	builder.WriteString(fmt.Sprintf("environment: %s\n", gateway.Spec.Environment))
	builder.WriteString(fmt.Sprintf("enableTelemetry: %t\n", gateway.Spec.EnableTelemetry))
	builder.WriteString(fmt.Sprintf("enableAuth: %t\n", gateway.Spec.EnableAuth))

	if gateway.Spec.OIDC != nil {
		builder.WriteString("oidc:\n")
		builder.WriteString(fmt.Sprintf("  issuerUrl: %s\n", gateway.Spec.OIDC.IssuerURL))
		builder.WriteString(fmt.Sprintf("  clientId: %s\n", gateway.Spec.OIDC.ClientID))
	}

	if gateway.Spec.Server != nil {
		builder.WriteString("server:\n")
		builder.WriteString(fmt.Sprintf("  host: %s\n", gateway.Spec.Server.Host))
		builder.WriteString(fmt.Sprintf("  port: %s\n", gateway.Spec.Server.Port))
		if gateway.Spec.Server.ReadTimeout != "" {
			builder.WriteString(fmt.Sprintf("  readTimeout: %s\n", gateway.Spec.Server.ReadTimeout))
		}
		if gateway.Spec.Server.WriteTimeout != "" {
			builder.WriteString(fmt.Sprintf("  writeTimeout: %s\n", gateway.Spec.Server.WriteTimeout))
		}
		if gateway.Spec.Server.IdleTimeout != "" {
			builder.WriteString(fmt.Sprintf("  idleTimeout: %s\n", gateway.Spec.Server.IdleTimeout))
		}
	}

	if len(gateway.Spec.Providers) > 0 {
		builder.WriteString("providers:\n")
		for name, provider := range gateway.Spec.Providers {
			builder.WriteString(fmt.Sprintf("  %s:\n", name))
			builder.WriteString(fmt.Sprintf("    url: %s\n", provider.URL))
		}
	}

	return builder.String(), nil
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

	if gateway.Spec.EnableAuth && gateway.Spec.OIDC != nil && gateway.Spec.OIDC.ClientSecretRef != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "OIDC_CLIENT_SECRET",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: gateway.Spec.OIDC.ClientSecretRef.Name,
					},
					Key: gateway.Spec.OIDC.ClientSecretRef.Key,
				},
			},
		})
	}

	if gateway.Spec.Providers != nil {
		for providerName, provider := range gateway.Spec.Providers {
			if provider.TokenRef != nil {
				envVars = append(envVars, corev1.EnvVar{
					Name: fmt.Sprintf("%s_API_KEY", toUpperSnakeCase(providerName)),
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: provider.TokenRef.Name,
							},
							Key: provider.TokenRef.Key,
						},
					},
				})
			}
		}
	}

	port := int32(8080)
	if gateway.Spec.Server != nil && gateway.Spec.Server.Port != "" {
		var portInt int
		if _, err := fmt.Sscanf(gateway.Spec.Server.Port, "%d", &portInt); err == nil {
			port = int32(portInt)
		}
	}

	replicas := int32(1)
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
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: port,
									Name:          "http",
								},
							},
							Env: envVars,
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
		found.Spec = deployment.Spec
		logger.Info("Updating Deployment", "Deployment.Name", deployment.Name)
		if err = r.Update(ctx, found); err != nil {
			return nil, err
		}
		deployment = found
	}

	return deployment, nil
}

// reconcileService ensures the Service exists with the correct configuration
func (r *GatewayReconciler) reconcileService(ctx context.Context, gateway *corev1alpha1.Gateway) error {
	logger := log.FromContext(ctx)

	port := int32(8080)
	if gateway.Spec.Server != nil && gateway.Spec.Server.Port != "" {
		var portInt int
		if _, err := fmt.Sscanf(gateway.Spec.Server.Port, "%d", &portInt); err == nil {
			port = int32(portInt)
		}
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
			Ports: []corev1.ServicePort{
				{
					Port:       port,
					TargetPort: intstr.FromInt(int(port)),
					Protocol:   corev1.ProtocolTCP,
					Name:       "http",
				},
			},
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
				result += string(c - 32) // Convert to uppercase
			} else {
				result += string(c)
			}
		}
	}
	return result
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
