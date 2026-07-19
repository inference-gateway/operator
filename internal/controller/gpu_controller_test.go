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

package controller

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
	gpu "github.com/inference-gateway/operator/internal/gpu"
)

// fakeGPUProvider is an in-memory Provider that models idempotent-by-UID
// behavior: repeated Provision calls for the same UID return the same single
// allocation, so crash-recovery reconciliation cannot create a duplicate.
type fakeGPUProvider struct {
	mu             sync.Mutex
	allocs         map[string]gpu.Allocation // uid -> allocation
	ids            map[string]string         // id -> uid
	provisionCount int
	destroyed      []string
	state          gpu.AllocationState
	provisionErr   error
	getErr         error
	destroyErr     error
}

func newFakeGPUProvider() *fakeGPUProvider {
	return &fakeGPUProvider{
		allocs: map[string]gpu.Allocation{},
		ids:    map[string]string{},
		state:  gpu.StateRunning,
	}
}

func (f *fakeGPUProvider) Provision(_ context.Context, req gpu.ProvisionRequest) (gpu.Allocation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.provisionCount++
	if f.provisionErr != nil {
		return gpu.Allocation{}, f.provisionErr
	}
	if a, ok := f.allocs[req.UID]; ok {
		return a, nil
	}
	a := gpu.Allocation{ID: "inst-" + req.UID, URL: "http://fake-" + req.UID + ".local", State: f.state}
	f.allocs[req.UID] = a
	f.ids[a.ID] = req.UID
	return a, nil
}

func (f *fakeGPUProvider) Get(_ context.Context, id string) (gpu.Allocation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return gpu.Allocation{}, f.getErr
	}
	uid, ok := f.ids[id]
	if !ok {
		return gpu.Allocation{}, gpu.ErrNotFound
	}
	a := f.allocs[uid]
	a.State = f.state
	return a, nil
}

func (f *fakeGPUProvider) Destroy(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.destroyErr != nil {
		return f.destroyErr
	}
	f.destroyed = append(f.destroyed, id)
	if uid, ok := f.ids[id]; ok {
		delete(f.allocs, uid)
		delete(f.ids, id)
	}
	return nil
}

func (f *fakeGPUProvider) allocCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.allocs)
}

