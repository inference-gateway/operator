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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

// findEnvVar is a helper that returns the EnvVar with the given name, or nil.
func findEnvVar(envVars []corev1.EnvVar, name string) *corev1.EnvVar {
	for i := range envVars {
		if envVars[i].Name == name {
			return &envVars[i]
		}
	}
	return nil
}

var _ = Describe("Agent Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		agent := &v1alpha1.Agent{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Agent")
			err := k8sClient.Get(ctx, typeNamespacedName, agent)
			if err != nil && client.IgnoreNotFound(err) == nil {
				resource := &v1alpha1.Agent{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: v1alpha1.AgentSpec{
						Image: "test-image:latest",
						Port:  8080,
					},
				}
				err := k8sClient.Create(ctx, resource)
				Expect(err).To(Not(HaveOccurred()))
			}
		})

		AfterEach(func() {
			resource := &v1alpha1.Agent{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).To(Not(HaveOccurred()))

			By("Cleanup the specific resource instance Agent")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AgentReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).To(Not(HaveOccurred()))

			By("Checking if Deployment was created")
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, deployment)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			By("Checking if Service was created")
			service := &corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, service)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())
		})
	})

	// Unit-level tests for buildAgentEnvironmentVars — no cluster required.
	Context("buildAgentEnvironmentVars", func() {
		var reconciler *AgentReconciler

		BeforeEach(func() {
			reconciler = &AgentReconciler{}
		})

		It("splits provider/model and emits A2A_AGENT_CLIENT_PROVIDER + A2A_AGENT_CLIENT_MODEL", func() {
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						LLM: v1alpha1.LLMSpec{
							Model: "deepseek/deepseek-v4-flash",
						},
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			provider := findEnvVar(envVars, "A2A_AGENT_CLIENT_PROVIDER")
			Expect(provider).NotTo(BeNil())
			Expect(provider.Value).To(Equal("deepseek"))

			model := findEnvVar(envVars, "A2A_AGENT_CLIENT_MODEL")
			Expect(model).NotTo(BeNil())
			Expect(model.Value).To(Equal("deepseek-v4-flash"))
		})

		It("emits only A2A_AGENT_CLIENT_MODEL when model has no provider prefix", func() {
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						LLM: v1alpha1.LLMSpec{
							Model: "gpt-4o",
						},
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			provider := findEnvVar(envVars, "A2A_AGENT_CLIENT_PROVIDER")
			Expect(provider).To(BeNil(), "should not emit PROVIDER when there is no slash")

			model := findEnvVar(envVars, "A2A_AGENT_CLIENT_MODEL")
			Expect(model).NotTo(BeNil())
			Expect(model.Value).To(Equal("gpt-4o"))
		})

		It("emits A2A_AGENT_CLIENT_BASE_URL from spec.agent.llm.baseURL", func() {
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						LLM: v1alpha1.LLMSpec{
							BaseURL: "http://inference-gateway.inference-gateway.svc.cluster.local:8080/v1",
						},
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			baseURL := findEnvVar(envVars, "A2A_AGENT_CLIENT_BASE_URL")
			Expect(baseURL).NotTo(BeNil())
			Expect(baseURL.Value).To(Equal("http://inference-gateway.inference-gateway.svc.cluster.local:8080/v1"))
		})

		It("does not emit A2A_AGENT_CLIENT_BASE_URL when baseURL is empty", func() {
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						LLM: v1alpha1.LLMSpec{},
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			baseURL := findEnvVar(envVars, "A2A_AGENT_CLIENT_BASE_URL")
			Expect(baseURL).To(BeNil())
		})

		It("emits A2A_AGENT_CLIENT_API_KEY via valueFrom.secretKeyRef from apiKeySecretRef", func() {
			secretRef := &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "my-secret"},
				Key:                  "DEEPSEEK_API_KEY",
			}
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						LLM: v1alpha1.LLMSpec{
							APIKeySecretRef: secretRef,
						},
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			apiKey := findEnvVar(envVars, "A2A_AGENT_CLIENT_API_KEY")
			Expect(apiKey).NotTo(BeNil())
			Expect(apiKey.ValueFrom).NotTo(BeNil())
			Expect(apiKey.ValueFrom.SecretKeyRef).NotTo(BeNil())
			Expect(apiKey.ValueFrom.SecretKeyRef.Name).To(Equal("my-secret"))
			Expect(apiKey.ValueFrom.SecretKeyRef.Key).To(Equal("DEEPSEEK_API_KEY"))
			Expect(apiKey.Value).To(BeEmpty(), "should not set a plain Value alongside valueFrom")
		})

		It("does not emit A2A_AGENT_CLIENT_API_KEY when apiKeySecretRef is nil", func() {
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						LLM: v1alpha1.LLMSpec{},
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			apiKey := findEnvVar(envVars, "A2A_AGENT_CLIENT_API_KEY")
			Expect(apiKey).To(BeNil())
		})

		It("emits A2A_AGENT_CLIENT_MAX_TOKENS from spec.agent.llm.maxTokens", func() {
			tokens := int32(2048)
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						LLM: v1alpha1.LLMSpec{
							MaxTokens: &tokens,
						},
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			maxTokens := findEnvVar(envVars, "A2A_AGENT_CLIENT_MAX_TOKENS")
			Expect(maxTokens).NotTo(BeNil())
			Expect(maxTokens.Value).To(Equal("2048"))
		})

		It("emits A2A_AGENT_CLIENT_TEMPERATURE from spec.agent.llm.temperature", func() {
			temp := "0.5"
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						LLM: v1alpha1.LLMSpec{
							Temperature: &temp,
						},
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			temperature := findEnvVar(envVars, "A2A_AGENT_CLIENT_TEMPERATURE")
			Expect(temperature).NotTo(BeNil())
			Expect(temperature.Value).To(Equal("0.5"))
		})

		It("emits A2A_AGENT_CLIENT_MAX_CHAT_COMPLETION_ITERATIONS and A2A_AGENT_CLIENT_MAX_RETRIES", func() {
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						MaxChatCompletionIterations: 7,
						MaxRetries:                  4,
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			iterations := findEnvVar(envVars, "A2A_AGENT_CLIENT_MAX_CHAT_COMPLETION_ITERATIONS")
			Expect(iterations).NotTo(BeNil())
			Expect(iterations.Value).To(Equal("7"))

			retries := findEnvVar(envVars, "A2A_AGENT_CLIENT_MAX_RETRIES")
			Expect(retries).NotTo(BeNil())
			Expect(retries.Value).To(Equal("4"))
		})

		It("does not emit legacy AGENT_LLM_* or AGENT_MAX_* env vars", func() {
			tokens := int32(4096)
			temp := "0.7"
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Agent: v1alpha1.AgentConfigSpec{
						MaxChatCompletionIterations: 5,
						MaxRetries:                  3,
						LLM: v1alpha1.LLMSpec{
							Model:       "openai/gpt-4o",
							MaxTokens:   &tokens,
							Temperature: &temp,
						},
					},
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			Expect(findEnvVar(envVars, "AGENT_LLM_MODEL")).To(BeNil())
			Expect(findEnvVar(envVars, "AGENT_LLM_SYSTEM_PROMPT")).To(BeNil())
			Expect(findEnvVar(envVars, "AGENT_LLM_MAX_TOKENS")).To(BeNil())
			Expect(findEnvVar(envVars, "AGENT_LLM_TEMPERATURE")).To(BeNil())
			Expect(findEnvVar(envVars, "AGENT_MAX_CHAT_COMPLETION_ITERATIONS")).To(BeNil())
			Expect(findEnvVar(envVars, "AGENT_MAX_RETRIES")).To(BeNil())
			Expect(findEnvVar(envVars, "AGENT_API_KEY_SECRET_REF")).To(BeNil())
		})

		It("prepends user-supplied env vars so operator vars take precedence on conflict", func() {
			userEnv := []corev1.EnvVar{
				{Name: "MY_CUSTOM_VAR", Value: "custom-value"},
			}
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{
					Env: &userEnv,
				},
			}
			envVars := reconciler.buildAgentEnvironmentVars(agent)

			custom := findEnvVar(envVars, "MY_CUSTOM_VAR")
			Expect(custom).NotTo(BeNil())
			Expect(custom.Value).To(Equal("custom-value"))
		})
	})
})
