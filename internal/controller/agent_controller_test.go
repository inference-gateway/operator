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
	"net/http"
	"net/http/httptest"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
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

	Context("agentTelemetryEnvVars", func() {
		It("emits A2A_TELEMETRY_ENABLE=false and no OTEL vars when telemetry is disabled", func() {
			envVars := agentTelemetryEnvVars(v1alpha1.TelemetrySpec{Enabled: false})

			enable := findEnvVar(envVars, "A2A_TELEMETRY_ENABLE")
			Expect(enable).NotTo(BeNil())
			Expect(enable.Value).To(Equal("false"))

			Expect(findEnvVar(envVars, "A2A_OTEL_TRACES_EXPORTER")).To(BeNil())
			Expect(findEnvVar(envVars, "A2A_OTEL_METRICS_EXPORTER")).To(BeNil())
		})

		It("emits A2A_TELEMETRY_ENABLE (not the legacy TELEMETRY_ENABLED) from buildAgentEnvironmentVars", func() {
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{Telemetry: v1alpha1.TelemetrySpec{Enabled: true}},
			}
			envVars := (&AgentReconciler{}).buildAgentEnvironmentVars(agent)

			Expect(findEnvVar(envVars, "TELEMETRY_ENABLED")).To(BeNil(), "the Go ADK reads A2A_TELEMETRY_ENABLE, not TELEMETRY_ENABLED")
			Expect(findEnvVar(envVars, "A2A_TELEMETRY_ENABLE")).NotTo(BeNil())
		})

		It("maps traces OTLP and metrics Prometheus to A2A_OTEL_* vars", func() {
			tel := v1alpha1.TelemetrySpec{
				Enabled: true,
				Traces: &v1alpha1.TracesSpec{Exporter: &v1alpha1.TracesExporterSpec{
					OTLP: &v1alpha1.OTLPExporterSpec{Endpoint: "http://localhost:4318", Protocol: "http/protobuf"},
				}},
				Metrics: &v1alpha1.MetricsSpec{Exporter: &v1alpha1.MetricsExporterSpec{
					Prometheus: &v1alpha1.PrometheusExporterSpec{Port: 9464},
				}},
			}
			envVars := agentTelemetryEnvVars(tel)

			Expect(findEnvVar(envVars, "A2A_TELEMETRY_ENABLE").Value).To(Equal("true"))
			Expect(findEnvVar(envVars, "A2A_OTEL_TRACES_EXPORTER").Value).To(Equal("otlp"))
			Expect(findEnvVar(envVars, "A2A_OTEL_METRICS_EXPORTER").Value).To(Equal("prometheus"))
			Expect(findEnvVar(envVars, "A2A_OTEL_EXPORTER_OTLP_ENDPOINT").Value).To(Equal("http://localhost:4318"))
			Expect(findEnvVar(envVars, "A2A_OTEL_EXPORTER_OTLP_PROTOCOL").Value).To(Equal("http/protobuf"))
			Expect(findEnvVar(envVars, "A2A_OTEL_EXPORTER_PROMETHEUS_PORT").Value).To(Equal("9464"))
		})

		It("shares a single OTLP endpoint (traces preferred) when both signals use OTLP", func() {
			tel := v1alpha1.TelemetrySpec{
				Enabled: true,
				Traces: &v1alpha1.TracesSpec{Exporter: &v1alpha1.TracesExporterSpec{
					OTLP: &v1alpha1.OTLPExporterSpec{Endpoint: "http://traces:4318", Protocol: "grpc"},
				}},
				Metrics: &v1alpha1.MetricsSpec{Exporter: &v1alpha1.MetricsExporterSpec{
					OTLP: &v1alpha1.OTLPExporterSpec{Endpoint: "http://metrics:4318", Protocol: "grpc"},
				}},
			}
			envVars := agentTelemetryEnvVars(tel)

			Expect(findEnvVar(envVars, "A2A_OTEL_TRACES_EXPORTER").Value).To(Equal("otlp"))
			Expect(findEnvVar(envVars, "A2A_OTEL_METRICS_EXPORTER").Value).To(Equal("otlp"))
			Expect(findEnvVar(envVars, "A2A_OTEL_EXPORTER_OTLP_ENDPOINT").Value).To(Equal("http://traces:4318"))
		})

		It("falls back to the metrics OTLP endpoint when only metrics uses OTLP", func() {
			tel := v1alpha1.TelemetrySpec{
				Enabled: true,
				Metrics: &v1alpha1.MetricsSpec{Exporter: &v1alpha1.MetricsExporterSpec{
					OTLP: &v1alpha1.OTLPExporterSpec{Endpoint: "http://metrics:4318"},
				}},
			}
			envVars := agentTelemetryEnvVars(tel)

			Expect(findEnvVar(envVars, "A2A_OTEL_TRACES_EXPORTER").Value).To(Equal("none"))
			Expect(findEnvVar(envVars, "A2A_OTEL_METRICS_EXPORTER").Value).To(Equal("otlp"))
			Expect(findEnvVar(envVars, "A2A_OTEL_EXPORTER_OTLP_ENDPOINT").Value).To(Equal("http://metrics:4318"))
			Expect(findEnvVar(envVars, "A2A_OTEL_EXPORTER_OTLP_PROTOCOL")).To(BeNil(), "omitted protocol lets the SDK default apply")
		})

		It("defaults both exporters to none when enabled without exporter blocks", func() {
			envVars := agentTelemetryEnvVars(v1alpha1.TelemetrySpec{Enabled: true})

			Expect(findEnvVar(envVars, "A2A_OTEL_TRACES_EXPORTER").Value).To(Equal("none"))
			Expect(findEnvVar(envVars, "A2A_OTEL_METRICS_EXPORTER").Value).To(Equal("none"))
			Expect(findEnvVar(envVars, "A2A_OTEL_EXPORTER_OTLP_ENDPOINT")).To(BeNil())
		})
	})

	Context("agentMCPEnvVars", func() {
		It("emits A2A_MCP_ENABLE=false and no other MCP vars when disabled", func() {
			envVars := agentMCPEnvVars(v1alpha1.MCPClientSpec{Enable: false})

			enable := findEnvVar(envVars, "A2A_MCP_ENABLE")
			Expect(enable).NotTo(BeNil())
			Expect(enable.Value).To(Equal("false"))

			Expect(findEnvVar(envVars, "A2A_MCP_SERVERS")).To(BeNil())
			Expect(findEnvVar(envVars, "A2A_MCP_ENDPOINT")).To(BeNil())
			Expect(findEnvVar(envVars, "A2A_MCP_MAX_RETRIES")).To(BeNil())
		})

		It("emits the A2A_MCP_* knobs when enabled", func() {
			mcp := v1alpha1.MCPClientSpec{
				Enable:           true,
				Servers:          []string{"http://mcp-a:8080", "http://mcp-b:8080"},
				Endpoint:         "/mcp",
				RefreshInterval:  "5m",
				DialTimeout:      "30s",
				CallTimeout:      "30s",
				MaxRetries:       0,
				RetryInterval:    "2s",
				RetryMaxInterval: "30s",
			}
			envVars := agentMCPEnvVars(mcp)

			Expect(findEnvVar(envVars, "A2A_MCP_ENABLE").Value).To(Equal("true"))
			Expect(findEnvVar(envVars, "A2A_MCP_SERVERS").Value).To(Equal("http://mcp-a:8080,http://mcp-b:8080"))
			Expect(findEnvVar(envVars, "A2A_MCP_ENDPOINT").Value).To(Equal("/mcp"))
			Expect(findEnvVar(envVars, "A2A_MCP_REFRESH_INTERVAL").Value).To(Equal("5m"))
			Expect(findEnvVar(envVars, "A2A_MCP_DIAL_TIMEOUT").Value).To(Equal("30s"))
			Expect(findEnvVar(envVars, "A2A_MCP_CALL_TIMEOUT").Value).To(Equal("30s"))
			Expect(findEnvVar(envVars, "A2A_MCP_MAX_RETRIES").Value).To(Equal("0"))
			Expect(findEnvVar(envVars, "A2A_MCP_RETRY_INTERVAL").Value).To(Equal("2s"))
			Expect(findEnvVar(envVars, "A2A_MCP_RETRY_MAX_INTERVAL").Value).To(Equal("30s"))
		})

		It("omits A2A_MCP_SERVERS when no servers are configured", func() {
			envVars := agentMCPEnvVars(v1alpha1.MCPClientSpec{Enable: true, Endpoint: "/mcp"})

			Expect(findEnvVar(envVars, "A2A_MCP_SERVERS")).To(BeNil())
			Expect(findEnvVar(envVars, "A2A_MCP_ENDPOINT").Value).To(Equal("/mcp"))
		})

		It("always emits A2A_MCP_ENABLE from buildAgentEnvironmentVars", func() {
			agent := &v1alpha1.Agent{
				Spec: v1alpha1.AgentSpec{MCP: v1alpha1.MCPClientSpec{Enable: true, Servers: []string{"http://mcp:8080"}}},
			}
			envVars := (&AgentReconciler{}).buildAgentEnvironmentVars(agent)

			Expect(findEnvVar(envVars, "A2A_MCP_ENABLE").Value).To(Equal("true"))
			Expect(findEnvVar(envVars, "A2A_MCP_SERVERS").Value).To(Equal("http://mcp:8080"))
		})
	})

	Context("buildAgentDeployment resources", func() {
		var reconciler *AgentReconciler

		BeforeEach(func() {
			reconciler = &AgentReconciler{}
		})

		It("does not set container resources when spec.resources is unset", func() {
			agent := &v1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-no-resources",
					Namespace: "default",
				},
				Spec: v1alpha1.AgentSpec{
					Image: "test-image:latest",
					Port:  8080,
				},
			}

			deployment := reconciler.buildAgentDeployment(agent)
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]
			Expect(container.Resources.Requests).To(BeNil())
			Expect(container.Resources.Limits).To(BeNil())
		})

		It("propagates spec.resources requests and limits to the agent container", func() {
			agent := &v1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-with-resources",
					Namespace: "default",
				},
				Spec: v1alpha1.AgentSpec{
					Image: "test-image:latest",
					Port:  8080,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("512Mi"),
						},
					},
				},
			}

			deployment := reconciler.buildAgentDeployment(agent)
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]

			Expect(container.Resources.Requests).NotTo(BeNil())
			cpuReq := container.Resources.Requests[corev1.ResourceCPU]
			Expect(cpuReq.String()).To(Equal("100m"))
			memReq := container.Resources.Requests[corev1.ResourceMemory]
			Expect(memReq.String()).To(Equal("128Mi"))

			Expect(container.Resources.Limits).NotTo(BeNil())
			cpuLim := container.Resources.Limits[corev1.ResourceCPU]
			Expect(cpuLim.String()).To(Equal("500m"))
			memLim := container.Resources.Limits[corev1.ResourceMemory]
			Expect(memLim.String()).To(Equal("512Mi"))
		})

		It("uses defaultAgentPort when spec.port is unset", func() {
			agent := &v1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-default-port",
					Namespace: "default",
				},
				Spec: v1alpha1.AgentSpec{
					Image: "test-image:latest",
				},
			}

			svc := buildAgentService(agent)
			Expect(svc.Spec.Ports).To(HaveLen(1))
			Expect(svc.Spec.Ports[0].Port).To(Equal(defaultAgentPort))
			Expect(svc.Spec.Ports[0].TargetPort.IntVal).To(Equal(defaultAgentPort))
		})

		It("uses agent.spec.port for the service when set", func() {
			agent := &v1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-custom-port",
					Namespace: "default",
				},
				Spec: v1alpha1.AgentSpec{
					Image: "test-image:latest",
					Port:  9090,
				},
			}

			svc := buildAgentService(agent)
			Expect(svc.Spec.Ports).To(HaveLen(1))
			Expect(svc.Spec.Ports[0].Port).To(Equal(int32(9090)))
			Expect(svc.Spec.Ports[0].TargetPort.IntVal).To(Equal(int32(9090)))
		})

		It("propagates only requests when limits are unset", func() {
			agent := &v1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "agent-requests-only",
					Namespace: "default",
				},
				Spec: v1alpha1.AgentSpec{
					Image: "test-image:latest",
					Port:  8080,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("250m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			}

			deployment := reconciler.buildAgentDeployment(agent)
			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := deployment.Spec.Template.Spec.Containers[0]

			Expect(container.Resources.Requests).NotTo(BeNil())
			cpuReq := container.Resources.Requests[corev1.ResourceCPU]
			Expect(cpuReq.String()).To(Equal("250m"))
			memReq := container.Resources.Requests[corev1.ResourceMemory]
			Expect(memReq.String()).To(Equal("256Mi"))

			Expect(container.Resources.Limits).To(BeNil())
		})
	})

	Context("agentCardPort", func() {
		It("returns agent.spec.port when set", func() {
			agent := &v1alpha1.Agent{Spec: v1alpha1.AgentSpec{Port: 9091}}
			svc := &corev1.Service{Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 1234}},
			}}
			Expect(agentCardPort(agent, svc)).To(Equal(int32(9091)))
		})

		It("falls back to the service port when agent.spec.port is unset", func() {
			agent := &v1alpha1.Agent{}
			svc := &corev1.Service{Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{{Port: 1234}},
			}}
			Expect(agentCardPort(agent, svc)).To(Equal(int32(1234)))
		})

		It("falls back to defaultAgentPort when neither is set", func() {
			Expect(agentCardPort(&v1alpha1.Agent{}, &corev1.Service{})).To(Equal(defaultAgentPort))
		})
	})

	Context("agentCardURLs", func() {
		It("returns nil when service is nil", func() {
			Expect(agentCardURLs(&v1alpha1.Agent{}, nil)).To(BeNil())
		})

		It("returns the agent-card.json path before the legacy agent.json path", func() {
			agent := &v1alpha1.Agent{Spec: v1alpha1.AgentSpec{Port: 8080}}
			svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Name:      "my-agent",
				Namespace: "agents",
			}}

			urls := agentCardURLs(agent, svc)
			Expect(urls).To(HaveLen(2))
			Expect(urls[0]).To(Equal("http://my-agent.agents.svc.cluster.local:8080/.well-known/agent-card.json"))
			Expect(urls[1]).To(Equal("http://my-agent.agents.svc.cluster.local:8080/.well-known/agent.json"))
		})

		It("honors a non-default agent.spec.port in the URL", func() {
			agent := &v1alpha1.Agent{Spec: v1alpha1.AgentSpec{Port: 9090}}
			svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{
				Name:      "alpha",
				Namespace: "beta",
			}}

			urls := agentCardURLs(agent, svc)
			Expect(urls).To(HaveLen(2))
			Expect(urls[0]).To(Equal("http://alpha.beta.svc.cluster.local:9090/.well-known/agent-card.json"))
			Expect(urls[1]).To(Equal("http://alpha.beta.svc.cluster.local:9090/.well-known/agent.json"))
		})
	})

	Context("agentAdvertisedURL", func() {
		It("returns the in-cluster Service URL when spec.card.url is unset", func() {
			agent := &v1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "agents"},
				Spec:       v1alpha1.AgentSpec{Port: 8080},
			}
			Expect(agentAdvertisedURL(agent)).To(Equal("http://my-agent.agents.svc.cluster.local:8080"))
		})

		It("uses defaultAgentPort when port is unset", func() {
			agent := &v1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "ns"},
			}
			Expect(agentAdvertisedURL(agent)).To(Equal("http://a.ns.svc.cluster.local:8080"))
		})

		It("returns spec.card.url when set, ignoring derived URL", func() {
			agent := &v1alpha1.Agent{
				ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "agents"},
				Spec: v1alpha1.AgentSpec{
					Port: 8080,
					Card: v1alpha1.CardSpec{URL: "https://agent.example.com"},
				},
			}
			Expect(agentAdvertisedURL(agent)).To(Equal("https://agent.example.com"))
		})
	})

	Context("getAgentCard", func() {
		It("decodes a valid agent card JSON response", func() {
			body := `{
				"name": "test-agent",
				"version": "1.2.3",
				"description": "desc",
				"url": "http://test-agent.agents.svc.cluster.local:8080",
				"defaultInputModes": ["text"],
				"defaultOutputModes": ["text"],
				"capabilities": {
					"streaming": true,
					"pushNotifications": false,
					"stateTransitionHistory": true
				},
				"skills": []
			}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer server.Close()

			card, err := getAgentCard(&http.Client{Timeout: 2 * time.Second}, server.URL)
			Expect(err).NotTo(HaveOccurred())
			Expect(card).NotTo(BeNil())
			Expect(card.Version).To(Equal("1.2.3"))
			Expect(card.URL).To(Equal("http://test-agent.agents.svc.cluster.local:8080"))
			Expect(card.Capabilities.Streaming).To(BeTrue())
			Expect(card.Capabilities.PushNotifications).To(BeFalse())
			Expect(card.Capabilities.StateTransitionHistory).To(BeTrue())
		})

		It("returns an error on non-2xx responses", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNotFound)
			}))
			defer server.Close()

			_, err := getAgentCard(&http.Client{Timeout: 2 * time.Second}, server.URL)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unexpected status"))
		})

		It("returns an error on malformed JSON", func() {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte(`{not-json`))
			}))
			defer server.Close()

			_, err := getAgentCard(&http.Client{Timeout: 2 * time.Second}, server.URL)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("setReadyCondition", func() {
		It("appends a Ready condition when none exists", func() {
			var conditions []metav1.Condition
			setReadyCondition(&conditions, metav1.ConditionTrue, "Ok", "all good")

			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0].Type).To(Equal("Ready"))
			Expect(conditions[0].Status).To(Equal(metav1.ConditionTrue))
			Expect(conditions[0].Reason).To(Equal("Ok"))
			Expect(conditions[0].Message).To(Equal("all good"))
			Expect(conditions[0].LastTransitionTime.IsZero()).To(BeFalse())
		})

		It("updates the existing Ready condition without appending duplicates", func() {
			old := metav1.NewTime(time.Now().Add(-time.Hour))
			conditions := []metav1.Condition{{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Ok",
				Message:            "all good",
				LastTransitionTime: old,
			}}

			setReadyCondition(&conditions, metav1.ConditionFalse, "Boom", "broken")
			Expect(conditions).To(HaveLen(1))
			Expect(conditions[0].Status).To(Equal(metav1.ConditionFalse))
			Expect(conditions[0].Reason).To(Equal("Boom"))
			Expect(conditions[0].Message).To(Equal("broken"))
			Expect(conditions[0].LastTransitionTime.After(old.Time)).To(BeTrue(), "transition time should advance on status change")
		})

		It("does not bump LastTransitionTime when status is unchanged", func() {
			old := metav1.NewTime(time.Now().Add(-time.Hour))
			conditions := []metav1.Condition{{
				Type:               "Ready",
				Status:             metav1.ConditionTrue,
				Reason:             "Ok",
				Message:            "all good",
				LastTransitionTime: old,
			}}

			setReadyCondition(&conditions, metav1.ConditionTrue, "Ok2", "still good")
			Expect(conditions[0].LastTransitionTime.Time.Equal(old.Time)).To(BeTrue())
			Expect(conditions[0].Reason).To(Equal("Ok2"))
			Expect(conditions[0].Message).To(Equal("still good"))
		})
	})

	Context("fetchAgentCard fallback behavior", func() {
		It("returns an error when service is nil", func() {
			_, err := fetchAgentCard(&v1alpha1.Agent{}, nil)
			Expect(err).To(HaveOccurred())
		})
	})
})
