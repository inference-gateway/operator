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
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrchestratorSpec defines the desired state of Orchestrator.
//
// An Orchestrator deploys the Inference Gateway CLI's `infer daemon`: an
// LLM-driven loop that listens on chat channels, serves devices over its AG-UI
// binding, and fans out to A2A Agents and tools (incl. MCP). It always runs one
// daemon, as a Recreate Deployment or an agent-sandbox Sandbox, because one
// daemon owns its channels and sessions.
// +kubebuilder:validation:XValidation:rule="!has(self.sandbox) || self.runtime == 'Sandbox'",message="spec.sandbox needs spec.runtime: Sandbox"
type OrchestratorSpec struct {
	Image string `json:"image"`

	// Runtime is the workload the daemon runs as, fixed at creation because a
	// Sandbox's volumes would not survive a switch. Sandbox needs agent-sandbox
	// (agents.x-k8s.io) installed in the cluster.
	// +optional
	// +kubebuilder:default=Deployment
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.runtime is fixed when the Orchestrator is created"
	Runtime OrchestratorRuntime `json:"runtime,omitempty"`

	// Sandbox configures the Sandbox the daemon runs in with runtime: Sandbox.
	// +optional
	Sandbox *OrchestratorSandboxSpec `json:"sandbox,omitempty"`

	// Channels configures the chat channels the daemon listens on.
	// +optional
	Channels *ChannelsSpec `json:"channels,omitempty"`

	// Gateway configures how the daemon reaches the Inference Gateway.
	// +optional
	Gateway *OrchestratorGatewaySpec `json:"gateway,omitempty"`

	// Agent configures the daemon's LLM agent.
	// +optional
	Agent *OrchestratorAgentSpec `json:"agent,omitempty"`

	// Daemon configures what `infer daemon` serves beyond its channels.
	// +optional
	Daemon *OrchestratorDaemonSpec `json:"daemon,omitempty"`

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
	// the `infer daemon` CLI.
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

	// +optional
	Telegram *TelegramChannelSpec `json:"telegram,omitempty"`
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

// OrchestratorGatewaySpec configures how the orchestrator reaches the Inference
// Gateway: a URL for a gateway the operator does not manage, or a Gateway in the
// same namespace, which the operator addresses and scales for this daemon.
// +kubebuilder:validation:XValidation:rule="has(self.url) != has(self.ref)",message="spec.gateway needs url or ref, not both"
type OrchestratorGatewaySpec struct {
	// URL is the address the daemon calls the gateway at.
	// +optional
	URL string `json:"url,omitempty"`

	// Ref names a Gateway in the same namespace. The operator derives the
	// daemon's gateway URL from that Gateway's Service, and runs the Gateway
	// with zero replicas while every Orchestrator referencing it is suspended.
	// +optional
	Ref string `json:"ref,omitempty"`

	// +optional
	APIKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`
}

// OrchestratorRuntime is the workload an Orchestrator's daemon runs as.
// +kubebuilder:validation:Enum=Deployment;Sandbox
type OrchestratorRuntime string

const (
	// RuntimeDeployment runs the daemon as a singleton Recreate Deployment.
	RuntimeDeployment OrchestratorRuntime = "Deployment"
	// RuntimeSandbox runs the daemon in an agent-sandbox Sandbox: a stable pod
	// with persistent volumes that can be suspended and resumed.
	RuntimeSandbox OrchestratorRuntime = "Sandbox"
)

// SandboxOperatingMode runs or parks the daemon's Sandbox.
// +kubebuilder:validation:Enum=Running;Suspended
type SandboxOperatingMode string

const (
	// SandboxRunning keeps the Sandbox's pod running.
	SandboxRunning SandboxOperatingMode = "Running"
	// SandboxSuspended deletes the Sandbox's pod and keeps its volumes.
	SandboxSuspended SandboxOperatingMode = "Suspended"
)

// OrchestratorSandboxSpec configures the Sandbox the daemon runs in. Its
// volumes hold the daemon's workspace (/workspace) and its state
// (~/.infer/projects), and outlive a suspension. The pod has no Kubernetes API
// access and runs as a non-root user without capabilities.
type OrchestratorSandboxSpec struct {
	// Storage is the size of the workspace and of the state volume, fixed when
	// the Sandbox is created.
	// +optional
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="spec.sandbox.storage is fixed when the Sandbox is created"
	Storage *resource.Quantity `json:"storage,omitempty"`

	// OperatingMode runs the Sandbox or suspends it, keeping its volumes. The
	// operator sets Suspended itself when the daemon idles out, so a client
	// resumes the daemon by setting Running.
	// +optional
	// +kubebuilder:default=Running
	OperatingMode SandboxOperatingMode `json:"operatingMode,omitempty"`

	// ShutdownTime deletes the Orchestrator, its Sandbox and the volumes once
	// it passes.
	// +optional
	ShutdownTime *metav1.Time `json:"shutdownTime,omitempty"`

	// Retention deletes the Orchestrator this long after its daemon idled out
	// with every agent done, unless a client resumed it first.
	// +optional
	Retention *metav1.Duration `json:"retention,omitempty"`

	// RuntimeClassName runs the pod under a RuntimeClass such as gVisor or Kata,
	// the isolation agent-sandbox recommends for untrusted, LLM-driven code.
	// +optional
	RuntimeClassName *string `json:"runtimeClassName,omitempty"`
}

// OrchestratorDaemonSpec mirrors the CLI's daemon config for what the daemon
// serves to devices and other daemons.
type OrchestratorDaemonSpec struct {
	// Binding exposes the daemon's AG-UI binding to devices and other daemons.
	// +optional
	Binding *DaemonBindingSpec `json:"binding,omitempty"`
}

// DaemonBindingSpec exposes the daemon's AG-UI binding, the WebSocket the
// chat TUI, the browser extension and other daemons sync sessions over. It
// listens on every interface, so it always serves TLS, and clients prove who
// they are with the token, a client certificate, or both.
// +kubebuilder:validation:XValidation:rule="has(self.tokenSecretRef) || (has(self.tls.clientAuth) && self.tls.clientAuth)",message="binding needs tokenSecretRef or tls.clientAuth to authenticate clients"
type DaemonBindingSpec struct {
	// Port is the port the binding listens on.
	// +optional
	// +kubebuilder:default=52789
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// TokenSecretRef selects the token clients present in their hello.
	// +optional
	TokenSecretRef *corev1.SecretKeySelector `json:"tokenSecretRef,omitempty"`

	TLS DaemonBindingTLSSpec `json:"tls"`
}

// DaemonBindingTLSSpec is the certificate the binding serves.
type DaemonBindingTLSSpec struct {
	// SecretName names a Secret holding tls.crt and tls.key, plus ca.crt when
	// clientAuth is on.
	SecretName string `json:"secretName"`

	// ClientAuth requires clients to present a certificate signed by ca.crt.
	// +optional
	ClientAuth bool `json:"clientAuth,omitempty"`
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

	// PodName is the pod the daemon's Sandbox runs, for clients that
	// port-forward to it. Empty with runtime: Deployment.
	// +optional
	PodName string `json:"podName,omitempty"`

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
// +kubebuilder:printcolumn:name="RUNTIME",type=string,JSONPath=".spec.runtime",description="The workload the daemon runs as"
// +kubebuilder:printcolumn:name="READY",type=boolean,JSONPath=".status.ready",description="Whether the Orchestrator's daemon is ready"
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
