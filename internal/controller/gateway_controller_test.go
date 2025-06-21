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
	corev1 "k8s.io/api/core/v1"
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
							Port:    2112,
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

			gatewayReconciler := &GatewayReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := gatewayReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: gatewayLookupKey,
			})
			Expect(err).NotTo(HaveOccurred())

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
	})
})
