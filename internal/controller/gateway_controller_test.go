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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

func checkGatewayDeploymentEnvVars(ctx context.Context, k8sClient client.Client, gateway *corev1alpha1.Gateway, expectedEnvVars []corev1.EnvVar, timeout time.Duration, interval time.Duration) {
	gatewayLookupKey := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}
	createdGateway := &corev1alpha1.Gateway{}

	Eventually(func() bool {
		err := k8sClient.Get(ctx, gatewayLookupKey, createdGateway)
		return err == nil
	}, timeout, interval).Should(BeTrue())

	gatewayReconciler := &GatewayReconciler{
		Client: k8sClient,
		Scheme: k8sClient.Scheme(),
	}

	_, err := gatewayReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: gatewayLookupKey,
	})
	Expect(err).NotTo(HaveOccurred())

	deploymentName := types.NamespacedName{
		Name:      gateway.Name,
		Namespace: gateway.Namespace,
	}
	createdDeployment := &appsv1.Deployment{}
	Eventually(func() bool {
		err := k8sClient.Get(ctx, deploymentName, createdDeployment)
		return err == nil
	}, timeout, interval).Should(BeTrue())

	containers := createdDeployment.Spec.Template.Spec.Containers
	Expect(containers).To(HaveLen(1))
	envVars := containers[0].Env

	for _, expected := range expectedEnvVars {
		Expect(envVars).To(ContainElement(expected))
	}

	Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
}

