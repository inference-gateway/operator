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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

var _ = Describe("Orchestrator Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-orchestrator"

		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		orch := &v1alpha1.Orchestrator{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Orchestrator")
			err := k8sClient.Get(ctx, typeNamespacedName, orch)
			if err != nil && client.IgnoreNotFound(err) == nil {
				resource := &v1alpha1.Orchestrator{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: v1alpha1.OrchestratorSpec{
						Image: "ghcr.io/inference-gateway/cli:test",
						Channels: v1alpha1.ChannelsSpec{
							Telegram: v1alpha1.TelegramChannelSpec{
								Enabled: true,
								TokenSecretRef: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{Name: "telegram-bot-credentials"},
									Key:                  "token",
								},
							},
						},
						Gateway: v1alpha1.OrchestratorGatewaySpec{URL: "http://inference-gateway:8080"},
						Agent:   v1alpha1.OrchestratorAgentSpec{Model: "test/model"},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &v1alpha1.Orchestrator{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Orchestrator")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should create a singleton Recreate Deployment running channels-manager", func() {
			By("Reconciling the created resource")
			controllerReconciler := &OrchestratorReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			By("Checking the Deployment was created")
			deployment := &appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, deployment)
			}, time.Second*10, time.Millisecond*250).Should(Succeed())

			Expect(deployment.Spec.Replicas).NotTo(BeNil())
			Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))
			Expect(deployment.Spec.Strategy.Type).To(Equal(appsv1.RecreateDeploymentStrategyType))

			Expect(deployment.Spec.Template.Spec.Containers).To(HaveLen(1))
			c := deployment.Spec.Template.Spec.Containers[0]
			Expect(c.Command).To(Equal([]string{"infer"}))
			Expect(c.Args).To(Equal([]string{"channels-manager"}))
		})
	})
})

