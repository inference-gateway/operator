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

// MCPSpec defines the desired state of MCP.
type MCPSpec struct {
	// Image is the container image to use for the MCP server.
	// +kubebuilder:Required
	Image string `json:"image,omitempty"`

	// Server defines the configuration for the MCP server.
	// +optional
	Server *MCPServerSpec `json:"server,omitempty"`

	// Bridge is an optional side-car container that allows to use an MCP server that natively supports only STDIO as a streamable http webserver.
	Bridge *MCPBridgeSpec `json:"bridge,omitempty"`

	// TLS defines the TLS configuration for the MCP server.
	// +optional
	TLS *TLSConfig `json:"tls,omitempty"`

	// Ingress defines the ingress configuration for the MCP server.
	// +optional
	Ingress *IngressSpec `json:"ingress,omitempty"`
}

type MCPBridgeSpec struct {
	// Create indicates whether to create a bridge sidecar container.
	// +kubebuilder:default=true
	// +optional
	Create bool `json:"create,omitempty"`

	// Port is the port on which the bridge listens.
	// +kubebuilder:default=8081
	// +optional
	Port int32 `json:"port,omitempty"`
}

type MCPServerSpec struct {
	// Port is the port on which the MCP server listens.
	// +kubebuilder:default=8080
	// +optional
	Port int32 `json:"port,omitempty"`

	// Timeout is the timeout for the MCP server.
	// +kubebuilder:default="30s"
	// +optional
	Timeout string `json:"timeout,omitempty"`
}

// MCPStatus defines the observed state of MCP.
type MCPStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// MCP is the Schema for the mcps API.
type MCP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MCPSpec   `json:"spec,omitempty"`
	Status MCPStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// MCPList contains a list of MCP.
type MCPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MCP `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MCP{}, &MCPList{})
}
