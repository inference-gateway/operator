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
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

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
						Tracing: &corev1alpha1.TracingSpec{
							Enabled:  true,
							Endpoint: "http://jaeger:14268/api/traces",
						},
					},
					Auth: &corev1alpha1.AuthSpec{
						Enabled:  true,
						Provider: "oidc",
						OIDC: &corev1alpha1.OIDCSpec{
							IssuerURL: "https://auth.example.com",
							ClientID:  "test-client",
							ClientSecretRef: &corev1alpha1.SecretKeySelector{
								Name: "oidc-secret",
								Key:  "client-secret",
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
							Type: "openai",
							Config: &corev1alpha1.ProviderConfig{
								BaseURL:  "https://api.openai.com/v1",
								AuthType: "bearer",
								TokenRef: &corev1alpha1.SecretKeySelector{
									Name: "openai-secret",
									Key:  "api-key",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

			gatewayLookupKey := types.NamespacedName{Name: GatewayName, Namespace: GatewayNamespace}
			createdGateway := &corev1alpha1.Gateway{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, gatewayLookupKey, createdGateway)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			configMapName := types.NamespacedName{
				Name:      GatewayName + "-config",
				Namespace: GatewayNamespace,
			}
			createdConfigMap := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, configMapName, createdConfigMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdConfigMap.Data).To(HaveKey("config.yaml"))
			configContent := createdConfigMap.Data["config.yaml"]

			Expect(configContent).To(ContainSubstring("environment: production"))
			Expect(configContent).To(ContainSubstring("telemetry:"))
			Expect(configContent).To(ContainSubstring("enabled: true"))
			Expect(configContent).To(ContainSubstring("auth:"))
			Expect(configContent).To(ContainSubstring("provider: oidc"))
			Expect(configContent).To(ContainSubstring("issuerUrl: https://auth.example.com"))
			Expect(configContent).To(ContainSubstring("baseUrl: https://api.openai.com/v1"))
			Expect(configContent).To(ContainSubstring("name: openai"))
			Expect(configContent).To(ContainSubstring("type: openai"))

			deploymentName := types.NamespacedName{
				Name:      GatewayName,
				Namespace: GatewayNamespace,
			}
			createdDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentName, createdDeployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdDeployment.Spec.Template.Spec.Volumes).To(ContainElement(
				HaveField("Name", GatewayName+"-config-volume"),
			))

			containers := createdDeployment.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			Expect(containers[0].VolumeMounts).To(ContainElement(
				HaveField("Name", GatewayName+"-config-volume"),
			))

			envVars := containers[0].Env
			Expect(envVars).To(ContainElement(
				HaveField("Name", "OIDC_CLIENT_SECRET"),
			))
			Expect(envVars).To(ContainElement(
				HaveField("Name", "OPENAI_API_KEY"),
			))

			Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
		})

		It("Should create a deployment and configmap with OpenTelemetry telemetry configuration", func() {
			By("Creating a Gateway with comprehensive OpenTelemetry configuration")
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GatewayName + "-otel",
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "production",
					Replicas:    &[]int32{2}[0],
					Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
					Telemetry: &corev1alpha1.TelemetrySpec{
						Enabled: true,
						Metrics: &corev1alpha1.MetricsSpec{
							Enabled: true,
							Port:    9464,
						},
						Tracing: &corev1alpha1.TracingSpec{
							Enabled:  true,
							Endpoint: "http://jaeger-collector:14268/api/traces",
						},
					},
					Server: &corev1alpha1.ServerSpec{
						Host: "0.0.0.0",
						Port: 8080,
					},
					Providers: []corev1alpha1.ProviderSpec{
						{
							Name: "openai",
							Type: "openai",
							Config: &corev1alpha1.ProviderConfig{
								BaseURL:  "https://api.openai.com/v1",
								AuthType: "bearer",
								TokenRef: &corev1alpha1.SecretKeySelector{
									Name: "openai-secret",
									Key:  "api-key",
								},
							},
						},
						{
							Name: "anthropic",
							Type: "anthropic",
							Config: &corev1alpha1.ProviderConfig{
								BaseURL:  "https://api.anthropic.com/v1",
								AuthType: "bearer",
								TokenRef: &corev1alpha1.SecretKeySelector{
									Name: "anthropic-secret",
									Key:  "api-key",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

			gatewayLookupKey := types.NamespacedName{Name: GatewayName + "-otel", Namespace: GatewayNamespace}
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

			configMapName := types.NamespacedName{
				Name:      GatewayName + "-otel-config",
				Namespace: GatewayNamespace,
			}
			createdConfigMap := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, configMapName, createdConfigMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdConfigMap.Data).To(HaveKey("config.yaml"))
			configContent := createdConfigMap.Data["config.yaml"]

			Expect(configContent).To(ContainSubstring("environment: production"))

			Expect(configContent).To(ContainSubstring("telemetry:"))
			Expect(configContent).To(ContainSubstring("enabled: true"))
			Expect(configContent).To(ContainSubstring("metrics:"))
			Expect(configContent).To(ContainSubstring("port: 9464"))
			Expect(configContent).To(ContainSubstring("tracing:"))
			Expect(configContent).To(ContainSubstring("endpoint: http://jaeger-collector:14268/api/traces"))

			Expect(configContent).To(ContainSubstring("name: openai"))
			Expect(configContent).To(ContainSubstring("type: openai"))
			Expect(configContent).To(ContainSubstring("baseUrl: https://api.openai.com/v1"))
			Expect(configContent).To(ContainSubstring("name: anthropic"))
			Expect(configContent).To(ContainSubstring("type: anthropic"))
			Expect(configContent).To(ContainSubstring("baseUrl: https://api.anthropic.com/v1"))

			deploymentName := types.NamespacedName{
				Name:      GatewayName + "-otel",
				Namespace: GatewayNamespace,
			}
			createdDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentName, createdDeployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			containers := createdDeployment.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))

			ports := containers[0].Ports
			var hasMetricsPort bool
			for _, port := range ports {
				if port.ContainerPort == 9464 {
					hasMetricsPort = true
					break
				}
			}
			Expect(hasMetricsPort).To(BeTrue(), "Metrics port 9464 should be exposed in container")

			envVars := containers[0].Env
			Expect(envVars).To(ContainElement(
				HaveField("Name", "OPENAI_API_KEY"),
			))
			Expect(envVars).To(ContainElement(
				HaveField("Name", "ANTHROPIC_API_KEY"),
			))

			Expect(containers[0].VolumeMounts).To(ContainElement(
				HaveField("Name", GatewayName+"-otel-config-volume"),
			))

			Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
		})

		It("Should handle telemetry disabled configuration correctly", func() {
			By("Creating a Gateway with telemetry disabled")
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GatewayName + "-no-telemetry",
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "development",
					Replicas:    &[]int32{1}[0],
					Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
					Telemetry: &corev1alpha1.TelemetrySpec{
						Enabled: false,
						Metrics: &corev1alpha1.MetricsSpec{
							Enabled: false,
							Port:    9464,
						},
						Tracing: &corev1alpha1.TracingSpec{
							Enabled:  false,
							Endpoint: "",
						},
					},
					Server: &corev1alpha1.ServerSpec{
						Host: "0.0.0.0",
						Port: 8080,
					},
					Providers: []corev1alpha1.ProviderSpec{
						{
							Name: "openai",
							Type: "openai",
							Config: &corev1alpha1.ProviderConfig{
								BaseURL:  "https://api.openai.com/v1",
								AuthType: "bearer",
								TokenRef: &corev1alpha1.SecretKeySelector{
									Name: "openai-secret",
									Key:  "api-key",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

			gatewayLookupKey := types.NamespacedName{Name: GatewayName + "-no-telemetry", Namespace: GatewayNamespace}
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

			configMapName := types.NamespacedName{
				Name:      GatewayName + "-no-telemetry-config",
				Namespace: GatewayNamespace,
			}
			createdConfigMap := &corev1.ConfigMap{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, configMapName, createdConfigMap)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(createdConfigMap.Data).To(HaveKey("config.yaml"))
			configContent := createdConfigMap.Data["config.yaml"]

			Expect(configContent).To(ContainSubstring("telemetry:"))
			Expect(configContent).To(ContainSubstring("enabled: false"))

			deploymentName := types.NamespacedName{
				Name:      GatewayName + "-no-telemetry",
				Namespace: GatewayNamespace,
			}
			createdDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, deploymentName, createdDeployment)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			containers := createdDeployment.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))

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
						Enabled:                             true,
						MinReplicas:                         &[]int32{2}[0],
						MaxReplicas:                         5,
						TargetCPUUtilizationPercentage:      &[]int32{70}[0],
						TargetMemoryUtilizationPercentage:   &[]int32{80}[0],
						ScaleDownStabilizationWindowSeconds: &[]int32{300}[0],
						ScaleUpStabilizationWindowSeconds:   &[]int32{60}[0],
					},
					Providers: []corev1alpha1.ProviderSpec{
						{
							Name: "openai",
							Type: "openai",
						},
					},
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
						Enabled:                        true,
						MinReplicas:                    &[]int32{1}[0],
						MaxReplicas:                    3,
						TargetCPUUtilizationPercentage: &[]int32{80}[0],
					},
					Providers: []corev1alpha1.ProviderSpec{
						{
							Name: "openai",
							Type: "openai",
						},
					},
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
	})
})
