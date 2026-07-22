/*
Copyright 2026 Inference Gateway

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
	testutil "github.com/inference-gateway/operator/internal/controller/testutil"
)

// newFakeMCPClient builds a controller-runtime fake client with the v1alpha1 scheme
// registered and the given MCP objects pre-loaded - for use in non-envtest unit tests
// that exercise reconciler helpers in isolation.
func newFakeMCPClient(objs ...client.Object) client.Client {
	return testutil.NewFakeClient(objs...)
}

// gatewayTestScheme is a runtime.Scheme prebuilt with the project APIs registered,
// used to satisfy `Scheme` on reconcilers under unit tests.
var gatewayTestScheme = testutil.Scheme()

func checkGatewayDeploymentEnvVars(ctx context.Context, k8sClient client.Client, gateway *corev1alpha1.Gateway, expectedEnvVars, notExpectedEnvVars []corev1.EnvVar, timeout time.Duration, interval time.Duration) {
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

	Eventually(func() error {
		_, err := gatewayReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: gatewayLookupKey,
		})
		return err
	}, timeout, interval).Should(Succeed())

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

	for _, notExpected := range notExpectedEnvVars {
		Expect(envVars).NotTo(ContainElement(notExpected))
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
				Name:  "TELEMETRY_ENABLE",
				Value: "true",
			}))
			Expect(envVars).To(ContainElement(corev1.EnvVar{
				Name:  "AUTH_ENABLE",
				Value: "true",
			}))
			Expect(envVars).To(ContainElement(corev1.EnvVar{
				Name:  "AUTH_OIDC_ISSUER",
				Value: "https://auth.example.com",
			}))
			Expect(envVars).To(ContainElement(corev1.EnvVar{
				Name:  "AUTH_OIDC_CLIENT_ID",
				Value: "test-client",
			}))
			Expect(envVars).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name":      Equal("AUTH_OIDC_CLIENT_SECRET"),
				"ValueFrom": Not(BeNil()),
			})))
			Expect(envVars).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name":      Equal("OPENAI_API_KEY"),
				"ValueFrom": Not(BeNil()),
			})))

			Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
		})

		It("Should mount the OIDC CA certificate and set SSL_CERT_FILE when caCertRef is set", func() {
			By("Creating a Gateway with auth.oidc.caCertRef")
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      GatewayName + "-oidc-ca",
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "production",
					Replicas:    &[]int32{1}[0],
					Auth: &corev1alpha1.AuthSpec{
						Enabled:  true,
						Provider: "oidc",
						OIDC: &corev1alpha1.OIDCSpec{
							IssuerURL: "https://keycloak.example.com/realms/test",
							ClientID:  "test-client",
							ClientSecretRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "oidc-secret"},
								Key:                  "client-secret",
							},
							CACertRef: &corev1.ConfigMapKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{Name: "keycloak-ca"},
								Key:                  "ca.crt",
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

			deploymentName := types.NamespacedName{Name: GatewayName + "-oidc-ca", Namespace: GatewayNamespace}
			createdDeployment := &appsv1.Deployment{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, deploymentName, createdDeployment) == nil
			}, timeout, interval).Should(BeTrue())

			By("Setting SSL_CERT_FILE on the container")
			containers := createdDeployment.Spec.Template.Spec.Containers
			Expect(containers).To(HaveLen(1))
			Expect(containers[0].Env).To(ContainElement(corev1.EnvVar{
				Name:  "SSL_CERT_FILE",
				Value: "/usr/local/share/ca-certificates/oidc-ca.crt",
			}))

			By("Mounting the CA ConfigMap into the pod")
			Expect(containers[0].VolumeMounts).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name":      Equal("oidc-ca"),
				"MountPath": Equal("/usr/local/share/ca-certificates/oidc-ca.crt"),
				"SubPath":   Equal("ca.crt"),
			})))
			Expect(createdDeployment.Spec.Template.Spec.Volumes).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name": Equal("oidc-ca"),
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

		DescribeTable("Routing reconciliation in default mode",
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
						Routing: &corev1alpha1.RoutingSpec{
							Enabled: true,
							Gateway: &corev1alpha1.RoutingGatewaySpec{
								GatewayClassName: "envoy",
								TLS: &corev1alpha1.RoutingTLSSpec{
									Enabled: tlsEnabled,
								},
							},
							HTTPRoute: &corev1alpha1.RoutingHTTPRouteSpec{
								Hostnames: []gwapiv1.Hostname{gwapiv1.Hostname(host)},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

				key := types.NamespacedName{Name: gwName, Namespace: gwNs}

				createdGateway := &gwapiv1.Gateway{}
				Eventually(func() bool {
					return k8sClient.Get(ctx, key, createdGateway) == nil
				}, timeout, interval).Should(BeTrue())
				Expect(createdGateway.Spec.GatewayClassName).To(Equal(gwapiv1.ObjectName("envoy")))
				Expect(createdGateway.Spec.Listeners).ToNot(BeEmpty())
				if tlsEnabled {
					Expect(createdGateway.Spec.Listeners[0].Protocol).To(Equal(gwapiv1.HTTPSProtocolType))
					Expect(createdGateway.Spec.Listeners[0].Port).To(Equal(gwapiv1.PortNumber(443)))
					Expect(createdGateway.Spec.Listeners[0].TLS).ToNot(BeNil())
					Expect(createdGateway.Spec.Listeners[0].TLS.CertificateRefs).ToNot(BeEmpty())
					Expect(string(createdGateway.Spec.Listeners[0].TLS.CertificateRefs[0].Name)).To(ContainSubstring(gwName))
				} else {
					Expect(createdGateway.Spec.Listeners[0].Protocol).To(Equal(gwapiv1.HTTPProtocolType))
					Expect(createdGateway.Spec.Listeners[0].Port).To(Equal(gwapiv1.PortNumber(80)))
				}

				createdRoute := &gwapiv1.HTTPRoute{}
				Eventually(func() bool {
					return k8sClient.Get(ctx, key, createdRoute) == nil
				}, timeout, interval).Should(BeTrue())
				Expect(createdRoute.Spec.Hostnames).To(ContainElement(gwapiv1.Hostname(host)))
				Expect(createdRoute.Spec.ParentRefs).To(HaveLen(1))
				Expect(createdRoute.Spec.ParentRefs[0].Name).To(Equal(gwapiv1.ObjectName(gwName)))
				Expect(createdRoute.Spec.Rules).To(HaveLen(1))
				Expect(createdRoute.Spec.Rules[0].BackendRefs).To(HaveLen(1))
				Expect(createdRoute.Spec.Rules[0].BackendRefs[0].Name).To(Equal(gwapiv1.ObjectName(gwName)))

				Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
			},
			Entry("should create Gateway+HTTPRoute without TLS", "routing-no-tls", "test-no-tls.local", false),
			Entry("should create Gateway+HTTPRoute with TLS", "routing-with-tls", "test-with-tls.local", true),
		)

		It("Should add cert-manager.io/cluster-issuer annotation when TLS Issuer is set", func() {
			ctx := context.Background()
			gwName := "routing-issuer"
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      gwName,
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "development",
					Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
					Routing: &corev1alpha1.RoutingSpec{
						Enabled: true,
						Gateway: &corev1alpha1.RoutingGatewaySpec{
							GatewayClassName: "envoy",
							TLS: &corev1alpha1.RoutingTLSSpec{
								Enabled: true,
								Issuer:  "letsencrypt-prod",
							},
						},
						HTTPRoute: &corev1alpha1.RoutingHTTPRouteSpec{
							Hostnames: []gwapiv1.Hostname{"issuer.example.com"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

			createdGateway := &gwapiv1.Gateway{}
			Eventually(func() string {
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: gwName, Namespace: GatewayNamespace}, createdGateway); err != nil {
					return ""
				}
				return createdGateway.Annotations["cert-manager.io/cluster-issuer"]
			}, timeout, interval).Should(Equal("letsencrypt-prod"))

			Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
		})

		It("Should skip Gateway creation in advanced mode (parentRefs set)", func() {
			ctx := context.Background()
			gwName := "routing-advanced"
			parentName := gwapiv1.ObjectName("shared-gw")
			parentNs := gwapiv1.Namespace("shared")
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      gwName,
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "development",
					Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
					Routing: &corev1alpha1.RoutingSpec{
						Enabled: true,
						Gateway: &corev1alpha1.RoutingGatewaySpec{
							ParentRefs: []gwapiv1.ParentReference{
								{Name: parentName, Namespace: &parentNs},
							},
						},
						HTTPRoute: &corev1alpha1.RoutingHTTPRouteSpec{
							Hostnames: []gwapiv1.Hostname{"advanced.example.com"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

			key := types.NamespacedName{Name: gwName, Namespace: GatewayNamespace}

			createdRoute := &gwapiv1.HTTPRoute{}
			Eventually(func() bool {
				return k8sClient.Get(ctx, key, createdRoute) == nil
			}, timeout, interval).Should(BeTrue())
			Expect(createdRoute.Spec.ParentRefs).To(HaveLen(1))
			Expect(createdRoute.Spec.ParentRefs[0].Name).To(Equal(parentName))
			Expect(createdRoute.Spec.ParentRefs[0].Namespace).ToNot(BeNil())
			Expect(*createdRoute.Spec.ParentRefs[0].Namespace).To(Equal(parentNs))

			Consistently(func() bool {
				err := k8sClient.Get(ctx, key, &gwapiv1.Gateway{})
				return err != nil
			}, time.Second*2, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
		})

		It("Should delete owned Gateway and HTTPRoute when routing is disabled", func() {
			ctx := context.Background()
			gwName := "routing-disable"
			gateway := &corev1alpha1.Gateway{
				TypeMeta: metav1.TypeMeta{
					APIVersion: "core.inference-gateway.com/v1alpha1",
					Kind:       "Gateway",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      gwName,
					Namespace: GatewayNamespace,
				},
				Spec: corev1alpha1.GatewaySpec{
					Environment: "development",
					Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
					Routing: &corev1alpha1.RoutingSpec{
						Enabled: true,
						HTTPRoute: &corev1alpha1.RoutingHTTPRouteSpec{
							Hostnames: []gwapiv1.Hostname{"disable.example.com"},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())

			key := types.NamespacedName{Name: gwName, Namespace: GatewayNamespace}
			Eventually(func() bool {
				return k8sClient.Get(ctx, key, &gwapiv1.HTTPRoute{}) == nil
			}, timeout, interval).Should(BeTrue())

			updated := &corev1alpha1.Gateway{}
			Expect(k8sClient.Get(ctx, key, updated)).To(Succeed())
			updated.Spec.Routing.Enabled = false
			Expect(k8sClient.Update(ctx, updated)).To(Succeed())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, &gwapiv1.HTTPRoute{})
				return err != nil
			}, timeout, interval).Should(BeTrue())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, key, &gwapiv1.Gateway{})
				return err != nil
			}, timeout, interval).Should(BeTrue())

			Expect(k8sClient.Delete(ctx, gateway)).Should(Succeed())
		})

		DescribeTable("Should create a deployment and configmap with correct telemetry configuration",
			func(gatewayName, environment string, telemetrySpec *corev1alpha1.TelemetrySpec, expectedEnvVars, notExpectedEnvVars []corev1.EnvVar) {
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
						Telemetry:   telemetrySpec,
						Server: &corev1alpha1.ServerSpec{
							Host: "0.0.0.0",
							Port: 8080,
						},
						Providers: []corev1alpha1.ProviderSpec{},
					},
				}
				Expect(k8sClient.Create(ctx, gateway)).Should(Succeed())
				checkGatewayDeploymentEnvVars(ctx, k8sClient, gateway, expectedEnvVars, notExpectedEnvVars, timeout, interval)
			},
			Entry("OpenTelemetry enabled", GatewayName+"-otel", "production", &corev1alpha1.TelemetrySpec{
				Enabled: true,
				Metrics: &corev1alpha1.MetricsSpec{
					Enabled: true,
					Port:    9464,
				},
			}, []corev1.EnvVar{
				{Name: "ENVIRONMENT", Value: "production"},
				{Name: "TELEMETRY_ENABLE", Value: "true"},
			}, nil),
			Entry("Telemetry enabled in development", GatewayName+"-no-telemetry", "development", &corev1alpha1.TelemetrySpec{
				Enabled: true,
				Metrics: &corev1alpha1.MetricsSpec{
					Enabled: true,
					Port:    9464,
				},
			}, []corev1.EnvVar{
				{Name: "ENVIRONMENT", Value: "development"},
				{Name: "TELEMETRY_ENABLE", Value: "true"},
			}, nil),
			Entry("Traces OTLP exporter", GatewayName+"-traces", "production", &corev1alpha1.TelemetrySpec{
				Enabled: true,
				Traces: &corev1alpha1.TracesSpec{
					Exporter: &corev1alpha1.TracesExporterSpec{
						OTLP: &corev1alpha1.OTLPExporterSpec{
							Endpoint: "http://otel-collector:4318",
							Protocol: "http/protobuf",
						},
					},
				},
			}, []corev1.EnvVar{
				{Name: "ENVIRONMENT", Value: "production"},
				{Name: "TELEMETRY_ENABLE", Value: "true"},
				{Name: "TELEMETRY_TRACING_ENABLE", Value: "true"},
				{Name: "TELEMETRY_TRACING_OTLP_ENDPOINT", Value: "http://otel-collector:4318"},
			}, nil),
			Entry("Telemetry disabled with traces", GatewayName+"-traces-disabled", "production", &corev1alpha1.TelemetrySpec{
				Enabled: false,
				Traces: &corev1alpha1.TracesSpec{
					Exporter: &corev1alpha1.TracesExporterSpec{
						OTLP: &corev1alpha1.OTLPExporterSpec{
							Endpoint: "http://otel-collector:4318",
							Protocol: "http/protobuf",
						},
					},
				},
			}, []corev1.EnvVar{
				{Name: "ENVIRONMENT", Value: "production"},
				{Name: "TELEMETRY_ENABLE", Value: "false"},
			}, []corev1.EnvVar{
				{Name: "TELEMETRY_TRACING_ENABLE", Value: "true"},
			}),
		)

	})
})

var _ = Describe("Gateway MCP service discovery", func() {
	ctx := context.Background()

	makeGateway := func(static []corev1alpha1.MCPServer, sd *corev1alpha1.MCPServiceDiscoverySpec) *corev1alpha1.Gateway {
		return &corev1alpha1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "default"},
			Spec: corev1alpha1.GatewaySpec{
				MCP: &corev1alpha1.MCPServersSpec{
					Enabled:          true,
					Servers:          static,
					ServiceDiscovery: sd,
				},
			},
		}
	}

	withFakeClient := func(mcps ...*corev1alpha1.MCP) *GatewayReconciler {
		objs := make([]client.Object, 0, len(mcps))
		for _, m := range mcps {
			objs = append(objs, m)
		}
		c := newFakeMCPClient(objs...)
		return &GatewayReconciler{Client: c, Scheme: gatewayTestScheme}
	}

	It("returns only static URLs when service discovery is disabled", func() {
		r := withFakeClient()
		gw := makeGateway(
			[]corev1alpha1.MCPServer{
				{Name: "static-a", URL: "http://static-a:8080"},
				{Name: "static-b", URL: "http://static-b:8080"},
			},
			nil,
		)
		urls := r.assembleMCPServerURLs(ctx, gw)
		Expect(urls).To(Equal([]string{"http://static-a:8080", "http://static-b:8080"}))
	})

	It("returns the union of static and discovered URLs sorted, deduped on URL", func() {
		r := withFakeClient(
			&corev1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mcp-z",
					Namespace: "default",
					Labels:    map[string]string{"discoverable": "true"},
				},
				Spec:   corev1alpha1.MCPSpec{Server: &corev1alpha1.MCPServerSpec{Port: 3000}},
				Status: corev1alpha1.MCPStatus{URL: "http://mcp-z-service.default.svc.cluster.local:3000"},
			},
			&corev1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mcp-a",
					Namespace: "default",
					Labels:    map[string]string{"discoverable": "true"},
				},
				Spec:   corev1alpha1.MCPSpec{Server: &corev1alpha1.MCPServerSpec{Port: 3000}},
				Status: corev1alpha1.MCPStatus{URL: "http://static-a:8080"}, // collides with static below
			},
		)
		gw := makeGateway(
			[]corev1alpha1.MCPServer{{Name: "static-a", URL: "http://static-a:8080"}},
			&corev1alpha1.MCPServiceDiscoverySpec{
				Enabled:  true,
				Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"discoverable": "true"}},
			},
		)
		urls := r.assembleMCPServerURLs(ctx, gw)
		Expect(urls).To(Equal([]string{
			"http://mcp-z-service.default.svc.cluster.local:3000",
			"http://static-a:8080",
		}))
	})

	It("filters discovered MCPs by selector and respects an empty selector (matches all)", func() {
		r := withFakeClient(
			&corev1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{Name: "m1", Namespace: "mcp", Labels: map[string]string{"a": "1"}},
				Spec:       corev1alpha1.MCPSpec{Server: &corev1alpha1.MCPServerSpec{Port: 3000}},
			},
			&corev1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{Name: "m2", Namespace: "mcp", Labels: map[string]string{"a": "2"}},
				Spec:       corev1alpha1.MCPSpec{Server: &corev1alpha1.MCPServerSpec{Port: 3000}},
			},
		)
		gw := makeGateway(nil, &corev1alpha1.MCPServiceDiscoverySpec{
			Enabled:   true,
			Namespace: "mcp",
		})
		urls := r.assembleMCPServerURLs(ctx, gw)
		Expect(urls).To(ConsistOf(
			"http://m1-service.mcp.svc.cluster.local:3000/mcp",
			"http://m2-service.mcp.svc.cluster.local:3000/mcp",
		))
	})

	It("honors a custom spec.server.path on the MCP CR", func() {
		r := withFakeClient(
			&corev1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{Name: "sse-srv", Namespace: "mcp"},
				Spec: corev1alpha1.MCPSpec{Server: &corev1alpha1.MCPServerSpec{
					Port: 3001,
					Path: "/sse",
				}},
			},
		)
		gw := makeGateway(nil, &corev1alpha1.MCPServiceDiscoverySpec{Enabled: true, Namespace: "mcp"})
		urls := r.assembleMCPServerURLs(ctx, gw)
		Expect(urls).To(Equal([]string{"http://sse-srv-service.mcp.svc.cluster.local:3001/sse"}))
	})

	It("uses https when the MCP TLS is enabled and a custom port", func() {
		r := withFakeClient(
			&corev1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{Name: "secure", Namespace: "mcp"},
				Spec: corev1alpha1.MCPSpec{Server: &corev1alpha1.MCPServerSpec{
					Port: 9443,
					TLS:  &corev1alpha1.MCPTLSConfig{Enabled: true, SecretName: "tls"},
				}},
			},
		)
		gw := makeGateway(nil, &corev1alpha1.MCPServiceDiscoverySpec{Enabled: true, Namespace: "mcp"})
		urls := r.assembleMCPServerURLs(ctx, gw)
		Expect(urls).To(Equal([]string{"https://secure-service.mcp.svc.cluster.local:9443/mcp"}))
	})
})

var _ = Describe("Gateway model routing", func() {
	ctx := context.Background()

	newReconciler := func(objs ...client.Object) *GatewayReconciler {
		return &GatewayReconciler{Client: testutil.NewFakeClient(objs...), Scheme: gatewayTestScheme}
	}

	makeGateway := func(mr *corev1alpha1.ModelRoutingSpec) *corev1alpha1.Gateway {
		return &corev1alpha1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: "gw", Namespace: "default"},
			Spec:       corev1alpha1.GatewaySpec{ModelRouting: mr},
		}
	}

	hasEnv := func(env []corev1.EnvVar, name string) bool {
		for _, e := range env {
			if e.Name == name {
				return true
			}
		}
		return false
	}

	hasVolume := func(vols []corev1.Volume, name string) *corev1.Volume {
		for i := range vols {
			if vols[i].Name == name {
				return &vols[i]
			}
		}
		return nil
	}

	It("emits no routing env vars or volume when model routing is unset", func() {
		r := newReconciler()
		dep := r.buildDeployment(ctx, makeGateway(nil))
		env := dep.Spec.Template.Spec.Containers[0].Env
		Expect(hasEnv(env, "ROUTING_ENABLED")).To(BeFalse())
		Expect(hasEnv(env, "ROUTING_CONFIG_PATH")).To(BeFalse())
		Expect(hasVolume(dep.Spec.Template.Spec.Volumes, "model-routing")).To(BeNil())
	})

	It("wires env, volume and an owned ConfigMap for inline config", func() {
		gw := makeGateway(&corev1alpha1.ModelRoutingSpec{
			Enabled: true,
			Config:  "models:\n  fast-chat:\n    deployments:\n      - provider: groq\n        model: a\n      - provider: openai\n        model: b\n",
		})
		r := newReconciler()

		dep := r.buildDeployment(ctx, gw)
		env := dep.Spec.Template.Spec.Containers[0].Env
		Expect(env).To(ContainElement(corev1.EnvVar{Name: "ROUTING_ENABLED", Value: "true"}))
		Expect(env).To(ContainElement(corev1.EnvVar{Name: "ROUTING_CONFIG_PATH", Value: "/etc/inference-gateway/routing/routing.yaml"}))

		vol := hasVolume(dep.Spec.Template.Spec.Volumes, "model-routing")
		Expect(vol).NotTo(BeNil())
		Expect(vol.ConfigMap.Name).To(Equal("gw-routing"))

		Expect(r.reconcileModelRoutingConfig(ctx, gw)).To(Succeed())
		cm := &corev1.ConfigMap{}
		Expect(r.Get(ctx, types.NamespacedName{Name: "gw-routing", Namespace: "default"}, cm)).To(Succeed())
		Expect(cm.Data["routing.yaml"]).To(Equal(gw.Spec.ModelRouting.Config))
	})

	It("uses the referenced ConfigMap key and does not create an owned one", func() {
		gw := makeGateway(&corev1alpha1.ModelRoutingSpec{
			Enabled: true,
			ConfigMapRef: &corev1.ConfigMapKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "my-routing"},
				Key:                  "custom.yaml",
			},
		})
		r := newReconciler()

		dep := r.buildDeployment(ctx, gw)
		env := dep.Spec.Template.Spec.Containers[0].Env
		Expect(env).To(ContainElement(corev1.EnvVar{Name: "ROUTING_CONFIG_PATH", Value: "/etc/inference-gateway/routing/custom.yaml"}))
		vol := hasVolume(dep.Spec.Template.Spec.Volumes, "model-routing")
		Expect(vol).NotTo(BeNil())
		Expect(vol.ConfigMap.Name).To(Equal("my-routing"))

		Expect(r.reconcileModelRoutingConfig(ctx, gw)).To(Succeed())
		cm := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: "gw-routing", Namespace: "default"}, cm)
		Expect(err).To(HaveOccurred())
	})

	It("deletes a previously-owned ConfigMap when routing is disabled", func() {
		existing := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "gw-routing", Namespace: "default"},
			Data:       map[string]string{"routing.yaml": "models: {}"},
		}
		r := newReconciler(existing)

		Expect(r.reconcileModelRoutingConfig(ctx, makeGateway(nil))).To(Succeed())
		cm := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{Name: "gw-routing", Namespace: "default"}, cm)
		Expect(err).To(HaveOccurred())
	})
})
