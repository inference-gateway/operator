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
// +kubebuilder:validation:XValidation:rule="!has(self.userData) || self.runtime == 'Sandbox'",message="spec.userData needs spec.runtime: Sandbox"
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

	// UserData is a cloud-init user-data script the computer runs as root when
	// it boots, before the daemon starts, the way a cloud VM runs it. The pod
	// gets a user namespace, so its root is never root on the node.
	// +optional
	UserData *UserDataSpec `json:"userData,omitempty"`

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

// OrchestratorConditionReady is the condition type that reports whether the
// daemon runs. Its reason is one of the Reason constants, which tells a client
// whether to keep waiting or fail fast.
const OrchestratorConditionReady = "Ready"

const (
	// ReasonDeploymentAvailable reports Ready=True for the Deployment runtime.
	ReasonDeploymentAvailable = "DeploymentAvailable"
	// ReasonDeploymentNotAvailable reports the Deployment has no available
	// replica yet. It clears on its own once the pod starts.
	ReasonDeploymentNotAvailable = "DeploymentNotAvailable"
	// ReasonSandboxReady reports Ready=True for the Sandbox runtime.
	ReasonSandboxReady = "SandboxReady"
	// ReasonSandboxNotReady reports the Sandbox's pod does not run yet. It clears
	// on its own unless the Sandbox's own reason, such as an image pull error,
	// needs a fix to the spec or the cluster.
	ReasonSandboxNotReady = "SandboxNotReady"
	// ReasonSandboxSuspended reports spec.sandbox.operatingMode is Suspended. It
	// needs a spec change to Running.
	ReasonSandboxSuspended = "SandboxSuspended"
	// ReasonInvalidSpec reports a spec that gives the daemon nothing to run. It
	// needs a spec change.
	ReasonInvalidSpec = "InvalidSpec"
	// ReasonSandboxCRDMissing reports runtime Sandbox on a cluster without
	// agent-sandbox. It needs agent-sandbox installed and the operator restarted.
	ReasonSandboxCRDMissing = "SandboxCRDMissing"
	// ReasonGatewayNotFound reports spec.gateway.ref names no readable Gateway.
	// The controller retries, so it clears once that Gateway exists.
	ReasonGatewayNotFound = "GatewayNotFound"
)

// AgentsDoneMessage is the termination message a pod daemon leaves once every
// agent on it is done, which starts spec.sandbox.retention.
const AgentsDoneMessage = "agents-done=true"

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

	// Volumes mount existing PersistentVolumeClaims into the daemon's container,
	// for an app or content its sessions share. The Sandbox never owns such a
	// claim, so it outlives a suspension, the retention and the Orchestrator.
	// +optional
	// +listType=map
	// +listMapKey=name
	// +kubebuilder:validation:MaxItems=16
	// +kubebuilder:validation:XValidation:rule="self.all(a, self.exists_one(b, b.mountPath == a.mountPath))",message="spec.sandbox.volumes must not share a mountPath"
	Volumes []SandboxVolume `json:"volumes,omitempty"`
}

// SandboxVolume mounts a PersistentVolumeClaim of the Orchestrator's namespace
// into the daemon's container. The claim is created and filled elsewhere. A
// claim shared by pods on several nodes needs ReadWriteMany or ReadOnlyMany.
// +kubebuilder:validation:XValidation:rule="!['/workspace', '/home/infer/.infer/projects', '/var/lib/cloud/seed/nocloud'].exists(p, (self.mountPath + '/').startsWith(p + '/'))",message="mountPath overlaps a path the operator mounts"
type SandboxVolume struct {
	// Name identifies the volume within the Orchestrator.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	// +kubebuilder:validation:MaxLength=48
	Name string `json:"name"`

	// ClaimName is the PersistentVolumeClaim to mount.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	ClaimName string `json:"claimName"`

	// MountPath is the absolute path the claim is mounted at.
	// +kubebuilder:validation:Pattern=`^/[^/]`
	// +kubebuilder:validation:MaxLength=256
	MountPath string `json:"mountPath"`

	// ReadOnly mounts the claim read-only, for content the daemon's agents
	// copy before they change it, so the original stays as it is.
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`
}

// UserDataSpec selects cloud-init user-data from a Secret, the shape KubeVirt's
// cloudInitNoCloud volume takes, so a VM runtime can hand it over unchanged.
type UserDataSpec struct {
	// SecretRef selects the Secret key holding the user-data, by convention
	// the key "userdata".
	SecretRef corev1.SecretKeySelector `json:"secretRef"`
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

	// Service adds metadata to the <name>-binding Service for other controllers
	// to act on, for example a VPN operator or external-dns.
	// +optional
	Service *DaemonBindingServiceSpec `json:"service,omitempty"`
}

// DaemonBindingServiceSpec is metadata copied verbatim onto the binding Service.
type DaemonBindingServiceSpec struct {
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// +optional
	Labels map[string]string `json:"labels,omitempty"`
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

// OrchestratorMCPSpec configures Model Context Protocol integration. The
// controller writes the servers, and the MCP CRs discovery finds, into the
// mcp.yaml mounted at ~/.infer/mcp.yaml in the pod. A content hash on the pod
// template rolls the pod when it changes, since Kubernetes does not propagate
// updates to a ConfigMap mounted with subPath.
type OrchestratorMCPSpec struct {
	Enabled bool `json:"enabled"`

	// Servers are MCP servers the pod reaches by URL, such as hosted ones that
	// authenticate by header or ones with no MCP CR in the cluster.
	// +optional
	// +listType=map
	// +listMapKey=name
	Servers []OrchestratorMCPServer `json:"servers,omitempty"`

	// ServiceDiscovery configures automatic discovery of MCP CRs by label selector.
	// +optional
	ServiceDiscovery OrchestratorServiceDiscoverySpec `json:"serviceDiscovery,omitempty"`
}

// OrchestratorMCPServer is one MCP server the pod reaches by URL. Its header
// values reach the pod's mcp.yaml as given, so a ${VAR} stays a reference the
// CLI expands from the pod's environment, such as a variable spec.env fills
// from a Secret. Never put a secret value in a header here.
type OrchestratorMCPServer struct {
	// Name is the server's alias, which namespaces its tools in the pod.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// URL is the server's endpoint, such as https://mcp.example.com/mcp.
	// +kubebuilder:validation:MinLength=1
	URL string `json:"url"`

	// Headers are sent with every request to the server.
	// +optional
	Headers map[string]string `json:"headers,omitempty"`

	// IncludeTools limits the pod to these tools of the server, named as the
	// server lists them. When set, ExcludeTools is ignored.
	// +optional
	IncludeTools []string `json:"includeTools,omitempty"`

	// ExcludeTools hides these tools of the server from the pod.
	// +optional
	ExcludeTools []string `json:"excludeTools,omitempty"`

	// TimeoutSeconds bounds one request to the server. Unset, the pod's default
	// applies.
	// +optional
	// +kubebuilder:validation:Minimum=1
	TimeoutSeconds int32 `json:"timeoutSeconds,omitempty"`
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
