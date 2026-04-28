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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BotSpec defines the desired state of Bot.
//
// A Bot deploys the Inference Gateway CLI's `channels-manager` daemon as a
// Telegram bot. The Deployment is always a singleton (replicas=1, strategy=Recreate)
// because Telegram allows only one active getUpdates consumer per token.
type BotSpec struct {
	Image    string         `json:"image"`
	Channels ChannelsSpec   `json:"channels"`
	Gateway  BotGatewaySpec `json:"gateway"`
	Agent    BotAgentSpec   `json:"agent"`

	// +optional
	Tools BotToolsSpec `json:"tools,omitempty"`

	// +optional
	A2A BotA2ASpec `json:"a2a,omitempty"`

	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// Env is an optional passthrough for additional environment variables.
	// +optional
	Env *[]corev1.EnvVar `json:"env,omitempty"`
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

// BotGatewaySpec configures how the bot reaches the Inference Gateway.
type BotGatewaySpec struct {
	URL string `json:"url"`

	// +optional
	APIKeySecretRef *corev1.SecretKeySelector `json:"apiKeySecretRef,omitempty"`
}

// BotAgentSpec configures the LLM agent used by the bot.
type BotAgentSpec struct {
	Model string `json:"model"`

	// +optional
	SystemPrompt string `json:"systemPrompt,omitempty"`
}

// BotToolsSpec toggles built-in CLI tools.
type BotToolsSpec struct {
	Enabled bool `json:"enabled"`

	// +optional
	Schedule bool `json:"schedule,omitempty"`
}

// BotA2ASpec configures Agent-to-Agent integration.
type BotA2ASpec struct {
	Enabled bool `json:"enabled"`

	// +optional
	Agents []string `json:"agents,omitempty"`
}

// BotStatus defines the observed state of Bot.
type BotStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	Ready bool `json:"ready,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=bot,categories=inference-gateway
// +kubebuilder:printcolumn:name="READY",type=boolean,JSONPath=".status.ready",description="Whether the Bot Deployment is available"
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp",description="Age of the resource"

// Bot is the Schema for the bots API.
type Bot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BotSpec   `json:"spec,omitempty"`
	Status BotStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// BotList contains a list of Bot.
type BotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bot `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Bot{}, &BotList{})
}
