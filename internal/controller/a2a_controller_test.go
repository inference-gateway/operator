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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	types "k8s.io/apimachinery/pkg/types"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

var _ = Describe("A2A Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "agents",
		}
		a2a := &corev1alpha1.A2A{}

		BeforeEach(func() {
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "agents",
				},
			}
			_ = k8sClient.Create(ctx, ns)

			By("creating the custom resource for the Kind A2A")
			err := k8sClient.Get(ctx, typeNamespacedName, a2a)
			if err != nil && errors.IsNotFound(err) {
				resource := &corev1alpha1.A2A{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "agents",
					},
					Spec: corev1alpha1.A2ASpec{
						Image:        "test-image",
						Timezone:     "UTC",
						Port:         8080,
						Host:         "localhost",
						ReadTimeout:  "30s",
						WriteTimeout: "30s",
						IdleTimeout:  "30s",
						Logging: corev1alpha1.LoggingSpec{
							Level:  "info",
							Format: "json",
						},
						Telemetry: corev1alpha1.TelemetrySpec{
							Enabled: true,
						},
						Queue: corev1alpha1.QueueSpec{
							Enabled:         true,
							MaxSize:         100,
							CleanupInterval: "5m",
						},
						TLS: corev1alpha1.TLSSpec{
							Enabled:   false,
							SecretRef: "",
						},
						Agent: corev1alpha1.AgentSpec{
							Enabled:                     true,
							TLS:                         corev1alpha1.TLSSpec{Enabled: false},
							MaxConversationHistory:      10,
							MaxChatCompletionIterations: 5,
							MaxRetries:                  3,
							APIKey: corev1alpha1.APIKeySpec{
								SecretRef: "test-api-key",
							},
							LLM: corev1alpha1.LLMSpec{
								Model:       "deepseek/deepseek-chat",
								MaxTokens:   &[]int32{1000}[0],
								Temperature: &[]string{"0.7"}[0],
								CustomHeaders: &[]corev1alpha1.HeaderSpec{
									{
										Name:  "User-Agent",
										Value: "test-a2a",
									},
								},
								SystemPrompt: "You are a helpful assistant.",
							},
						},
						Card: corev1alpha1.CardSpec{
							Name:               "Test Card",
							Description:        "Card description",
							URL:                "http://example.com",
							DocumentationURL:   "http://example.com/docs",
							DefaultInputModes:  []string{"text", "image"},
							DefaultOutputModes: []string{"text", "image"},
							Skills:             []string{"summarize", "translate"},
							Capabilities: corev1alpha1.CapabilitiesSpec{
								Streaming:              true,
								StateTransitionHistory: false,
								PushNotifications:      true,
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &corev1alpha1.A2A{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance A2A")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &A2AReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
