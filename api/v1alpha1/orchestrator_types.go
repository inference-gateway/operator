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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrchestratorSpec defines the desired state of Orchestrator.
//
// An Orchestrator deploys the Inference Gateway CLI's `channels-manager` daemon:
// an LLM-driven loop that receives messages from a chat channel, optionally
// fans out to A2A Agents and tools (incl. MCP), and replies. The Deployment is
// always a singleton (replicas=1, strategy=Recreate) because Telegram allows
// only one active getUpdates consumer per token.
type OrchestratorSpec struct {
	Image    string                  `json:"image"`
	Channels ChannelsSpec            `json:"channels"`
	Gateway  OrchestratorGatewaySpec `json:"gateway"`
	Agent    OrchestratorAgentSpec   `json:"agent"`

	// +optional
	Tools OrchestratorToolsSpec `json:"tools,omitempty"`

	// +optional
	A2A OrchestratorA2ASpec `json:"a2a,omitempty"`

	// MCP configures Model Context Protocol integration, including optional
	// label-selector discovery of MCP CRs in the cluster.
	// +optional
	MCP OrchestratorMCPSpec `json:"mcp,omitempty"`

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Env is an optional passthrough for additional environment variables.
	// +optional
	Env *[]corev1.EnvVar `json:"env,omitempty"`

	// Telemetry configures observability (traces and metrics) for the
	// orchestrator. Mapped onto INFER_TELEMETRY_* env vars consumed by
	// the `infer channels-manager` CLI.
	// +optional
	Telemetry *TelemetrySpec `json:"telemetry,omitempty"`
}

// ChannelsSpec mirrors the CLI's ChannelsConfig top-level shared fields.
// `enabled` is omitted because the controller always sets INFER_CHANNELS_ENABLED=true.
type ChannelsSpec struct {
	// +optional
	MaxWorkers *int32 `json:"maxWorkers,omitempty"`
	// +optional
	ImageRetention *int32 `json:"imageRetention,omitempty"`
	// +optional
	RequireApproval *bool `json:"requireApproval,omitempty"`

	Telegram TelegramChannelSpec `json:"telegram"`
}

// TelegramChannelSpec mirrors the CLI's TelegramChannelConfig.
// Token and allowed users are sourced from Secrets and injected as env vars
// via valueFrom.secretKeyRef.
type TelegramChannelSpec struct {
	Enabled        bool                     `json:"enabled"`
	TokenSecretRef corev1.SecretKeySelector `json:"tokenSecretRef"`

	// +optional
	AllowedUsersSecretRef *corev1.SecretKeySelector `json:"allowedUsersSecretRef,omitempty"`

	// +optional
	PollTimeout *metav1.Duration `json:"pollTimeout,omitempty"`
}

// OrchestratorGatewaySpec configures how the orchestrator reaches the Inference Gateway.
type OrchestratorGatewaySpec struct {
	URL string `json:"url"`

	// +optional
	APIKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`
}

// OrchestratorAgentSpec configures the orchestrating LLM agent.
type OrchestratorAgentSpec struct {
	Model string `json:"model"`

	// +optional
	SystemPrompt string `json:"systemPrompt,omitempty"`
}

// OrchestratorToolsSpec toggles built-in CLI tools.
type OrchestratorToolsSpec struct {
	Enabled bool `json:"enabled"`

	// +optional
	Schedule bool `json:"schedule,omitempty"`
}

// OrchestratorServiceDiscoverySpec configures automatic discovery of Agent or MCP CRs.
type OrchestratorServiceDiscoverySpec struct {
	// Enabled toggles automatic service discovery.
	Enabled bool `json:"enabled"`

	// Namespace is the namespace to discover CRs in.
	// Defaults to the Orchestrator's own namespace when empty.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Selector filters which CRs are discovered by their labels.
	// A nil or empty selector matches all CRs in the namespace.
	// Supports matchLabels and matchExpressions.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// OrchestratorA2ASpec configures Agent-to-Agent integration.
type OrchestratorA2ASpec struct {
	Enabled bool `json:"enabled"`

	// +optional
	Agents []string `json:"agents,omitempty"`

	// ServiceDiscovery configures automatic discovery of Agent CRs by label selector.
	// Discovered agents are written into agents.yaml and mounted in the orchestrator pod.
	// The pod is rolled when the discovered set changes (a content hash is stamped on the
	// pod template) because Kubernetes does not propagate live updates to ConfigMap volumes
	// mounted with subPath.
	// +optional
	ServiceDiscovery OrchestratorServiceDiscoverySpec `json:"serviceDiscovery,omitempty"`
}

// OrchestratorMCPSpec configures Model Context Protocol integration. When
// ServiceDiscovery is enabled, the controller discovers MCP CRs matching the
// selector and writes them into mcp.yaml mounted at ~/.infer/mcp.yaml inside
// the orchestrator pod. The pod is rolled when the discovered set changes
// (a content hash is stamped on the pod template) because Kubernetes does not
// propagate live updates to ConfigMap volumes mounted with subPath.
type OrchestratorMCPSpec struct {
	Enabled bool `json:"enabled"`

	// Servers is a static list of MCP server URLs, kept for parity with
	// A2A.Agents. Useful for referencing externally-hosted MCPs that have no
	// MCP CR in the cluster.
	// +optional
	Servers []string `json:"servers,omitempty"`

	// ServiceDiscovery configures automatic discovery of MCP CRs by label selector.
	// +optional
	ServiceDiscovery OrchestratorServiceDiscoverySpec `json:"serviceDiscovery,omitempty"`
}

// OrchestratorStatus defines the observed state of Orchestrator.
type OrchestratorStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	Ready bool `json:"ready,omitempty"`

	// DiscoveredAgents is the sorted list of agent URLs found via service discovery.
	// +optional
	DiscoveredAgents []string `json:"discoveredAgents,omitempty"`

	// DiscoveredAgentCount is the number of agents currently discovered via service discovery.
	// +optional
	DiscoveredAgentCount int32 `json:"discoveredAgentCount,omitempty"`

	// DiscoveredMCPs is the sorted list of MCP server URLs found via service discovery.
	// +optional
	DiscoveredMCPs []string `json:"discoveredMCPs,omitempty"`

	// DiscoveredMCPCount is the number of MCP servers currently discovered via service discovery.
	// +optional
	DiscoveredMCPCount int32 `json:"discoveredMCPCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=orch,categories=inference-gateway
// +kubebuilder:printcolumn:name="READY",type=boolean,JSONPath=".status.ready",description="Whether the Orchestrator Deployment is available"
// +kubebuilder:printcolumn:name="AGENTS",type=integer,JSONPath=".status.discoveredAgentCount",description="Number of discovered agents"
// +kubebuilder:printcolumn:name="MCPS",type=integer,JSONPath=".status.discoveredMCPCount",description="Number of discovered MCP servers"
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp",description="Age of the resource"

// Orchestrator is the Schema for the orchestrators API.
type Orchestrator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrchestratorSpec   `json:"spec,omitempty"`
	Status OrchestratorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrchestratorList contains a list of Orchestrator.
type OrchestratorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Orchestrator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Orchestrator{}, &OrchestratorList{})
}