var _ = Describe("Gateway controller", func() {
	Context("When reconciling a Gateway resource", func() {
		const (
			GatewayName      = "test-gateway"
			GatewayNamespace = "default"
			timeout          = time.Second * 10
			interval         = time.Millisecond * 250
		)

		ctx := context.Background()

		It("Should create a deployment and configmap with proper configuration", func() {
			By("Creating a new Gateway")
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GatewayName,
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "production",
					Replicas:    &[]int32{1}[0],
					Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
					Telemetry: &corev1alpha1.TelemetrySpec{
						Enabled: true,
						Metrics: &corev1alpha1.MetricsSpec{
							Enabled: true,
							Port:    9464,
						},
					},
					Auth: &corev1alpha1.AuthSpec{
						Enabled:  true,
						Provider: "oidc",
						OIDC: &corev1alpha1.OIDCSpec{
							IssuerURL: "https://auth.example.com",
							ClientID:  "test-client",
							ClientSecretRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "oidc-secret",
								},
								Key: "client-secret",
							},
						},
					},
					Server: &corev1alpha1.ServerSpec{
						Host: "0.0.0.0",
						Port: 8080,
						Timeouts: &corev1alpha1.ServerTimeouts{
							Read:  "60s",
							Write: "60s",
							Idle:  "300s",
						},
					},
					Providers: []corev1alpha1.ProviderSpec{
						{
							Name: "openai",
							Env: &[]corev1.EnvVar{
								{
									Name: "OPENAI_API_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "openai-secret",
											},
											Key: "OPENAI_API_KEY",
										},
									},
								},
							},
						},
					},
				},
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "openai-secret",
					Namespace: GatewayNamespace,
				},
				StringData: map[string]string{
					"OPENAI_API_KEY": "super-secret-value",
				},
			}

			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())

			gatewayLookupKey := types.NamespacedName{Name: GatewayName, Namespace: GatewayNamespace}
			createdGateway := &corev1alpha1.Gateway{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayLookupKey, createdGateway)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			deploymentName := types.NamespacedName{
				Name:      GatewayName,
				Namespace: GatewayNamespace,
			}
			createdDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentName, createdDeployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			containers := createdDeployment.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			envVars := containers[0].Env

			Expect(envVars).To(ContainElement(corev1.EnvVar{
				Name:  "ENVIRONMENT",
				Value: "production",
			}))
			Expect(envVars).To(ContainElement(corev1.EnvVar{
				Name:  "ENABLE_TELEMETRY",
				Value: "true",
			}))
			Expect(envVars).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name":      Equal("OIDC_CLIENT_SECRET"),
				"ValueFrom": Not(BeNil()),
			})))
			Expect(envVars).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name":      Equal("OPENAI_API_KEY"),
				"ValueFrom": Not(BeNil()),
			})))

			Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
		})

		It("Should create and manage HPA when enabled", func() {
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GatewayName + "-hpa",
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "production",
					Replicas:    nil,
					Resources: &corev1alpha1.ResourceRequirements{
						Requests: &corev1alpha1.ResourceList{
							CPU:    "100m",
							Memory: "128Mi",
						},
						Limits: &corev1alpha1.ResourceList{
							CPU:    "500m",
							Memory: "512Mi",
						},
					},
					HPA: &corev1alpha1.HPASpec{
						Enabled: true,
						Config: &corev1alpha1.CustomHorizontalPodAutoscalerSpec{
							MinReplicas: &[]int32{2}[0],
							MaxReplicas: 5,
							Metrics: []autoscalingv2.MetricSpec{
								{
									Type: autoscalingv2.ResourceMetricSourceType,
									Resource: &autoscalingv2.ResourceMetricSource{
										Name: corev1.ResourceCPU,
										Target: autoscalingv2.MetricTarget{
											Type:               autoscalingv2.UtilizationMetricType,
											AverageUtilization: &[]int32{70}[0],
										},
									},
								},
								{
									Type: autoscalingv2.ResourceMetricSourceType,
									Resource: &autoscalingv2.ResourceMetricSource{
										Name: corev1.ResourceMemory,
										Target: autoscalingv2.MetricTarget{
											Type:               autoscalingv2.UtilizationMetricType,
											AverageUtilization: &[]int32{80}[0],
										},
									},
								},
							},
							Behavior: &autoscalingv2.HorizontalPodAutoscalerBehavior{
								ScaleDown: &autoscalingv2.HPAScalingRules{
									StabilizationWindowSeconds: &[]int32{300}[0],
								},
								ScaleUp: &autoscalingv2.HPAScalingRules{
									StabilizationWindowSeconds: &[]int32{60}[0],
								},
							},
						},
					},
					Providers: []corev1alpha1.ProviderSpec{},
				},
			}

			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

			gatewayName := types.NamespacedName{Name: GatewayName + "-hpa", Namespace: GatewayNamespace}
			createdGateway := &corev1alpha1.Gateway{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayName, createdGateway)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			deploymentName := types.NamespacedName{Name: GatewayName + "-hpa", Namespace: GatewayNamespace}
			createdDeployment := &appsv1.Deployment{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentName, createdDeployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			hpaName := types.NamespacedName{Name: GatewayName + "-hpa-hpa", Namespace: GatewayNamespace}
			createdHPA := &autoscalingv2.HorizontalPodAutoscaler{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, hpaName, createdHPA)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdHPA.Spec.MinReplicas).To(Equal(&[]int32{2}[0]))
			Expect(createdHPA.Spec.MaxReplicas).To(Equal(int32(5)))
			Expect(createdHPA.Spec.ScaleTargetRef.Name).To(Equal(GatewayName + "-hpa"))
			Expect(createdHPA.Spec.ScaleTargetRef.Kind).To(Equal("Deployment"))

			Expect(createdHPA.Spec.Metrics).To(HaveLen(2))

			var cpuMetric *autoscalingv2.MetricSpec
			var memoryMetric *autoscalingv2.MetricSpec

			for i := range createdHPA.Spec.Metrics {
				if createdHPA.Spec.Metrics[i].Resource != nil {
					switch createdHPA.Spec.Metrics[i].Resource.Name {
					case corev1.ResourceCPU:
						cpuMetric = &createdHPA.Spec.Metrics[i]
					case corev1.ResourceMemory:
						memoryMetric = &createdHPA.Spec.Metrics[i]
					}
				}
			}

			Expect(cpuMetric).ToNot(BeNil())
			Expect(cpuMetric.Resource.Target.AverageUtilization).To(Equal(&[]int32{70}[0]))

			Expect(memoryMetric).ToNot(BeNil())
			Expect(memoryMetric.Resource.Target.AverageUtilization).To(Equal(&[]int32{80}[0]))

			Expect(createdHPA.Spec.Behavior).ToNot(BeNil())
			Expect(createdHPA.Spec.Behavior.ScaleDown).ToNot(BeNil())
			Expect(createdHPA.Spec.Behavior.ScaleDown.StabilizationWindowSeconds).To(Equal(&[]int32{300}[0]))
			Expect(createdHPA.Spec.Behavior.ScaleUp).ToNot(BeNil())
			Expect(createdHPA.Spec.Behavior.ScaleUp.StabilizationWindowSeconds).To(Equal(&[]int32{60}[0]))

			Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
		})

		It("Should delete HPA when disabled", func() {
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GatewayName + "-hpa-disable",
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "production",
					HPA: &corev1alpha1.HPASpec{
						Enabled: true,
						Config: &corev1alpha1.CustomHorizontalPodAutoscalerSpec{
							MinReplicas: &[]int32{1}[0],
							MaxReplicas: 3,
							Metrics: []autoscalingv2.MetricSpec{
								{
									Type: autoscalingv2.ResourceMetricSourceType,
									Resource: &autoscalingv2.ResourceMetricSource{
										Name: corev1.ResourceCPU,
										Target: autoscalingv2.MetricTarget{
											Type:               autoscalingv2.UtilizationMetricType,
											AverageUtilization: &[]int32{80}[0],
										},
									},
								},
							},
						},
					},
					Providers: []corev1alpha1.ProviderSpec{},
				},
			}

			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

			gatewayName := types.NamespacedName{Name: GatewayName + "-hpa-disable", Namespace: GatewayNamespace}
			hpaName := types.NamespacedName{Name: GatewayName + "-hpa-disable-hpa", Namespace: GatewayNamespace}

			createdHPA := &autoscalingv2.HorizontalPodAutoscaler{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, hpaName, createdHPA)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			createdGateway := &corev1alpha1.Gateway{}
			Expect(k8sClient.Get(ctx, gatewayName, createdGateway)).Should(Succeed())
			createdGateway.Spec.HPA.Enabled = false
			Expect(k8sClient.Update(ctx, createdGateway)).Should(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, hpaName, createdHPA)
				return err != nil
			}, timeout, interval).Should(BeTrue())

			deploymentName := types.NamespacedName{Name: GatewayName + "-hpa-disable", Namespace: GatewayNamespace}
			createdDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentName, createdDeployment)
				if err != nil {
					return false
				}
				return createdDeployment.Spec.Replicas != nil
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
		})

		DescribeTable("Ingress reconciliation",
			func(name, host string, tlsEnabled bool) {
				ctx := context.Background()
				gwName := name
				gwNs := GatewayNamespace
				gateway := &corev1alpha1.Gateway{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "core.inference-gateway.com/v1alpha1",
						Kind:       "Gateway",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      gwName,
						Namespace: gwNs,
					},
					Spec: corev1alpha1.GatewaySpec{
						Environment: "development",
						Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
						Ingress: &corev1alpha1.IngressSpec{
							Enabled: true,
							Host:    host,
							TLS: &corev1alpha1.IngressTLSConfig{
								Enabled: tlsEnabled,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

				ingressName := types.NamespacedName{Name: gwName, Namespace: gwNs}
				createdIngress := &networkingv1.Ingress{}
				Eventually(func() bool {
					return k8sClient.Get(ctx, ingressName, createdIngress) == nil
				}, timeout, interval).Should(BeTrue())
				Expect(createdIngress.Spec.Rules).ToNot(BeEmpty())
				Expect(createdIngress.Spec.Rules[0].Host).To(Equal(host))
				if tlsEnabled {
					Expect(createdIngress.Spec.TLS).ToNot(BeEmpty())
				}

				if len(createdIngress.Spec.TLS) > 0 {
					Expect(createdIngress.Spec.TLS[0].Hosts).To(ContainElement(host))
					Expect(createdIngress.Spec.TLS[0].SecretName).To(ContainSubstring(gwName))
				}

				Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
			},
			Entry("should create ingress without TLS", "ingress-no-tls", "test-no-tls.local", false),
			Entry("should create ingress with TLS", "ingress-with-tls", "test-with-tls.local", true),
		)

		DescribeTable("Should create a deployment and configmap with correct telemetry configuration",
			func(gatewayName, environment string, telemetryEnabled bool, expectedEnvVars []corev1.EnvVar) {
				gateway := &corev1alpha1.Gateway{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "core.inference-gateway.com/v1alpha1",
						Kind:       "Gateway",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      gatewayName,
						Namespace: GatewayNamespace,
					},
					Spec: corev1alpha1.GatewaySpec{
						Environment: environment,
						Replicas:    &[]int32{1}[0],
						Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
						Telemetry: &corev1alpha1.TelemetrySpec{
							Enabled: telemetryEnabled,
							Metrics: &corev1alpha1.MetricsSpec{
								Enabled: true,
								Port:    9464,
							},
						},
						Server: &corev1alpha1.ServerSpec{
							Host: "0.0.0.0",
							Port: 8080,
						},
						Providers: []corev1alpha1.ProviderSpec{},
					},
				}
				Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())
				checkGatewayDeploymentEnvVars(ctx, k8sClient, gateway, expectedEnvVars, timeout, interval)
			},
			Entry("OpenTelemetry enabled", GatewayName+"-otel", "production", true, []corev1.EnvVar{
				{Name: "ENVIRONMENT", Value: "production"},
				{Name: "ENABLE_TELEMETRY", Value: "true"},
			}),
			Entry("Telemetry enabled in development", GatewayName+"-no-telemetry", "development", true, []corev1.EnvVar{
				{Name: "ENVIRONMENT", Value: "development"},
				{Name: "ENABLE_TELEMETRY", Value: "true"},
			}),
		)

		It("Should set A2A service discovery environment variables when configured", func() {
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GatewayName + "-a2a-sd",
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "production",
					Replicas:    &[]int32{1}[0],
					Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
					A2A: &corev1alpha1.A2AServersSpec{
						Enabled: true,
						ServiceDiscovery: &corev1alpha1.A2AServiceDiscovery{
							Enabled:         true,
							Namespace:       "test-namespace",
							LabelSelector:   "app=test-agent",
							PollingInterval: "60s",
						},
					},
				},
			}

			expectedEnvVars := []corev1.EnvVar{
				{Name: "ENVIRONMENT", Value: "production"},
				{Name: "A2A_ENABLE", Value: "true"},
				{Name: "A2A_EXPOSE", Value: "false"},
				{Name: "A2A_AGENTS", Value: ""},
				{Name: "A2A_CLIENT_TIMEOUT", Value: "5s"},
				{Name: "A2A_SERVICE_DISCOVERY_ENABLED", Value: "true"},
				{Name: "A2A_SERVICE_DISCOVERY_NAMESPACE", Value: "test-namespace"},
				{Name: "A2A_SERVICE_DISCOVERY_LABEL_SELECTOR", Value: "app=test-agent"},
				{Name: "A2A_SERVICE_DISCOVERY_POLLING_INTERVAL", Value: "60s"},
			}

			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())
			checkGatewayDeploymentEnvVars(ctx, k8sClient, gateway, expectedEnvVars, timeout, interval)
		})

		It("Should set A2A service discovery environment variables with defaults when minimal config", func() {
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GatewayName + "-a2a-sd-defaults",
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "production",
					Replicas:    &[]int32{1}[0],
					Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
					A2A: &corev1alpha1.A2AServersSpec{
						Enabled: true,
						ServiceDiscovery: &corev1alpha1.A2AServiceDiscovery{
							Enabled: true,
						},
					},
				},
			}

			expectedEnvVars := []corev1.EnvVar{
				{Name: "ENVIRONMENT", Value: "production"},
				{Name: "A2A_ENABLE", Value: "true"},
				{Name: "A2A_EXPOSE", Value: "false"},
				{Name: "A2A_AGENTS", Value: ""},
				{Name: "A2A_CLIENT_TIMEOUT", Value: "5s"},
				{Name: "A2A_SERVICE_DISCOVERY_ENABLED", Value: "true"},
				{Name: "A2A_SERVICE_DISCOVERY_NAMESPACE", Value: "default"},
				{Name: "A2A_SERVICE_DISCOVERY_LABEL_SELECTOR", Value: "inference-gateway.com/a2a-agent=true"},
				{Name: "A2A_SERVICE_DISCOVERY_POLLING_INTERVAL", Value: "30s"},
			}

			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())
			checkGatewayDeploymentEnvVars(ctx, k8sClient, gateway, expectedEnvVars, timeout, interval)
		})
	})
})