var _ = Describe("GPU Controller", func() {
	const credName = "runpod-creds"

	var (
		fake           *fakeGPUProvider
		readyErr       error
		capturedAPIKey string
		clock          time.Time
		reconciler     *GPUReconciler
	)

	newGPU := func(name string) *v1alpha1.GPU {
		return &v1alpha1.GPU{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: v1alpha1.GPUSpec{
				Provider:       "runpod",
				CredentialsRef: corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: credName}, Key: "api-key"},
				Image:          "img",
				GPUTypes:       []string{"NVIDIA RTX A6000"},
				Endpoint:       v1alpha1.GPUEndpointSpec{Port: 8080, ReadinessPath: "/v1/models"},
				MaxRuntime:     metav1.Duration{Duration: time.Hour},
			},
		}
	}

	reconcileOnce := func(nn types.NamespacedName) reconcile.Result {
		res, err := reconciler.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		return res
	}

	reconcileUntil := func(nn types.NamespacedName, phase v1alpha1.GPUPhase) {
		Eventually(func() v1alpha1.GPUPhase {
			reconcileOnce(nn)
			g := &v1alpha1.GPU{}
			Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
			return g.Status.Phase
		}, "5s", "10ms").Should(Equal(phase))
	}

	BeforeEach(func() {
		fake = newFakeGPUProvider()
		readyErr = nil
		capturedAPIKey = ""
		clock = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		reconciler = &GPUReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			NewProvider: func(_, apiKey string) (gpu.Provider, error) {
				capturedAPIKey = apiKey
				return fake, nil
			},
			CheckReady: func(_ context.Context, _, _ string) error { return readyErr },
			Now:        func() time.Time { return clock },
		}
		_ = k8sClient.Create(context.Background(), &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: credName, Namespace: "default"},
			StringData: map[string]string{"api-key": "provider-secret"},
		})
	})

	It("reaches Ready only after the HTTP endpoint responds, and publishes a connection Secret", func() {
		nn := types.NamespacedName{Name: "gpu-lifecycle", Namespace: "default"}
		Expect(k8sClient.Create(context.Background(), newGPU(nn.Name))).To(Succeed())

		readyErr = context.DeadlineExceeded
		reconcileUntil(nn, v1alpha1.GPUPhaseStarting)

		g := &v1alpha1.GPU{}
		Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
		Expect(g.Status.InstanceID).To(Equal("inst-" + string(g.UID)))
		Expect(g.Status.ExpiresAt).NotTo(BeNil())
		Expect(readyConditionTrue(g)).To(BeFalse())
		Expect(capturedAPIKey).To(Equal("provider-secret"))

		readyErr = nil
		reconcileUntil(nn, v1alpha1.GPUPhaseReady)

		Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
		Expect(readyConditionTrue(g)).To(BeTrue())
		Expect(g.Status.URL).NotTo(BeEmpty())
		Expect(g.Status.ConnectionSecretRef).NotTo(BeNil())

		secret := &corev1.Secret{}
		Expect(k8sClient.Get(context.Background(), types.NamespacedName{
			Name: nn.Name + "-connection", Namespace: "default"}, secret)).To(Succeed())
		Expect(secret.Data).To(HaveKey("url"))
		Expect(secret.Data).To(HaveKey("apiKey"))
		Expect(string(secret.Data["apiKey"])).NotTo(Equal("provider-secret"))
		Expect(string(secret.Data["apiKey"])).To(HaveLen(64))
	})

	It("terminates the allocation at MaxRuntime and parks in Expired without reprovisioning", func() {
		nn := types.NamespacedName{Name: "gpu-expiry", Namespace: "default"}
		Expect(k8sClient.Create(context.Background(), newGPU(nn.Name))).To(Succeed())
		reconcileUntil(nn, v1alpha1.GPUPhaseReady)
		Expect(fake.provisionCount).To(Equal(1))

		g := &v1alpha1.GPU{}
		Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
		instanceID := g.Status.InstanceID

		clock = clock.Add(2 * time.Hour)
		reconcileOnce(nn)

		Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
		Expect(g.Status.Phase).To(Equal(v1alpha1.GPUPhaseExpired))
		Expect(fake.destroyed).To(ContainElement(instanceID))

		// An Expired resource is terminal: further reconciles must not reprovision.
		provisionsBefore := fake.provisionCount
		reconcileOnce(nn)
		reconcileOnce(nn)
		Expect(fake.provisionCount).To(Equal(provisionsBefore))
		Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
		Expect(g.Status.Phase).To(Equal(v1alpha1.GPUPhaseExpired))
	})

	It("recovers a single allocation after a crash between provider creation and status persistence", func() {
		nn := types.NamespacedName{Name: "gpu-crash", Namespace: "default"}
		Expect(k8sClient.Create(context.Background(), newGPU(nn.Name))).To(Succeed())

		reconcileOnce(nn)
		reconcileOnce(nn)
		Expect(fake.provisionCount).To(Equal(1))
		Expect(fake.allocCount()).To(Equal(1))

		g := &v1alpha1.GPU{}
		Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
		g.Status = v1alpha1.GPUStatus{}
		Expect(k8sClient.Status().Update(context.Background(), g)).To(Succeed())

		reconcileOnce(nn)
		Expect(fake.provisionCount).To(Equal(2))
		Expect(fake.allocCount()).To(Equal(1))

		Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
		Expect(g.Status.InstanceID).To(Equal("inst-" + string(g.UID)))
	})

	It("releases the external allocation via the finalizer on deletion", func() {
		nn := types.NamespacedName{Name: "gpu-delete", Namespace: "default"}
		Expect(k8sClient.Create(context.Background(), newGPU(nn.Name))).To(Succeed())
		reconcileUntil(nn, v1alpha1.GPUPhaseReady)

		g := &v1alpha1.GPU{}
		Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
		instanceID := g.Status.InstanceID

		Expect(k8sClient.Delete(context.Background(), g)).To(Succeed())
		reconcileOnce(nn)

		Expect(fake.destroyed).To(ContainElement(instanceID))
		err := k8sClient.Get(context.Background(), nn, g)
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})

	It("reports Failed when the credentials Secret is missing", func() {
		nn := types.NamespacedName{Name: "gpu-nocreds", Namespace: "default"}
		g := newGPU(nn.Name)
		g.Spec.CredentialsRef.Name = "does-not-exist"
		Expect(k8sClient.Create(context.Background(), g)).To(Succeed())

		reconcileOnce(nn)
		reconcileOnce(nn)

		Expect(k8sClient.Get(context.Background(), nn, g)).To(Succeed())
		Expect(g.Status.Phase).To(Equal(v1alpha1.GPUPhaseFailed))
		Expect(fake.provisionCount).To(Equal(0))
	})
})

// readyConditionTrue reports whether the Ready condition is True.
func readyConditionTrue(g *v1alpha1.GPU) bool {
	for _, c := range g.Status.Conditions {
		if c.Type == readyConditionType {
			return c.Status == metav1.ConditionTrue
		}
	}
	return false
}
