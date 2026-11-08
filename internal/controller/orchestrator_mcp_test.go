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
	"bytes"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	yaml "go.yaml.in/yaml/v3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

// cliMCPConfig is mcp.yaml as the CLI declares it, decoded with yaml.v3 as the
// CLI does, so a key the CLI does not know fails the decode.
type cliMCPConfig struct {
	Enabled bool           `yaml:"enabled"`
	Servers []cliMCPServer `yaml:"servers"`
}

type cliMCPServer struct {
	Name    string            `yaml:"name"`
	Enabled bool              `yaml:"enabled"`
	URL     string            `yaml:"url"`
	Headers map[string]string `yaml:"headers"`
	Scheme  string            `yaml:"scheme"`
	Host    string            `yaml:"host"`
	Port    int               `yaml:"port"`
	Path    string            `yaml:"path"`
}

func parseCLIMCPConfig(rendered string) cliMCPConfig {
	decoder := yaml.NewDecoder(bytes.NewBufferString(rendered))
	decoder.KnownFields(true)
	var cfg cliMCPConfig
	ExpectWithOffset(1, decoder.Decode(&cfg)).To(Succeed())
	return cfg
}

func renderMCPs(spec corev1alpha1.OrchestratorMCPSpec, discovered []corev1alpha1.MCP) cliMCPConfig {
	rendered, err := buildMCPsYAML(spec, discovered)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	return parseCLIMCPConfig(rendered)
}

var _ = Describe("buildMCPsYAML", func() {
	It("enables MCP with no servers when there are no static, hosted or discovered MCPs", func() {
		cfg := renderMCPs(corev1alpha1.OrchestratorMCPSpec{}, nil)
		Expect(cfg.Enabled).To(BeTrue())
		Expect(cfg.Servers).To(BeEmpty())
	})

	It("writes a static URL under a synthetic name with its url and its split fields", func() {
		cfg := renderMCPs(corev1alpha1.OrchestratorMCPSpec{Servers: []string{"http://x:3000/mcp", "http://y:3000/mcp"}}, nil)
		Expect(cfg.Servers).To(Equal([]cliMCPServer{
			{Name: "static-mcp-0", Enabled: true, URL: "http://x:3000/mcp", Scheme: "http", Host: "x", Port: 3000, Path: "/mcp"},
			{Name: "static-mcp-1", Enabled: true, URL: "http://y:3000/mcp", Scheme: "http", Host: "y", Port: 3000, Path: "/mcp"},
		}))
	})

	It("writes a hosted server under its name with its headers exactly as given", func() {
		cfg := renderMCPs(corev1alpha1.OrchestratorMCPSpec{HostedServers: []corev1alpha1.OrchestratorHostedMCPServer{{
			Name:    "mailbox",
			URL:     "https://mcp.example.com/mcp",
			Headers: map[string]string{"x-api-key": "${MAILBOX_API_KEY}", "Authorization": "Bearer ${MAILBOX_TOKEN}"},
		}}}, nil)
		Expect(cfg.Servers).To(Equal([]cliMCPServer{{
			Name: "mailbox", Enabled: true, URL: "https://mcp.example.com/mcp",
			Headers: map[string]string{"x-api-key": "${MAILBOX_API_KEY}", "Authorization": "Bearer ${MAILBOX_TOKEN}"},
			Scheme:  "https", Host: "mcp.example.com", Port: 443, Path: "/mcp",
		}}))
	})

	It("keeps a header value that looks like YAML inside its one string", func() {
		injected := "${KEY}\n  - name: injected\n    url: http://evil.example/mcp"
		cfg := renderMCPs(corev1alpha1.OrchestratorMCPSpec{HostedServers: []corev1alpha1.OrchestratorHostedMCPServer{{
			Name: "mailbox", URL: "https://mcp.example.com/mcp", Headers: map[string]string{"x-api-key": injected},
		}}}, nil)
		Expect(cfg.Servers).To(HaveLen(1))
		Expect(cfg.Servers[0].Headers).To(Equal(map[string]string{"x-api-key": injected}))
	})

	It("orders the static servers, then the hosted ones in spec order, then the discovered MCPs by name", func() {
		cfg := renderMCPs(corev1alpha1.OrchestratorMCPSpec{
			Servers: []string{"http://static/mcp"},
			HostedServers: []corev1alpha1.OrchestratorHostedMCPServer{
				{Name: "zulu", URL: "https://zulu.example.com/mcp"},
				{Name: "alpha", URL: "https://alpha.example.com/mcp"},
			},
		}, []corev1alpha1.MCP{
			{ObjectMeta: metav1.ObjectMeta{Name: "zebra", Namespace: "mcp"}, Status: corev1alpha1.MCPStatus{URL: "http://z/mcp"}},
			{ObjectMeta: metav1.ObjectMeta{Name: "apple", Namespace: "mcp"}, Status: corev1alpha1.MCPStatus{URL: "http://a/mcp"}},
		})
		names := make([]string, 0, len(cfg.Servers))
		for _, server := range cfg.Servers {
			names = append(names, server.Name)
		}
		Expect(names).To(Equal([]string{"static-mcp-0", "zulu", "alpha", "apple", "zebra"}))
	})

	It("prefers status.URL when populated, falling back to the deterministic construction", func() {
		cfg := renderMCPs(corev1alpha1.OrchestratorMCPSpec{}, []corev1alpha1.MCP{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "with-status", Namespace: "mcp"},
				Status:     corev1alpha1.MCPStatus{URL: "https://override.example/mcp"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "no-status", Namespace: "mcp"},
				Spec:       corev1alpha1.MCPSpec{Server: &corev1alpha1.MCPServerSpec{Port: 3000}},
			},
		})
		Expect(cfg.Servers).To(Equal([]cliMCPServer{
			{Name: "no-status", Enabled: true, URL: "http://no-status-service.mcp.svc.cluster.local:3000/mcp", Scheme: "http", Host: "no-status-service.mcp.svc.cluster.local", Port: 3000, Path: "/mcp"},
			{Name: "with-status", Enabled: true, URL: "https://override.example/mcp", Scheme: "https", Host: "override.example", Port: 443, Path: "/mcp"},
		}))
	})

	It("uses https when TLS is enabled on the MCP server", func() {
		cfg := renderMCPs(corev1alpha1.OrchestratorMCPSpec{}, []corev1alpha1.MCP{{
			ObjectMeta: metav1.ObjectMeta{Name: "tls-mcp", Namespace: "mcp"},
			Spec: corev1alpha1.MCPSpec{Server: &corev1alpha1.MCPServerSpec{
				Port: 3000,
				TLS:  &corev1alpha1.MCPTLSConfig{Enabled: true, SecretName: "x"},
			}},
		}})
		Expect(cfg.Servers).To(HaveLen(1))
		Expect(cfg.Servers[0]).To(SatisfyAll(
			HaveField("Scheme", "https"),
			HaveField("Host", "tls-mcp-service.mcp.svc.cluster.local"),
			HaveField("Port", 3000),
		))
	})
})

