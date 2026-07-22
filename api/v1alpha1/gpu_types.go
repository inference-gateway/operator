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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GPUSpec defines the desired state of GPU.
//
// A GPU resource represents one externally managed, GPU-backed inference-runtime
// allocation reachable over HTTP. The Kubernetes cluster is the control plane
// only: GPU hardware, drivers, scheduling and inference execution stay with an
// external infrastructure provider (e.g. RunPod). It is NOT a worker node, a
// device, an `nvidia.com/gpu` request, or a replacement for the device plugin/DRA.
type GPUSpec struct {
	// Provider selects the GPU infrastructure provider driver.
	// +kubebuilder:validation:Enum=runpod
	// +kubebuilder:default=runpod
	Provider string `json:"provider"`

	// CredentialsRef references a Secret key holding the provider management API
	// key. The credential is read at reconcile time and is never written into
	// status, events, or the connection Secret.
	CredentialsRef corev1.SecretKeySelector `json:"credentialsRef"`

	// Image is the container image the provider runs on the GPU instance.
	Image string `json:"image"`

	// Command overrides the container command/args passed to the runtime.
	// +optional
	Command []string `json:"command,omitempty"`

	// GPUTypes is an ordered preference list of provider GPU type identifiers
	// (e.g. "NVIDIA RTX A6000"). The provider selects the first available.
	// +kubebuilder:validation:MinItems=1
	GPUTypes []string `json:"gpuTypes"`

	// Endpoint configures the HTTP port and readiness path of the runtime.
	Endpoint GPUEndpointSpec `json:"endpoint"`

	// MaxRuntime is the maximum lifetime of the allocation. When it elapses the
	// allocation is released and the resource transitions to Expired. Required.
	MaxRuntime metav1.Duration `json:"maxRuntime"`
}

// GPUEndpointSpec configures the runtime's HTTP endpoint.
type GPUEndpointSpec struct {
	// Port is the HTTP port the runtime listens on inside the instance.
	// +kubebuilder:default=8080
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	Port int32 `json:"port,omitempty"`

	// ReadinessPath is the HTTP path polled to confirm inference readiness.
	// Ready=True is only reported once this path responds successfully.
	// +kubebuilder:default="/"
	// +kubebuilder:validation:Pattern=`^/.*`
	// +optional
	ReadinessPath string `json:"readinessPath,omitempty"`
}

// GPUPhase is a high-level summary of where the allocation is in its lifecycle.
// +kubebuilder:validation:Enum=Pending;Provisioning;Starting;Ready;Terminating;Expired;Failed
type GPUPhase string

const (
	// GPUPhasePending is the initial phase before provisioning starts.
	GPUPhasePending GPUPhase = "Pending"
	// GPUPhaseProvisioning means the provider allocation has been requested.
	GPUPhaseProvisioning GPUPhase = "Provisioning"
	// GPUPhaseStarting means infrastructure is up but the HTTP endpoint is not ready.
	GPUPhaseStarting GPUPhase = "Starting"
	// GPUPhaseReady means the HTTP readiness endpoint responded successfully.
	GPUPhaseReady GPUPhase = "Ready"
	// GPUPhaseTerminating means the allocation is being released.
	GPUPhaseTerminating GPUPhase = "Terminating"
	// GPUPhaseExpired means MaxRuntime elapsed and the allocation was released.
	// An Expired resource is not reprovisioned without an explicit spec change.
	GPUPhaseExpired GPUPhase = "Expired"
	// GPUPhaseFailed means provisioning or readiness failed.
	GPUPhaseFailed GPUPhase = "Failed"
)

// GPUStatus defines the observed state of GPU.
type GPUStatus struct {
	// Phase is a high-level summary of the allocation lifecycle.
	// +optional
	Phase GPUPhase `json:"phase,omitempty"`

	// InstanceID is the provider's allocation identifier.
	// +optional
	InstanceID string `json:"instanceID,omitempty"`

	// URL is the resolved HTTP endpoint of the inference runtime.
	// +optional
	URL string `json:"url,omitempty"`

	// ConnectionSecretRef references the Secret holding the runtime URL and a
	// per-allocation credential (never the provider API key).
	// +optional
	ConnectionSecretRef *corev1.LocalObjectReference `json:"connectionSecretRef,omitempty"`

	// StartedAt is when the allocation was requested.
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`

	// ExpiresAt is StartedAt + MaxRuntime, after which the allocation is released.
	// +optional
	ExpiresAt *metav1.Time `json:"expiresAt,omitempty"`

	// ObservedGeneration is the most recent generation observed for this resource.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the resource's state.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=gpu,categories=inference-gateway
// +kubebuilder:printcolumn:name="PHASE",type=string,JSONPath=".status.phase",description="Lifecycle phase"
// +kubebuilder:printcolumn:name="URL",type=string,JSONPath=".status.url",description="Inference endpoint URL"
// +kubebuilder:printcolumn:name="INSTANCE",type=string,JSONPath=".status.instanceID",description="Provider allocation ID"
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=".metadata.creationTimestamp",description="Age of the resource"

// GPU is the Schema for the gpus API.
type GPU struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GPUSpec   `json:"spec,omitempty"`
	Status GPUStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GPUList contains a list of GPU.
type GPUList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GPU `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GPU{}, &GPUList{})
}
