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

// Package gpu defines the pluggable provider interface used by the GPU
// controller to lease externally hosted, GPU-backed inference runtimes, plus
// the concrete provider drivers (currently RunPod).
package gpu

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotFound is returned by Provider.Get and Provider.Destroy when the external
// allocation no longer exists. Callers treat it as successful cleanup.
var ErrNotFound = errors.New("gpu: allocation not found")

// AllocationState is the provider-reported lifecycle state of an allocation.
type AllocationState string

const (
	// StatePending means the allocation is reserved but not yet running.
	StatePending AllocationState = "Pending"
	// StateRunning means the underlying infrastructure is up. It does NOT imply
	// the HTTP inference endpoint is ready; the controller confirms that itself.
	StateRunning AllocationState = "Running"
	// StateFailed means the provider reported a terminal failure.
	StateFailed AllocationState = "Failed"
	// StateTerminated means the allocation has been released.
	StateTerminated AllocationState = "Terminated"
)

// ProvisionRequest describes a runtime to lease. UID/Namespace/Name identify the
// owning Kubernetes object; UID is the idempotency key providers tag external
// resources with so reconciliation recovers an existing allocation instead of
// creating a duplicate.
type ProvisionRequest struct {
	UID       string
	Namespace string
	Name      string

	Image    string
	Command  []string
	GPUTypes []string
	Port     int32

	// EndpointToken is a per-allocation credential injected into the runtime
	// (as the API_KEY env var) so the inference endpoint is protected without
	// exposing the provider management API key.
	EndpointToken string
}

// Allocation is the provider's view of a leased runtime.
type Allocation struct {
	ID    string
	URL   string
	State AllocationState
}

// Provider is the pluggable interface every GPU infrastructure driver implements.
// Implementations must be idempotent: Provision recovers an allocation already
// tagged with the request UID, and Destroy treats an absent allocation as success.
type Provider interface {
	Provision(ctx context.Context, req ProvisionRequest) (Allocation, error)
	Get(ctx context.Context, allocationID string) (Allocation, error)
	Destroy(ctx context.Context, allocationID string) error
}

// NewProvider returns the driver for the named provider, authenticated with the
// given management API key.
func NewProvider(name, apiKey string) (Provider, error) {
	switch name {
	case "runpod":
		return NewRunPodProvider(apiKey, ""), nil
	default:
		return nil, fmt.Errorf("gpu: unknown provider %q", name)
	}
}
