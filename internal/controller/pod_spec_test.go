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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
	testutil "github.com/inference-gateway/operator/internal/controller/testutil"
)

var _ = Describe("spec.pod customization", func() {
	ctx := context.Background()

	podSpec := &corev1alpha1.PodSpec{
		Labels:      map[string]string{"sidecar.example.com/inject": "true", "app": "hijacked"},
		Annotations: map[string]string{"linkerd.io/inject": "enabled"},
		NodeSelector: map[string]string{
			"kubernetes.io/hostname": "worker-0",
		},
		Tolerations: []corev1.Toleration{{
			Key:      "gpu",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		}},
		Affinity: &corev1.Affinity{
			PodAntiAffinity: &corev1.PodAntiAffinity{
				PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{{
					Weight:          100,
					PodAffinityTerm: corev1.PodAffinityTerm{TopologyKey: "kubernetes.io/hostname"},
				}},
			},
		},
	}

	gatewayTemplate := func(pod *corev1alpha1.PodSpec) corev1.PodTemplateSpec {
		r := &GatewayReconciler{Client: testutil.NewFakeClient(), Scheme: gatewayTestScheme}
		gw := &corev1alpha1.Gateway{
			ObjectMeta: metav1.ObjectMeta{Name: "workload", Namespace: "default"},
			Spec:       corev1alpha1.GatewaySpec{Pod: pod},
		}
		return r.buildDeployment(ctx, gw, false).Spec.Template
	}

	agentTemplate := func(pod *corev1alpha1.PodSpec) corev1.PodTemplateSpec {
		agent := &corev1alpha1.Agent{
			ObjectMeta: metav1.ObjectMeta{Name: "workload", Namespace: "default"},
			Spec:       corev1alpha1.AgentSpec{Image: "agent:latest", Pod: pod},
		}
		return (&AgentReconciler{}).buildAgentDeployment(agent).Spec.Template
	}

	mcpTemplate := func(pod *corev1alpha1.PodSpec) corev1.PodTemplateSpec {
		mcp := &corev1alpha1.MCP{
			ObjectMeta: metav1.ObjectMeta{Name: "workload", Namespace: "default"},
			Spec:       corev1alpha1.MCPSpec{Image: "node:lts", Pod: pod},
		}
		return (&MCPReconciler{}).buildDeployment(mcp).Spec.Template
	}

	DescribeTable("applies spec.pod to the pod template",
		func(build func(*corev1alpha1.PodSpec) corev1.PodTemplateSpec) {
			template := build(podSpec)

			Expect(template.Labels).To(HaveKeyWithValue("sidecar.example.com/inject", "true"))
			Expect(template.Labels).To(HaveKeyWithValue("app", "workload"))
			Expect(template.Annotations).To(HaveKeyWithValue("linkerd.io/inject", "enabled"))
			Expect(template.Spec.NodeSelector).To(Equal(podSpec.NodeSelector))
			Expect(template.Spec.Tolerations).To(Equal(podSpec.Tolerations))
			Expect(template.Spec.Affinity).To(Equal(podSpec.Affinity))
		},
		Entry("Gateway", gatewayTemplate),
		Entry("Agent", agentTemplate),
		Entry("MCP", mcpTemplate),
	)

	DescribeTable("leaves the pod template untouched when spec.pod is unset",
		func(build func(*corev1alpha1.PodSpec) corev1.PodTemplateSpec) {
			template := build(nil)

			Expect(template.Labels).To(Equal(map[string]string{"app": "workload"}))
			Expect(template.Annotations).To(BeEmpty())
			Expect(template.Spec.NodeSelector).To(BeNil())
			Expect(template.Spec.Tolerations).To(BeNil())
			Expect(template.Spec.Affinity).To(BeNil())
		},
		Entry("Gateway", gatewayTemplate),
		Entry("Agent", agentTemplate),
		Entry("MCP", mcpTemplate),
	)
})
