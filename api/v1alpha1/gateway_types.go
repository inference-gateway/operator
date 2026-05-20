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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// GatewaySpec defines the desired state of Gateway.
type GatewaySpec struct {
	// Replicas is the number of gateway instances to run
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Replicas *int32 `json:"replicas,omitempty"`

	// Image is the container image to use for the gateway deployment
	// +optional
	// +kubebuilder:default="ghcr.io/inference-gateway/inference-gateway:latest"
	Image string `json:"image,omitempty"`

	// Environment defines the environment type (development, staging, production)
	// +optional
	// +kubebuilder:default="production"
	// +kubebuilder:validation:Enum=development;staging;production
	Environment string `json:"environment,omitempty"`

	// Telemetry configuration for observability
	// +optional
	Telemetry *TelemetrySpec `json:"telemetry,omitempty"`

	// Server configuration
	// +optional
	Server *ServerSpec `json:"server,omitempty"`

	// Authentication configuration
	// +optional
	Auth *AuthSpec `json:"auth,omitempty"`

	// Provider configurations for AI/ML backends
	// +optional
	Providers []ProviderSpec `json:"providers,omitempty"`

	// MCP (Model Context Protocol) configuration
	// +optional
	MCP *MCPServersSpec `json:"mcp,omitempty"`

	// Resource requirements for the gateway pods
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Service configuration
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// Routing configures north-south traffic using the Kubernetes Gateway
	// API (gateway.networking.k8s.io). Successor to the removed Ingress field.
	// +optional
	Routing *RoutingSpec `json:"routing,omitempty"`

	// HPA (Horizontal Pod Autoscaler) configuration
	// +optional
	HPA *HPASpec `json:"hpa,omitempty"`

	// ServiceAccount configuration for RBAC
	// +optional
	ServiceAccount *ServiceAccountSpec `json:"serviceAccount,omitempty"`
}

// ServiceAccountSpec contains service account configuration for RBAC
type ServiceAccountSpec struct {
	// Create specifies whether to create a service account
	// +optional
	// +kubebuilder:default=true
	Create bool `json:"create,omitempty"`

	// Name is the name of the service account to use
	// If not set and create is true, a name is generated using the gateway name
	// +optional
	Name string `json:"name,omitempty"`
}

type HPASpec struct {
	// Enable HPA for the gateway deployment
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Configures the Horizontal Pod Autoscaler for the gateway deployment
	// +optional
	Config *CustomHorizontalPodAutoscalerSpec `json:"config,omitempty"`
}

type CustomHorizontalPodAutoscalerSpec struct {
	// scaleTargetRef points to the target resource to scale, and is used to the pods for which metrics
	// should be collected, as well as to actually change the replica count.
	// +optional
	ScaleTargetRef *autoscalingv2.CrossVersionObjectReference `json:"scaleTargetRef" protobuf:"bytes,1,opt,name=scaleTargetRef"`

	// minReplicas is the lower limit for the number of replicas to which the autoscaler
	// can scale down.  It defaults to 1 pod.  minReplicas is allowed to be 0 if the
	// alpha feature gate HPAScaleToZero is enabled and at least one Object or External
	// metric is configured.  Scaling is active as long as at least one metric value is
	// available.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty" protobuf:"varint,2,opt,name=minReplicas"`

	// maxReplicas is the upper limit for the number of replicas to which the autoscaler can scale up.
	// It cannot be less that minReplicas.
	MaxReplicas int32 `json:"maxReplicas" protobuf:"varint,3,opt,name=maxReplicas"`

	// metrics contains the specifications for which to use to calculate the
	// desired replica count (the maximum replica count across all metrics will
	// be used).  The desired replica count is calculated multiplying the
	// ratio between the target value and the current value by the current
	// number of pods.  Ergo, metrics used must decrease as the pod count is
	// increased, and vice-versa.  See the individual metric source types for
	// more information about how each type of metric must respond.
	// If not set, the default metric will be set to 80% average CPU utilization.
	// +listType=atomic
	// +optional
	Metrics []autoscalingv2.MetricSpec `json:"metrics,omitempty" protobuf:"bytes,4,rep,name=metrics"`

	// behavior configures the scaling behavior of the target
	// in both Up and Down directions (scaleUp and scaleDown fields respectively).
	// If not set, the default HPAScalingRules for scale up and scale down are used.
	// +optional
	Behavior *autoscalingv2.HorizontalPodAutoscalerBehavior `json:"behavior,omitempty" protobuf:"bytes,5,opt,name=behavior"`
}