var _ = Describe("buildOrchestratorEnvironmentVars", func() {
	maxWorkers := int32(7)
	imageRetention := int32(3)
	requireApproval := false
	pollTimeout := metav1.Duration{Duration: 45 * time.Second}

	tokenRef := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "telegram-bot-credentials"},
		Key:                  "token",
	}
	allowedUsersRef := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "telegram-bot-credentials"},
		Key:                  "allowedUsers",
	}
	apiKeyRef := corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: "gateway-credentials"},
		Key:                  "apiKey",
	}

	makeOrchestrator := func() *v1alpha1.Orchestrator {
		return &v1alpha1.Orchestrator{
			ObjectMeta: metav1.ObjectMeta{Name: "o", Namespace: "default"},
			Spec: v1alpha1.OrchestratorSpec{
				Image: "img",
				Channels: v1alpha1.ChannelsSpec{
					MaxWorkers:      &maxWorkers,
					ImageRetention:  &imageRetention,
					RequireApproval: &requireApproval,
					Telegram: v1alpha1.TelegramChannelSpec{
						Enabled:               true,
						TokenSecretRef:        tokenRef,
						AllowedUsersSecretRef: &allowedUsersRef,
						PollTimeout:           &pollTimeout,
					},
				},
				Gateway: v1alpha1.OrchestratorGatewaySpec{
					URL:             "http://gw:8080",
					APIKeySecretRef: &apiKeyRef,
				},
				Agent: v1alpha1.OrchestratorAgentSpec{
					Model:        "openai/gpt-test",
					SystemPrompt: "be helpful",
				},
				Tools: v1alpha1.OrchestratorToolsSpec{Enabled: true, Schedule: true},
				A2A:   v1alpha1.OrchestratorA2ASpec{Enabled: true, Agents: []string{"a", "b"}},
			},
		}
	}

	envByName := func(envs []corev1.EnvVar) map[string]corev1.EnvVar {
		out := map[string]corev1.EnvVar{}
		for _, e := range envs {
			out[e.Name] = e
		}
		return out
	}

	It("emits hardcoded INFER_* defaults", func() {
		envs := envByName(buildOrchestratorEnvironmentVars(makeOrchestrator()))
		Expect(envs["INFER_CHANNELS_ENABLED"].Value).To(Equal("true"))
		Expect(envs["INFER_LOGGING_STDOUT"].Value).To(Equal("true"))
		Expect(envs["INFER_GATEWAY_RUN"].Value).To(Equal("false"))
	})

	It("translates plain channel knobs", func() {
		envs := envByName(buildOrchestratorEnvironmentVars(makeOrchestrator()))
		Expect(envs["INFER_CHANNELS_MAX_WORKERS"].Value).To(Equal("7"))
		Expect(envs["INFER_CHANNELS_IMAGE_RETENTION"].Value).To(Equal("3"))
		Expect(envs["INFER_CHANNELS_REQUIRE_APPROVAL"].Value).To(Equal("false"))
	})

	It("wires Telegram fields including secret refs and pollTimeout", func() {
		envs := envByName(buildOrchestratorEnvironmentVars(makeOrchestrator()))

		Expect(envs["INFER_CHANNELS_TELEGRAM_ENABLED"].Value).To(Equal("true"))
		Expect(envs["INFER_CHANNELS_TELEGRAM_POLL_TIMEOUT"].Value).To(Equal("45"))

		token := envs["INFER_CHANNELS_TELEGRAM_BOT_TOKEN"]
		Expect(token.Value).To(BeEmpty())
		Expect(token.ValueFrom).NotTo(BeNil())
		Expect(token.ValueFrom.SecretKeyRef).NotTo(BeNil())
		Expect(token.ValueFrom.SecretKeyRef.Name).To(Equal("telegram-bot-credentials"))
		Expect(token.ValueFrom.SecretKeyRef.Key).To(Equal("token"))

		users := envs["INFER_CHANNELS_TELEGRAM_ALLOWED_USERS"]
		Expect(users.ValueFrom).NotTo(BeNil())
		Expect(users.ValueFrom.SecretKeyRef.Name).To(Equal("telegram-bot-credentials"))
		Expect(users.ValueFrom.SecretKeyRef.Key).To(Equal("allowedUsers"))
	})

	It("wires gateway, agent, tools, and a2a", func() {
		envs := envByName(buildOrchestratorEnvironmentVars(makeOrchestrator()))

		Expect(envs["INFER_GATEWAY_URL"].Value).To(Equal("http://gw:8080"))
		apiKey := envs["INFER_GATEWAY_API_KEY"]
		Expect(apiKey.ValueFrom).NotTo(BeNil())
		Expect(apiKey.ValueFrom.SecretKeyRef.Name).To(Equal("gateway-credentials"))
		Expect(apiKey.ValueFrom.SecretKeyRef.Key).To(Equal("apiKey"))

		Expect(envs["INFER_AGENT_MODEL"].Value).To(Equal("openai/gpt-test"))
		Expect(envs["INFER_AGENT_SYSTEM_PROMPT"].Value).To(Equal("be helpful"))
		Expect(envs["INFER_TOOLS_ENABLED"].Value).To(Equal("true"))
		Expect(envs["INFER_TOOLS_SCHEDULE_ENABLED"].Value).To(Equal("true"))
		Expect(envs["INFER_A2A_ENABLED"].Value).To(Equal("true"))
		Expect(envs["INFER_A2A_AGENTS"].Value).To(Equal("a,b"))
	})

	It("omits optional channel fields when nil", func() {
		orch := makeOrchestrator()
		orch.Spec.Channels.MaxWorkers = nil
		orch.Spec.Channels.ImageRetention = nil
		orch.Spec.Channels.RequireApproval = nil
		orch.Spec.Channels.Telegram.AllowedUsersSecretRef = nil
		orch.Spec.Channels.Telegram.PollTimeout = nil
		orch.Spec.Gateway.APIKeySecretRef = nil

		envs := envByName(buildOrchestratorEnvironmentVars(orch))
		Expect(envs).NotTo(HaveKey("INFER_CHANNELS_MAX_WORKERS"))
		Expect(envs).NotTo(HaveKey("INFER_CHANNELS_IMAGE_RETENTION"))
		Expect(envs).NotTo(HaveKey("INFER_CHANNELS_REQUIRE_APPROVAL"))
		Expect(envs).NotTo(HaveKey("INFER_CHANNELS_TELEGRAM_ALLOWED_USERS"))
		Expect(envs).NotTo(HaveKey("INFER_CHANNELS_TELEGRAM_POLL_TIMEOUT"))
		Expect(envs).NotTo(HaveKey("INFER_GATEWAY_API_KEY"))
	})

	It("passes through spec.env", func() {
		orch := makeOrchestrator()
		orch.Spec.Env = &[]corev1.EnvVar{{Name: "EXTRA", Value: "yes"}}
		envs := envByName(buildOrchestratorEnvironmentVars(orch))
		Expect(envs["EXTRA"].Value).To(Equal("yes"))
	})
})