var _ = Describe("Orchestrator with static and hosted MCP servers and no discovery", func() {
	ctx := context.Background()

	It("renders and mounts mcp.yaml from its servers alone", func() {
		orch := &corev1alpha1.Orchestrator{
			ObjectMeta: metav1.ObjectMeta{Name: "hosted-mcps", Namespace: "default"},
			Spec: corev1alpha1.OrchestratorSpec{
				Image: "img",
				Daemon: &corev1alpha1.OrchestratorDaemonSpec{Binding: &corev1alpha1.DaemonBindingSpec{
					TokenSecretRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "binding-credentials"}, Key: "token"},
					TLS:            corev1alpha1.DaemonBindingTLSSpec{SecretName: "binding-tls"},
				}},
				MCP: corev1alpha1.OrchestratorMCPSpec{
					Enabled: true,
					Servers: []string{"http://static.example.com:3000/mcp"},
					HostedServers: []corev1alpha1.OrchestratorHostedMCPServer{{
						Name:    "mailbox",
						URL:     "https://mcp.example.com/mcp",
						Headers: map[string]string{"x-api-key": "${MAILBOX_API_KEY}"},
					}},
				},
			},
		}
		Expect(k8sClient.Create(ctx, orch)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, orch))).To(Succeed()) })

		reconciler := &OrchestratorReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
		_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "hosted-mcps", Namespace: "default"}})
		Expect(err).NotTo(HaveOccurred())

		cm := &corev1.ConfigMap{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: "hosted-mcps-mcps", Namespace: "default"}, cm)).To(Succeed())
		cfg := parseCLIMCPConfig(cm.Data["mcp.yaml"])
		Expect(cfg.Servers).To(HaveLen(2))
		Expect(cfg.Servers[1]).To(SatisfyAll(
			HaveField("Name", "mailbox"),
			HaveField("URL", "https://mcp.example.com/mcp"),
			HaveField("Headers", map[string]string{"x-api-key": "${MAILBOX_API_KEY}"}),
		))

		template := buildOrchestratorPodTemplate(orch, "", cm.Data["mcp.yaml"])
		Expect(template.Spec.Containers[0].VolumeMounts).To(ContainElement(SatisfyAll(
			HaveField("Name", "mcps-config"),
			HaveField("MountPath", "/home/infer/.infer/mcp.yaml"),
		)))
		Expect(template.Annotations).To(HaveKey("inference-gateway.com/mcps-config-hash"))
	})

	It("rolls the Sandbox when a hosted server's mcp.yaml changes", func() {
		orch := &corev1alpha1.Orchestrator{
			ObjectMeta: metav1.ObjectMeta{Name: "hosted-sandbox", Namespace: "default"},
			Spec: corev1alpha1.OrchestratorSpec{
				Image:   "img",
				Runtime: corev1alpha1.RuntimeSandbox,
				MCP: corev1alpha1.OrchestratorMCPSpec{
					Enabled:       true,
					HostedServers: []corev1alpha1.OrchestratorHostedMCPServer{{Name: "mailbox", URL: "https://mcp.example.com/mcp"}},
				},
			},
		}
		hashOf := func(url string) string {
			orch.Spec.MCP.HostedServers[0].URL = url
			rendered, err := buildMCPsYAML(orch.Spec.MCP, nil)
			Expect(err).NotTo(HaveOccurred())
			_, hash, err := sandboxPodTemplateObject(buildOrchestratorPodTemplate(orch, "", rendered))
			Expect(err).NotTo(HaveOccurred())
			return hash
		}
		Expect(hashOf("https://mcp.example.com/mcp")).NotTo(Equal(hashOf("https://mcp.example.com/v2/mcp")))
	})

	It("renders nothing while MCP is disabled, whatever servers it lists", func() {
		mcp := corev1alpha1.OrchestratorMCPSpec{HostedServers: []corev1alpha1.OrchestratorHostedMCPServer{{Name: "mailbox", URL: "https://mcp.example.com/mcp"}}}
		Expect(rendersMCPs(mcp)).To(BeFalse())
		mcp.Enabled = true
		Expect(rendersMCPs(mcp)).To(BeTrue())
	})
})