// TelemetrySpec contains telemetry and observability configuration
type TelemetrySpec struct {
	// Enable telemetry collection
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Metrics configuration
	// +optional
	// +kubebuilder:default={enabled: false, port: 9464}
	Metrics *MetricsSpec `json:"metrics,omitempty"`
}

// MetricsSpec contains metrics configuration
type MetricsSpec struct {
	// Enable metrics collection
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Port for metrics endpoint
	// +optional
	// +kubebuilder:default=9464
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// AuthSpec contains authentication configuration
type AuthSpec struct {
	// Enable authentication
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Authentication provider type
	// +optional
	// +kubebuilder:default="oidc"
	// +kubebuilder:validation:Enum=oidc;jwt;basic
	Provider string `json:"provider,omitempty"`

	// OIDC configuration
	// +optional
	OIDC *OIDCSpec `json:"oidc,omitempty"`
}

// OIDCSpec contains OIDC authentication configuration
type OIDCSpec struct {
	// +optional
	// +kubebuilder:default="http://keycloak:8080/realms/inference-gateway-realm"
	IssuerURL string `json:"issuerUrl,omitempty"`

	// +optional
	// +kubebuilder:default="inference-gateway-client"
	ClientID string `json:"clientId,omitempty"`

	// Reference to a secret containing the client secret
	// +optional
	ClientSecretRef *corev1.SecretKeySelector `json:"clientSecretRef,omitempty"`
}

// ServerSpec contains server configuration settings
type ServerSpec struct {
	// Server host
	// +optional
	// +kubebuilder:default="0.0.0.0"
	Host string `json:"host,omitempty"`

	// Server port
	// +optional
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// Server timeouts
	// +optional
	Timeouts *ServerTimeouts `json:"timeouts,omitempty"`

	// TLS configuration
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`
}

// ServerTimeouts contains server timeout configurations
type ServerTimeouts struct {
	// Read timeout
	// +optional
	// +kubebuilder:default="60s"
	Read string `json:"read,omitempty"`

	// Write timeout
	// +optional
	// +kubebuilder:default="60s"
	Write string `json:"write,omitempty"`

	// Idle timeout
	// +optional
	// +kubebuilder:default="300s"
	Idle string `json:"idle,omitempty"`
}

// TLSConfig contains TLS certificate configuration
type TLSConfig struct {
	// Enable TLS
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Reference to a secret containing the TLS certificate
	// +optional
	CertificateRef *corev1.SecretKeySelector `json:"certificateRef,omitempty"`

	// Reference to a secret containing the TLS private key
	// +optional
	KeyRef *corev1.SecretKeySelector `json:"keyRef,omitempty"`
}

// ProviderSpec contains configuration for a specific provider
type ProviderSpec struct {
	// Name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Enable provider
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Environment variables for the provider
	// +optional
	Env *[]corev1.EnvVar `json:"env,omitempty"`
}

// MCPServersSpec contains Model Context Protocol configuration
type MCPServersSpec struct {
	// Enable MCP integration
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Expose MCP endpoints externally
	// +optional
	// +kubebuilder:default=false
	Expose bool `json:"expose,omitempty"`

	// MCP client timeouts
	// +optional
	Timeouts *MCPTimeouts `json:"timeouts,omitempty"`

	// MCP servers configuration. Static entries are unioned with any servers
	// discovered via ServiceDiscovery when both are configured.
	// +optional
	Servers []MCPServer `json:"servers,omitempty"`

	// ServiceDiscovery configures automatic discovery of MCP CRs by label selector.
	// Discovered MCP URLs are appended to the static Servers list, deduped on URL,
	// sorted for determinism, and exposed to the gateway pod via the MCP_SERVERS env var.
	// +optional
	ServiceDiscovery *MCPServiceDiscoverySpec `json:"serviceDiscovery,omitempty"`
}

// MCPServiceDiscoverySpec configures automatic discovery of MCP CRs by label selector.
type MCPServiceDiscoverySpec struct {
	// Enabled toggles automatic discovery of MCP CRs.
	Enabled bool `json:"enabled"`

	// Namespace is the namespace to discover MCP CRs in.
	// Defaults to the Gateway's own namespace when empty.
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// Selector filters which MCP CRs are discovered by their labels.
	// A nil or empty selector matches all MCP CRs in the namespace.
	// Supports matchLabels and matchExpressions.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// MCPTimeouts contains timeout configurations for MCP
type MCPTimeouts struct {
	// Client timeout
	// +optional
	// +kubebuilder:default="10s"
	Client string `json:"client,omitempty"`

	// Dial timeout
	// +optional
	// +kubebuilder:default="5s"
	Dial string `json:"dial,omitempty"`

	// TLS handshake timeout
	// +optional
	// +kubebuilder:default="5s"
	TLSHandshake string `json:"tlsHandshake,omitempty"`

	// Response header timeout
	// +optional
	// +kubebuilder:default="5s"
	ResponseHeader string `json:"responseHeader,omitempty"`

	// Request timeout
	// +optional
	// +kubebuilder:default="10s"
	Request string `json:"request,omitempty"`
}

// MCPServer contains MCP server configuration
type MCPServer struct {
	// Server name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Server URL
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Health check configuration
	// +optional
	HealthCheck *HealthCheck `json:"healthCheck,omitempty"`
}

// HealthCheck contains health check configuration
type HealthCheck struct {
	// Enable health checks
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Health check path
	// +optional
	// +kubebuilder:default="/health"
	Path string `json:"path,omitempty"`

	// Health check interval
	// +optional
	// +kubebuilder:default="30s"
	Interval string `json:"interval,omitempty"`
}

// ResourceRequirements contains resource requirements for the gateway
type ResourceRequirements struct {
	// Resource requests
	// +optional
	Requests *ResourceList `json:"requests,omitempty"`

	// Resource limits
	// +optional
	Limits *ResourceList `json:"limits,omitempty"`
}

// ResourceList contains CPU and memory resource specifications
type ResourceList struct {
	// CPU resource specification
	// +optional
	CPU string `json:"cpu,omitempty"`

	// Memory resource specification
	// +optional
	Memory string `json:"memory,omitempty"`
}

// ServiceSpec contains service configuration
type ServiceSpec struct {
	// Service type
	// +optional
	// +kubebuilder:default="ClusterIP"
	// +kubebuilder:validation:Enum=ClusterIP;NodePort;LoadBalancer;ExternalName
	Type string `json:"type,omitempty"`

	// Service port
	// +optional
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// Service annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// RoutingSpec configures north-south traffic using the Kubernetes Gateway
// API. It is the v1alpha1 successor to the (removed) Ingress field.
//
// NOTE: The "Gateway" referenced under RoutingSpec is the upstream
// gateway.networking.k8s.io/v1.Gateway listener resource - NOT this CRD's
// own Gateway type.
type RoutingSpec struct {
	// Enable routing (operator-managed Gateway and/or HTTPRoute creation).
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Gateway configures the upstream Gateway API Gateway resource.
	// If ParentRefs is set, the operator does NOT create a Gateway; it
	// only creates an HTTPRoute attached to the referenced parents.
	// +optional
	Gateway *RoutingGatewaySpec `json:"gateway,omitempty"`

	// HTTPRoute configures the HTTPRoute resource the operator creates
	// to bind incoming traffic to the gateway Service.
	// +optional
	HTTPRoute *RoutingHTTPRouteSpec `json:"httpRoute,omitempty"`
}

// RoutingGatewaySpec configures the upstream Gateway API Gateway resource
// (gateway.networking.k8s.io/v1.Gateway), not this CRD's Gateway type.
type RoutingGatewaySpec struct {
	// GatewayClassName selects the GatewayClass implementing the Gateway.
	// Ignored when ParentRefs is set.
	// +optional
	// +kubebuilder:default="envoy"
	GatewayClassName string `json:"gatewayClassName,omitempty"`

	// ParentRefs, when non-empty, switches the operator into advanced
	// mode: it does NOT create a Gateway and the HTTPRoute attaches to
	// these parents (typically a platform-team-managed shared Gateway).
	// +optional
	ParentRefs []gwapiv1.ParentReference `json:"parentRefs,omitempty"`

	// TLS terminates at the Gateway listener, not the HTTPRoute.
	// No effect in advanced mode (the shared Gateway owner controls TLS).
	// +optional
	TLS *RoutingTLSSpec `json:"tls,omitempty"`
}

// RoutingTLSSpec configures listener-level TLS termination.
type RoutingTLSSpec struct {
	// Enable TLS on the default listener.
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Issuer is a cert-manager ClusterIssuer name. When set, the
	// cert-manager.io/cluster-issuer annotation is added to the Gateway.
	// Examples: "letsencrypt-prod", "letsencrypt-staging", "selfsigned-issuer"
	// +optional
	Issuer string `json:"issuer,omitempty"`

	// SecretName is the Secret referenced by the listener certificateRefs.
	// Auto-generated as "<gateway-name>-tls" if empty.
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// RoutingHTTPRouteSpec configures the HTTPRoute the operator owns.
type RoutingHTTPRouteSpec struct {
	// Hostnames the HTTPRoute matches. In default mode they also become
	// the hostnames of the operator-managed Gateway listener.
	// +optional
	Hostnames []gwapiv1.Hostname `json:"hostnames,omitempty"`
}

// GatewayStatus defines the observed state of Gateway.
type GatewayStatus struct {
	// Current number of ready replicas
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Current number of available replicas
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// Current deployment conditions
	// +optional
	Conditions []GatewayCondition `json:"conditions,omitempty"`

	// Phase represents the current phase of the Gateway
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Failed;Unknown
	Phase string `json:"phase,omitempty"`

	// Message provides additional information about the current state
	// +optional
	Message string `json:"message,omitempty"`

	// ObservedGeneration is the most recent generation observed
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// ProviderSummary is a comma-separated list of provider names
	// +optional
	ProviderSummary string `json:"providerSummary,omitempty"`

	// URL presents the address the gateway can be accessed at
	// If ingress is enabled, it will use the host from the ingress configuration
	// otherwise it will use the service URL
	// +optional
	URL string `json:"url,omitempty"`

	// ServiceAccountName is the name of the created service account
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// MCPServers is the sorted list of MCP server URLs (static + discovered)
	// that the gateway pod is configured with. Mirrors the value of MCP_SERVERS
	// passed to the container.
	// +optional
	MCPServers []string `json:"mcpServers,omitempty"`

	// MCPServerCount is the number of MCP servers (static + discovered) currently
	// wired into the gateway pod. Not omitempty so the column always renders as
	// a number (0 when MCP is disabled or no servers configured).
	MCPServerCount int32 `json:"mcpServerCount"`
}

// GatewayCondition represents a condition of a Gateway deployment
type GatewayCondition struct {
	// Type of Gateway condition
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Available;Progressing;ReplicaFailure
	Type string `json:"type"`

	// Status of the condition (True, False, Unknown)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=True;False;Unknown
	Status string `json:"status"`

	// Last time the condition transitioned
	// +optional
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`

	// Reason for the condition's last transition
	// +optional
	Reason string `json:"reason,omitempty"`

	// Human readable message indicating details about last transition
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=".status.url",description="Kubernetes service DNS address"
// +kubebuilder:printcolumn:name="Port",type=integer,JSONPath=".spec.server.port",description="Gateway port"
// +kubebuilder:printcolumn:name="Providers",type=string,JSONPath=".status.providerSummary",description="Configured providers"
// +kubebuilder:printcolumn:name="MCPS",type=integer,JSONPath=".status.mcpServerCount",description="Number of MCP servers connected (static + discovered)"
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp",description="Age of the resource"
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Gateway is the Schema for the gateways API.
type Gateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GatewaySpec   `json:"spec,omitempty"`
	Status GatewayStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GatewayList contains a list of Gateway.
type GatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Gateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Gateway{}, &GatewayList{})
}
