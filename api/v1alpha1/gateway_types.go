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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GatewaySpec defines the desired state of Gateway.
type GatewaySpec struct {
	// General settings
	// +optional
	// +kubebuilder:default=production
	Environment string `json:"environment,omitempty"`

	// +optional
	// +kubebuilder:default=false
	EnableTelemetry bool `json:"enableTelemetry,omitempty"`

	// +optional
	// +kubebuilder:default=false
	EnableAuth bool `json:"enableAuth,omitempty"`

	// OIDC configuration
	// +optional
	OIDC *OIDCSpec `json:"oidc,omitempty"`

	// Server configuration
	// +optional
	Server *ServerSpec `json:"server,omitempty"`

	// Provider configurations
	// +optional
	Providers map[string]*ProviderSpec `json:"providers,omitempty"`
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
	// +optional
	// +kubebuilder:default="0.0.0.0"
	Host string `json:"host,omitempty"`

	// +optional
	// +kubebuilder:default="8080"
	Port string `json:"port,omitempty"`

	// +optional
	// +kubebuilder:default="30s"
	ReadTimeout string `json:"readTimeout,omitempty"`

	// +optional
	// +kubebuilder:default="30s"
	WriteTimeout string `json:"writeTimeout,omitempty"`

	// +optional
	// +kubebuilder:default="120s"
	IdleTimeout string `json:"idleTimeout,omitempty"`

	// TLS configuration
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`
}

// TLSConfig contains TLS certificate configuration
type TLSConfig struct {
	// Reference to a secret containing the TLS certificate
	// +optional
	CertificateRef *SecretKeySelector `json:"certificateRef,omitempty"`

	// Reference to a secret containing the TLS private key
	// +optional
	KeyRef *SecretKeySelector `json:"keyRef,omitempty"`
}

// ProviderSpec contains configuration for a specific provider
type ProviderSpec struct {
	// Provider API URL
	URL string `json:"url,omitempty"`

	// Reference to a secret containing the API token/key
	// +optional
	TokenRef *SecretKeySelector `json:"tokenRef,omitempty"`
}

// SecretKeySelector selects a key from a Secret
type SecretKeySelector struct {
	// Name of the secret
	Name string `json:"name"`

	// Key within the secret
	// +optional
	// +kubebuilder:default="value"
	Key string `json:"key,omitempty"`

	// Namespace of the secret
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// GatewayStatus defines the observed state of Gateway.
type GatewayStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

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
