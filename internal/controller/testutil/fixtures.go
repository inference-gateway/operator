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

// Package testutil holds reusable building blocks for the controller test
// suite: a scheme registered with the project + Gateway API types, a fake
// client constructor, and typed CR builders (Gateway/Agent/MCP) that use the
// functional-options pattern. Centralising fixtures here keeps the per-controller
// specs short and consistent and lets helpers be shared across controllers.
package testutil

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	fake "sigs.k8s.io/controller-runtime/pkg/client/fake"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

const apiVersion = "core.inference-gateway.com/v1alpha1"

// Scheme returns a runtime.Scheme registered with the client-go built-ins, the
// project v1alpha1 API and the Gateway API v1 types - enough for any controller
// test to build a fake client or decode operator-managed objects.
func Scheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = corev1alpha1.AddToScheme(s)
	_ = gwapiv1.Install(s)
	return s
}

// NewFakeClient builds a controller-runtime fake client seeded with objs and
// backed by Scheme(). Suited to unit-testing reconciler helpers in isolation,
// without standing up envtest.
func NewFakeClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(Scheme()).
		WithObjects(objs...).
		Build()
}

// GatewayOption mutates a Gateway fixture.
type GatewayOption func(*corev1alpha1.Gateway)

// NewGateway returns a minimal, valid Gateway CR. Apply options to opt into
// additional spec (replicas, a non-default image, ...).
func NewGateway(name, namespace string, opts ...GatewayOption) *corev1alpha1.Gateway {
	gateway := &corev1alpha1.Gateway{
		TypeMeta: metav1.TypeMeta{APIVersion: apiVersion, Kind: "Gateway"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1alpha1.GatewaySpec{
			Environment: "development",
			Image:       "ghcr.io/inference-gateway/inference-gateway:latest",
		},
	}
	for _, opt := range opts {
		opt(gateway)
	}
	return gateway
}

// WithGatewayImage overrides spec.image.
func WithGatewayImage(image string) GatewayOption {
	return func(g *corev1alpha1.Gateway) { g.Spec.Image = image }
}

// WithGatewayReplicas pins spec.replicas.
func WithGatewayReplicas(replicas int32) GatewayOption {
	return func(g *corev1alpha1.Gateway) { g.Spec.Replicas = &replicas }
}

// AgentOption mutates an Agent fixture.
type AgentOption func(*corev1alpha1.Agent)

// NewAgent returns a minimal, valid Agent CR (valid image + port). Apply options
// to diverge - e.g. WithAgentImage("") to exercise invalid-spec handling.
func NewAgent(name, namespace string, opts ...AgentOption) *corev1alpha1.Agent {
	agent := &corev1alpha1.Agent{
		TypeMeta: metav1.TypeMeta{APIVersion: apiVersion, Kind: "Agent"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1alpha1.AgentSpec{
			Image: "ghcr.io/inference-gateway/agent:latest",
			Port:  8080,
		},
	}
	for _, opt := range opts {
		opt(agent)
	}
	return agent
}

// WithAgentImage overrides spec.image (pass "" to build an invalid Agent).
func WithAgentImage(image string) AgentOption {
	return func(a *corev1alpha1.Agent) { a.Spec.Image = image }
}

// WithAgentPort overrides spec.port.
func WithAgentPort(port int32) AgentOption {
	return func(a *corev1alpha1.Agent) { a.Spec.Port = port }
}

// MCPOption mutates an MCP fixture.
type MCPOption func(*corev1alpha1.MCP)

// NewMCP returns a minimal MCP CR with a server listening on port 3000. Apply
// options to adjust the server config.
func NewMCP(name, namespace string, opts ...MCPOption) *corev1alpha1.MCP {
	mcp := &corev1alpha1.MCP{
		TypeMeta: metav1.TypeMeta{APIVersion: apiVersion, Kind: "MCP"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1alpha1.MCPSpec{
			Server: &corev1alpha1.MCPServerSpec{Port: 3000},
		},
	}
	for _, opt := range opts {
		opt(mcp)
	}
	return mcp
}

// WithMCPServerPort sets spec.server.port.
func WithMCPServerPort(port int32) MCPOption {
	return func(m *corev1alpha1.MCP) {
		if m.Spec.Server == nil {
			m.Spec.Server = &corev1alpha1.MCPServerSpec{}
		}
		m.Spec.Server.Port = port
	}
}
