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
)

// A2AServerSpec defines the desired state of A2AServer.
type A2AServerSpec struct {
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
	Card         CardSpec      `json:"card"`
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
	Model         string       `json:"model"`
	MaxTokens     int32        `json:"maxTokens"`
	Temperature   string       `json:"temperature"`
	CustomHeaders []HeaderSpec `json:"customHeaders"`
	SystemPrompt  string       `json:"systemPrompt"`
}

type HeaderSpec struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type CardSpec struct {
	Name               string           `json:"name"`
	Description        string           `json:"description"`
	URL                string           `json:"url"`
	DefaultInputModes  []string         `json:"defaultInputModes"`
	DefaultOutputModes []string         `json:"defaultOutputModes"`
	DocumentationURL   string           `json:"documentationUrl"`
	Capabilities       CapabilitiesSpec `json:"capabilities"`
	Skills             []string         `json:"skills"`
}

type CapabilitiesSpec struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

// A2AServerStatus defines the observed state of A2AServer.
type A2AServerStatus struct {
	// ObservedGeneration is the most recent generation observed for this resource.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Conditions represent the latest available observations of the resource's state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// Ready indicates if the resource is ready.
	Ready bool `json:"ready,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// A2AServer is the Schema for the a2aservers API.
type A2AServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   A2AServerSpec   `json:"spec,omitempty"`
	Status A2AServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// A2AServerList contains a list of A2AServer.
type A2AServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []A2AServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&A2AServer{}, &A2AServerList{})
}
