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
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentSpec defines the desired state of Agent.
type AgentSpec struct {
	// Image is the container image for the agent.
	Image string `json:"image"`

	// Timezone for the agent process.
	// +optional
	// +kubebuilder:default="UTC"
	Timezone string `json:"timezone,omitempty"`

	// Port the agent listens on.
	// +optional
	// +kubebuilder:default=8080
	Port int32 `json:"port,omitempty"`

	// Host address the agent binds to.
	// +optional
	// +kubebuilder:default="0.0.0.0"
	Host string `json:"host,omitempty"`

	// ReadTimeout for HTTP server.
	// +optional
	// +kubebuilder:default="30s"
	ReadTimeout string `json:"readTimeout,omitempty"`

	// WriteTimeout for HTTP server.
	// +optional
	// +kubebuilder:default="30s"
	WriteTimeout string `json:"writeTimeout,omitempty"`

	// IdleTimeout for HTTP server.
	// +optional
	// +kubebuilder:default="60s"
	IdleTimeout string `json:"idleTimeout,omitempty"`

	// Logging configuration.
	// +optional
	Logging LoggingSpec `json:"logging,omitempty"`

	// Telemetry configuration.
	// +optional
	Telemetry TelemetrySpec `json:"telemetry,omitempty"`

	// Queue configuration.
	// +optional
	Queue QueueSpec `json:"queue,omitempty"`

	// TLS configuration for the agent HTTP server.
	// +optional
	TLS TLSSpec `json:"tls,omitempty"`

	// Agent-specific configuration (LLM, retries, etc.).
	// +optional
	Agent AgentConfigSpec `json:"agent,omitempty"`

	// Environment variables for the agent container.
	// +optional
	Env *[]corev1.EnvVar `json:"env,omitempty"`
}

type LoggingSpec struct {
	// Level sets the log verbosity.
	// +optional
	// +kubebuilder:default="info"
	Level string `json:"level,omitempty"`

	// Format sets the log output format.
	// +optional
	// +kubebuilder:default="json"
	Format string `json:"format,omitempty"`
}

type QueueSpec struct {
	// Enabled toggles the task queue.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// MaxSize is the maximum number of items in the queue.
	// Only meaningful when enabled is true.
	// +optional
	MaxSize int32 `json:"maxSize,omitempty"`

	// CleanupInterval is how often completed items are purged from the queue.
	// Only meaningful when enabled is true.
	// +optional
	CleanupInterval string `json:"cleanupInterval,omitempty"`
}

type TLSSpec struct {
	// Enabled toggles TLS.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// SecretRef is the name of the Secret holding TLS credentials.
	// Only required when enabled is true.
	// +optional
	SecretRef string `json:"secretRef,omitempty"`
}

type AgentConfigSpec struct {
	// Enabled toggles the agent loop.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// TLS configuration for agent-to-agent communication.
	// +optional
	TLS TLSSpec `json:"tls,omitempty"`

	// MaxConversationHistory is the number of messages retained in context.
	// +optional
	// +kubebuilder:default=10
	MaxConversationHistory int32 `json:"maxConversationHistory,omitempty"`

	// MaxChatCompletionIterations is the maximum number of LLM call iterations per request.
	// +optional
	// +kubebuilder:default=5
	MaxChatCompletionIterations int32 `json:"maxChatCompletionIterations,omitempty"`

	// MaxRetries is the maximum number of retries for failed LLM calls.
	// +optional
	// +kubebuilder:default=3
	MaxRetries int32 `json:"maxRetries,omitempty"`

	// APIKey holds a reference to the secret containing the LLM API key.
	// Deprecated: use spec.agent.llm.apiKeySecretRef instead.
	// +optional
	APIKey APIKeySpec `json:"apiKey,omitempty"`

	// LLM configuration for the agent's language model.
	// +optional
	LLM LLMSpec `json:"llm,omitempty"`
}

// APIKeySpec holds a plain-string reference to an API key secret.
// Deprecated: use corev1.SecretKeySelector via LLMSpec.APIKeySecretRef instead.
type APIKeySpec struct {
	// +optional
	SecretRef string `json:"secretRef,omitempty"`
}