var _ = Describe("buildAgentsYAML", func() {
	It("emits empty agents list when no static or discovered agents", func() {
		yaml := buildAgentsYAML(nil, nil)
		Expect(yaml).To(Equal("agents:\n"))
	})

	It("emits static agents with synthetic names", func() {
		yaml := buildAgentsYAML([]string{"http://a:8080", "http://b:9090"}, nil)
		Expect(yaml).To(ContainSubstring("- name: static-agent-0\n"))
		Expect(yaml).To(ContainSubstring("    url: http://a:8080\n"))
		Expect(yaml).To(ContainSubstring("- name: static-agent-1\n"))
		Expect(yaml).To(ContainSubstring("    url: http://b:9090\n"))
		Expect(yaml).To(ContainSubstring("    enabled: true\n"))
		Expect(yaml).To(ContainSubstring("    run: false\n"))
	})

	It("emits discovered agents by CR name with derived cluster URL", func() {
		agents := []v1alpha1.Agent{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "my-agent", Namespace: "agents"},
				Spec:       v1alpha1.AgentSpec{Port: 8080},
			},
		}
		yaml := buildAgentsYAML(nil, agents)
		Expect(yaml).To(ContainSubstring("- name: my-agent\n"))
		Expect(yaml).To(ContainSubstring("    url: http://my-agent.agents.svc.cluster.local:8080\n"))
	})

	It("uses default port 8080 when agent port is zero", func() {
		agents := []v1alpha1.Agent{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "zero-port", Namespace: "ns"},
				Spec:       v1alpha1.AgentSpec{Port: 0},
			},
		}
		yaml := buildAgentsYAML(nil, agents)
		Expect(yaml).To(ContainSubstring("url: http://zero-port.ns.svc.cluster.local:8080\n"))
	})

	It("sorts discovered agents by name for determinism", func() {
		agents := []v1alpha1.Agent{
			{ObjectMeta: metav1.ObjectMeta{Name: "zebra", Namespace: "ns"}, Spec: v1alpha1.AgentSpec{Port: 8080}},
			{ObjectMeta: metav1.ObjectMeta{Name: "alpha", Namespace: "ns"}, Spec: v1alpha1.AgentSpec{Port: 8080}},
		}
		yaml := buildAgentsYAML(nil, agents)
		alphaPos := strings.Index(yaml, "name: alpha")
		zebraPos := strings.Index(yaml, "name: zebra")
		Expect(alphaPos).To(BeNumerically("<", zebraPos))
	})

	It("combines static and discovered agents", func() {
		agents := []v1alpha1.Agent{
			{ObjectMeta: metav1.ObjectMeta{Name: "disc", Namespace: "ns"}, Spec: v1alpha1.AgentSpec{Port: 8080}},
		}
		yaml := buildAgentsYAML([]string{"http://static:8080"}, agents)
		Expect(yaml).To(ContainSubstring("name: static-agent-0"))
		Expect(yaml).To(ContainSubstring("name: disc"))
	})
})

var _ = Describe("buildOrchestratorDeployment with service discovery", func() {
	makeOrchestratorWithDiscovery := func(enabled bool) *v1alpha1.Orchestrator {
		return &v1alpha1.Orchestrator{
			ObjectMeta: metav1.ObjectMeta{Name: "orch", Namespace: "default"},
			Spec: v1alpha1.OrchestratorSpec{
				Image: "img",
				Channels: v1alpha1.ChannelsSpec{
					Telegram: v1alpha1.TelegramChannelSpec{
						Enabled: true,
						TokenSecretRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{Name: "secret"},
							Key:                  "token",
						},
					},
				},
				Gateway: v1alpha1.OrchestratorGatewaySpec{URL: "http://gw:8080"},
				Agent:   v1alpha1.OrchestratorAgentSpec{Model: "m"},
				A2A: v1alpha1.OrchestratorA2ASpec{
					Enabled: true,
					ServiceDiscovery: v1alpha1.OrchestratorServiceDiscoverySpec{
						Enabled:   enabled,
						Namespace: "agents",
					},
				},
			},
		}
	}

	r := &OrchestratorReconciler{}

	It("does not mount agents configmap when service discovery is disabled", func() {
		orch := makeOrchestratorWithDiscovery(false)
		dep := r.buildOrchestratorDeployment(orch)
		Expect(dep.Spec.Template.Spec.Volumes).To(BeEmpty())
		Expect(dep.Spec.Template.Spec.Containers[0].VolumeMounts).To(BeEmpty())
	})

	It("mounts agents configmap at /root/.infer/agents.yaml when service discovery is enabled", func() {
		orch := makeOrchestratorWithDiscovery(true)
		dep := r.buildOrchestratorDeployment(orch)

		Expect(dep.Spec.Template.Spec.Volumes).To(HaveLen(1))
		vol := dep.Spec.Template.Spec.Volumes[0]
		Expect(vol.Name).To(Equal("agents-config"))
		Expect(vol.ConfigMap).NotTo(BeNil())
		Expect(vol.ConfigMap.Name).To(Equal("orch-agents"))

		mounts := dep.Spec.Template.Spec.Containers[0].VolumeMounts
		Expect(mounts).To(HaveLen(1))
		Expect(mounts[0].Name).To(Equal("agents-config"))
		Expect(mounts[0].MountPath).To(Equal("/root/.infer/agents.yaml"))
		Expect(mounts[0].SubPath).To(Equal("agents.yaml"))
	})
})
