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

// A2ASpec defines the desired state of A2A.
type A2ASpec struct {
	Image        string        `json:"image"`
	Timezone     string        `json:"timezone"`
	Port         int32         `json:"port"`
	Host         string        `json:"host"`
	ReadTimeout  string        `json:"readTimeout"`
	WriteTimeout string        `json:"writeTimeout"`
	IdleTimeout  string        `json:"idleTimeout"`
	Logging      LoggingSpec   `json:"logging"`
	Telemetry    TelemetrySpec `json:"telemetry"`
	Queue        QueueSpec     `json:"queue"`
	TLS          TLSSpec       `json:"tls"`
	Agent        AgentSpec     `json:"agent"`

	// Environment variables for the provider
	// +optional
	Env *[]corev1.EnvVar `json:"env,omitempty"`
}

type LoggingSpec struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

type QueueSpec struct {
	Enabled         bool   `json:"enabled"`
	MaxSize         int32  `json:"maxSize"`
	CleanupInterval string `json:"cleanupInterval"`
}

type TLSSpec struct {
	Enabled   bool   `json:"enabled"`
	SecretRef string `json:"secretRef"`
}

type AgentSpec struct {
	Enabled                     bool       `json:"enabled"`
	TLS                         TLSSpec    `json:"tls"`
	MaxConversationHistory      int32      `json:"maxConversationHistory"`
	MaxChatCompletionIterations int32      `json:"maxChatCompletionIterations"`
	MaxRetries                  int32      `json:"maxRetries"`
	APIKey                      APIKeySpec `json:"apiKey"`
	LLM                         LLMSpec    `json:"llm"`
}

type APIKeySpec struct {
	SecretRef string `json:"secretRef"`
}

type LLMSpec struct {
	Model         string        `json:"model"`
	MaxTokens     *int32        `json:"maxTokens,omitempty"`
	Temperature   *string       `json:"temperature,omitempty"`
	CustomHeaders *[]HeaderSpec `json:"customHeaders,omitempty"`
	SystemPrompt  string        `json:"systemPrompt"`
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

	// DocumentationURL is an optional field that provides a URL to the documentation for the A2A.
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

// A2AStatus defines the observed state of A2A.
type A2AStatus struct {
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

	// Card indicates the version of the A2A resource.
	// +optional
	Card Card `json:"card,omitempty"`
}

// +kubebuilder:printcolumn:name="VERSION",type=string,JSONPath=".status.card.version",description="Version of the A2A resource"
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=".status.card.url",description="URL of the A2A resource"
// +kubebuilder:printcolumn:name="STREAMING",type=string,JSONPath=".status.card.capabilities.streaming",description="Streaming Capability of the A2A resource"
// +kubebuilder:printcolumn:name="PUSH NOTIFICATIONS",type=string,JSONPath=".status.card.capabilities.pushNotifications",description="Push Notifications Capability of the A2A resource"
// +kubebuilder:printcolumn:name="STATE TRANSITION HISTORY",type=string,JSONPath=".status.card.capabilities.stateTransitionHistory",description="State Transition History Capability of the A2A resource"
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp",description="Age of the resource"
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// A2A is the Schema for the a2as API.
type A2A struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   A2ASpec   `json:"spec,omitempty"`
	Status A2AStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// A2AList contains a list of A2A.
type A2AList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []A2A `json:"items"`
}

func init() {
	SchemeBuilder.Register(&A2A{}, &A2AList{})
}