type LLMSpec struct {
	// BaseURL is the base URL for the LLM API endpoint (e.g. the Inference Gateway URL).
	// Emitted as A2A_AGENT_CLIENT_BASE_URL.
	// +optional
	BaseURL string `json:"baseURL,omitempty"`

	// Model is the model identifier in "provider/model" format.
	// The provider prefix is split out and emitted as A2A_AGENT_CLIENT_PROVIDER;
	// the remainder is emitted as A2A_AGENT_CLIENT_MODEL.
	// +optional
	Model string `json:"model,omitempty"`

	// MaxTokens is the maximum number of tokens to generate.
	// Emitted as A2A_AGENT_CLIENT_MAX_TOKENS.
	// +optional
	// +kubebuilder:default=4096
	MaxTokens *int32 `json:"maxTokens,omitempty"`

	// Temperature controls the randomness of the LLM output.
	// Emitted as A2A_AGENT_CLIENT_TEMPERATURE.
	// +optional
	// +kubebuilder:default="0.7"
	Temperature *string `json:"temperature,omitempty"`

	// CustomHeaders are additional HTTP headers sent with each LLM request.
	// +optional
	CustomHeaders *[]HeaderSpec `json:"customHeaders,omitempty"`

	// SystemPrompt is the system prompt for the LLM.
	// +optional
	SystemPrompt string `json:"systemPrompt,omitempty"`

	// APIKeySecretRef references a Secret key that contains the LLM API key.
	// Emitted as A2A_AGENT_CLIENT_API_KEY via valueFrom.secretKeyRef.
	// +optional
	APIKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`
}

type HeaderSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Examples    []string `json:"examples"`
	InputModes  []string `json:"inputModes"`
	OutputModes []string `json:"outputModes"`
	Tags        []string `json:"tags"`
}

type SkillsList []Skill

func (s *SkillsList) UnmarshalJSON(data []byte) error {
	var arr []Skill
	if err := json.Unmarshal(data, &arr); err == nil {
		*s = arr
		return nil
	}

	var single Skill
	if err := json.Unmarshal(data, &single); err == nil {
		*s = []Skill{single}
		return nil
	}
	return fmt.Errorf("skills must be a Skill object or array of Skill objects")
}

// SkillsNames returns a comma-separated string of skill names.
func (s SkillsList) SkillsNames() string {
	if len(s) == 0 {
		return ""
	}
	names := make([]string, 0, len(s))
	for _, skill := range s {
		names = append(names, skill.Name)
	}
	return joinComma(names)
}

// joinComma joins a slice of strings with a comma and a space.
func joinComma(items []string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += ", "
		}
		result += item
	}
	return result
}

type Card struct {
	Name               string   `json:"name"`
	Version            string   `json:"version"`
	Description        string   `json:"description"`
	URL                string   `json:"url"`
	DefaultInputModes  []string `json:"defaultInputModes"`
	DefaultOutputModes []string `json:"defaultOutputModes"`

	// DocumentationURL is an optional field that provides a URL to the documentation for the Agent.
	// +optional
	DocumentationURL string `json:"documentationUrl,omitempty"`

	Capabilities CapabilitiesSpec `json:"capabilities"`
	Skills       SkillsList       `json:"skills"`

	// Comma separated string of skill names.
	SkillsNames string `json:"skillsNames,omitempty"`
}

type CapabilitiesSpec struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

// AgentStatus defines the observed state of Agent.
type AgentStatus struct {
	// ObservedGeneration is the most recent generation observed for this resource.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the resource's state.
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Failed;Unknown
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Ready indicates if the resource is ready.
	// +optional
	Ready bool `json:"ready,omitempty"`

	// Card indicates the version of the Agent resource.
	// +optional
	Card Card `json:"card,omitempty"`
}

// +kubebuilder:printcolumn:name="VERSION",type=string,JSONPath=".status.card.version",description="Version of the Agent resource"
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=".status.card.url",description="URL of the Agent resource"
// +kubebuilder:printcolumn:name="STREAMING",type=string,JSONPath=".status.card.capabilities.streaming",description="Streaming Capability of the Agent resource"
// +kubebuilder:printcolumn:name="PUSH NOTIFICATIONS",type=string,JSONPath=".status.card.capabilities.pushNotifications",description="Push Notifications Capability of the Agent resource"
// +kubebuilder:printcolumn:name="STATE TRANSITION HISTORY",type=string,JSONPath=".status.card.capabilities.stateTransitionHistory",description="State Transition History Capability of the Agent resource"
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp",description="Age of the resource"
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// Agent is the Schema for the agents API.
type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentSpec   `json:"spec,omitempty"`
	Status AgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentList contains a list of Agent.
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Agent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Agent{}, &AgentList{})
}
