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

	corev1 "k8s.io/api/core/v1"
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
	MCP *MCPSpec `json:"mcp,omitempty"`

	// A2A (Agent-to-Agent) configuration
	// +optional
	A2A *A2ASpec `json:"a2a,omitempty"`

	// Resource requirements for the gateway pods
	// +optional
	Resources *ResourceRequirements `json:"resources,omitempty"`

	// Service configuration
	// +optional
	Service *ServiceSpec `json:"service,omitempty"`

	// Ingress configuration
	// +optional
	Ingress *IngressSpec `json:"ingress,omitempty"`

	// HPA (Horizontal Pod Autoscaler) configuration
	// +optional
	HPA *HPASpec `json:"hpa,omitempty"`
}

// TelemetrySpec contains telemetry and observability configuration
type TelemetrySpec struct {
	// Enable telemetry collection
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Metrics configuration
	// +optional
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

// TracingSpec contains tracing configuration
type TracingSpec struct {
	// Enable tracing
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Tracing endpoint URL
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
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
	ClientSecretRef *SecretKeySelector `json:"clientSecretRef,omitempty"`
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
	CertificateRef *SecretKeySelector `json:"certificateRef,omitempty"`

	// Reference to a secret containing the TLS private key
	// +optional
	KeyRef *SecretKeySelector `json:"keyRef,omitempty"`
}

// SecretKeySelector selects a key from a Secret
type SecretKeySelector struct {
	// Name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Key within the secret
	// +optional
	Key string `json:"key,omitempty"`
}

// ProviderSpec contains configuration for a specific provider
type ProviderSpec struct {
	// Name of the secret
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Environment variables for the provider
	// +optional
	Env *[]corev1.EnvVar `json:"env,omitempty"`
}

// ProviderConfig contains provider-specific configuration
type ProviderConfig struct {
	// Base URL for the provider API
	// +optional
	BaseURL string `json:"baseUrl,omitempty"`

	// Authentication type
	// +optional
	// +kubebuilder:validation:Enum=bearer;basic;none
	AuthType string `json:"authType,omitempty"`

	// Reference to secret containing authentication token
	// +optional
	TokenRef *SecretKeySelector `json:"tokenRef,omitempty"`
}

// MCPSpec contains Model Context Protocol configuration
type MCPSpec struct {
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

	// MCP servers configuration
	// +optional
	Servers []MCPServer `json:"servers,omitempty"`
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

// A2ASpec contains Agent-to-Agent configuration
type A2ASpec struct {
	// Enable A2A integration
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Expose A2A endpoints externally
	// +optional
	// +kubebuilder:default=false
	Expose bool `json:"expose,omitempty"`

	// A2A client timeouts
	// +optional
	Timeouts *A2ATimeouts `json:"timeouts,omitempty"`

	// Polling configuration
	// +optional
	Polling *A2APolling `json:"polling,omitempty"`

	// A2A agents configuration
	// +optional
	Agents []A2AAgent `json:"agents,omitempty"`
}

// A2ATimeouts contains timeout configurations for A2A
type A2ATimeouts struct {
	// Client timeout
	// +optional
	// +kubebuilder:default="60s"
	Client string `json:"client,omitempty"`
}

// A2APolling contains polling configuration for A2A
type A2APolling struct {
	// Enable polling
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Polling interval
	// +optional
	// +kubebuilder:default="2s"
	Interval string `json:"interval,omitempty"`

	// Polling timeout
	// +optional
	// +kubebuilder:default="60s"
	Timeout string `json:"timeout,omitempty"`

	// Maximum polling attempts
	// +optional
	// +kubebuilder:default=30
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	MaxAttempts int32 `json:"maxAttempts,omitempty"`
}

// A2AAgent contains A2A agent configuration
type A2AAgent struct {
	// Agent name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Agent URL
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

// IngressSpec contains ingress configuration
type IngressSpec struct {
	// Enable ingress
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Simple host configuration (alternative to hosts array)
	// When specified, this will be used as the primary host with automatic TLS and path configuration
	// +optional
	Host string `json:"host,omitempty"`

	// Ingress class name
	// +optional
	ClassName string `json:"className,omitempty"`

	// Ingress annotations
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// Ingress hosts configuration (advanced usage)
	// Use 'host' field for simple single-host configuration
	// +optional
	Hosts []IngressHost `json:"hosts,omitempty"`

	// TLS configuration
	// +optional
	TLS *IngressTLSConfig `json:"tls,omitempty"`
}

// IngressTLSConfig contains simplified TLS configuration
type IngressTLSConfig struct {
	// Enable TLS for ingress
	// +optional
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Certificate issuer for cert-manager (automatically sets annotation)
	// Examples: "letsencrypt-prod", "letsencrypt-staging", "selfsigned-issuer"
	// +optional
	Issuer string `json:"issuer,omitempty"`

	// Secret name for TLS certificate (auto-generated if not specified)
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// Advanced TLS configuration (alternative to simple config above)
	// +optional
	Config []IngressTLS `json:"config,omitempty"`
}

// IngressHost contains ingress host configuration
type IngressHost struct {
	// Host name
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// Paths configuration
	// +optional
	Paths []IngressPath `json:"paths,omitempty"`
}

// IngressPath contains ingress path configuration
type IngressPath struct {
	// Path
	// +kubebuilder:validation:Required
	Path string `json:"path"`

	// Path type
	// +optional
	// +kubebuilder:default="Prefix"
	// +kubebuilder:validation:Enum=Exact;Prefix;ImplementationSpecific
	PathType string `json:"pathType,omitempty"`
}

// IngressTLS contains ingress TLS configuration
type IngressTLS struct {
	// Secret name containing TLS certificate
	// +kubebuilder:validation:Required
	SecretName string `json:"secretName"`

	// Hosts covered by the certificate
	// +optional
	Hosts []string `json:"hosts,omitempty"`
}

// HPASpec contains Horizontal Pod Autoscaler configuration
type HPASpec struct {
	// Enable HPA
	// +optional
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Minimum number of replicas
	// +optional
	// +kubebuilder:default=1
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// Maximum number of replicas
	// +optional
	// +kubebuilder:default=10
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	MaxReplicas int32 `json:"maxReplicas,omitempty"`

	// Target CPU utilization percentage
	// +optional
	// +kubebuilder:default=80
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// Target memory utilization percentage
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`

	// Custom metrics for scaling
	// +optional
	CustomMetrics []HPACustomMetric `json:"customMetrics,omitempty"`

	// Scale down stabilization window (in seconds)
	// +optional
	// +kubebuilder:default=300
	ScaleDownStabilizationWindowSeconds *int32 `json:"scaleDownStabilizationWindowSeconds,omitempty"`

	// Scale up stabilization window (in seconds)
	// +optional
	// +kubebuilder:default=0
	ScaleUpStabilizationWindowSeconds *int32 `json:"scaleUpStabilizationWindowSeconds,omitempty"`
}

// HPACustomMetric contains custom metric configuration for HPA
type HPACustomMetric struct {
	// Metric name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Metric type (Resource, Pods, Object, External)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Resource;Pods;Object;External
	Type string `json:"type"`

	// Target value for the metric
	// +kubebuilder:validation:Required
	Target HPAMetricTarget `json:"target"`
}

// HPAMetricTarget contains target configuration for HPA custom metrics
type HPAMetricTarget struct {
	// Target type (Utilization, Value, AverageValue)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=Utilization;Value;AverageValue
	Type string `json:"type"`

	// Target value (for Value and AverageValue types)
	// +optional
	Value string `json:"value,omitempty"`

	// Target average utilization percentage (for Utilization type)
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	AverageUtilization *int32 `json:"averageUtilization,omitempty"`
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

// +kubebuilder:printcolumn:name="IP",type=string,JSONPath=".spec.server.host",description="Gateway IP address"
// +kubebuilder:printcolumn:name="Port",type=integer,JSONPath=".spec.server.port",description="Gateway port"
// +kubebuilder:printcolumn:name="Providers",type=string,JSONPath=".status.providerSummary",description="Configured providers"
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
