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
// StateRunning means the infrastructure is up, not that the HTTP inference endpoint is
// ready. The controller confirms readiness itself.
type AllocationState string

const (
	StatePending    AllocationState = "Pending"
	StateRunning    AllocationState = "Running"
	StateFailed     AllocationState = "Failed"
	StateTerminated AllocationState = "Terminated"
)

// ProvisionRequest describes a runtime to lease. UID/Namespace/Name identify the owning
// Kubernetes object, and UID is the idempotency key that lets reconciliation recover an
// existing allocation. EndpointToken is injected as API_KEY to protect the inference
// endpoint without exposing the provider management API key.
type ProvisionRequest struct {
	UID       string
	Namespace string
	Name      string

	Image    string
	Command  []string
	GPUTypes []string
	Port     int32

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
