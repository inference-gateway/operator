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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
	"github.com/inference-gateway/operator/internal/gpu"
)

const (
	// gpuFinalizer guards deletion so the external allocation is released first.
	gpuFinalizer = "core.inference-gateway.com/gpu-cleanup"

	// connectionSecretSuffix is appended to the GPU name for its connection Secret.
	connectionSecretSuffix = "-connection"

	// Requeue cadences.
	gpuProvisioningRequeue = 10 * time.Second
	gpuFailedRequeue       = 2 * time.Minute
	gpuCleanupRequeue      = 30 * time.Second
	gpuReadyMaxRequeue     = 60 * time.Second

	readyConditionType = "Ready"
)

// GPUReconciler reconciles a GPU object by driving a pluggable provider through
// the lease lifecycle: provision, wait for HTTP readiness, publish a connection
// Secret, enforce MaxRuntime, and release the allocation on deletion/expiry.
type GPUReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// NewProvider builds a provider driver from its name and management API key.
	// Overridable in tests; defaults to gpu.NewProvider.
	NewProvider func(name, apiKey string) (gpu.Provider, error)
	// CheckReady probes the runtime's HTTP readiness endpoint (optionally
	// authenticated with the per-allocation token). Overridable in tests.
	CheckReady func(ctx context.Context, url, token string) error
	// Now is the clock, overridable in tests to drive MaxRuntime expiry.
	Now func() time.Time
}

// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gpus,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gpus/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=gpus/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile drives the GPU allocation lifecycle.
func (r *GPUReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.applyDefaults()
	logger := logf.FromContext(ctx)

	if !r.shouldWatchNamespace(ctx, req.Namespace) {
		logger.V(1).Info("skipping gpu in namespace not matching watch criteria", "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	var g v1alpha1.GPU
	if err := r.Get(ctx, req.NamespacedName, &g); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !g.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &g)
	}

	if !controllerutil.ContainsFinalizer(&g, gpuFinalizer) {
		controllerutil.AddFinalizer(&g, gpuFinalizer)
		if err := r.Update(ctx, &g); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Expired resources are terminal until the spec changes (guards against
	// silently re-leasing an expensive GPU on its own).
	if g.Status.Phase == v1alpha1.GPUPhaseExpired && g.Status.ObservedGeneration == g.Generation {
		return ctrl.Result{}, nil
	}

	provider, err := r.providerFor(ctx, &g)
	if err != nil {
		return r.fail(ctx, &g, "ProviderUnavailable", err)
	}

	// Enforce MaxRuntime once the allocation has a deadline.
	if g.Status.ExpiresAt != nil && !r.Now().Before(g.Status.ExpiresAt.Time) {
		return r.expire(ctx, &g, provider)
	}

	if g.Status.InstanceID == "" {
		return r.provision(ctx, &g, provider)
	}
	return r.observe(ctx, &g, provider)
}

// providerFor reads the management credential from the referenced Secret and
// builds the configured provider driver.
func (r *GPUReconciler) providerFor(ctx context.Context, g *v1alpha1.GPU) (gpu.Provider, error) {
	ref := g.Spec.CredentialsRef
	secret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: g.Namespace}, secret); err != nil {
		return nil, fmt.Errorf("failed to read credentials secret %q: %w", ref.Name, err)
	}
	apiKey := string(secret.Data[ref.Key])
	if apiKey == "" {
		return nil, fmt.Errorf("credentials secret %q has no data at key %q", ref.Name, ref.Key)
	}
	return r.NewProvider(g.Spec.Provider, apiKey)
}

// provision requests a new allocation. The per-allocation token and connection
// Secret are persisted BEFORE provisioning so the token handed to the runtime
// survives a crash, and the InstanceID recovers the same external allocation.
func (r *GPUReconciler) provision(ctx context.Context, g *v1alpha1.GPU, provider gpu.Provider) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	token, err := r.ensureConnectionSecret(ctx, g, "")
	if err != nil {
		return r.fail(ctx, g, "ConnectionSecretError", err)
	}

	alloc, err := provider.Provision(ctx, gpu.ProvisionRequest{
		UID:           string(g.UID),
		Namespace:     g.Namespace,
		Name:          g.Name,
		Image:         g.Spec.Image,
		Command:       g.Spec.Command,
		GPUTypes:      g.Spec.GPUTypes,
		Port:          endpointPort(g),
		EndpointToken: token,
	})
	if err != nil {
		return r.fail(ctx, g, "ProvisionFailed", err)
	}

	now := metav1.NewTime(r.Now())
	g.Status.InstanceID = alloc.ID
	g.Status.URL = alloc.URL
	g.Status.StartedAt = &now
	expires := metav1.NewTime(now.Add(g.Spec.MaxRuntime.Duration))
	g.Status.ExpiresAt = &expires
	g.Status.Phase = v1alpha1.GPUPhaseProvisioning
	setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionFalse, "Provisioning",
		"allocation requested, waiting for infrastructure"))

	logger.Info("provisioned gpu allocation", "gpu", g.Name, "instanceID", alloc.ID)
	if err := r.persistStatus(ctx, g); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: gpuProvisioningRequeue}, nil
}

// observe fetches the current allocation state and advances the phase, checking
// the actual HTTP inference endpoint before reporting Ready.
func (r *GPUReconciler) observe(ctx context.Context, g *v1alpha1.GPU, provider gpu.Provider) (ctrl.Result, error) {
	alloc, err := provider.Get(ctx, g.Status.InstanceID)
	if errors.Is(err, gpu.ErrNotFound) {
		return r.fail(ctx, g, "AllocationLost", fmt.Errorf("provider no longer has allocation %s", g.Status.InstanceID))
	}
	if err != nil {
		return r.fail(ctx, g, "ProviderError", err)
	}
	if alloc.URL != "" {
		g.Status.URL = alloc.URL
	}

	switch alloc.State {
	case gpu.StateFailed:
		return r.fail(ctx, g, "ProviderFailed", fmt.Errorf("provider reported allocation %s failed", alloc.ID))
	case gpu.StateTerminated:
		return r.fail(ctx, g, "AllocationTerminated", fmt.Errorf("allocation %s terminated externally", alloc.ID))
	case gpu.StateRunning:
		return r.observeRunning(ctx, g)
	default:
		g.Status.Phase = v1alpha1.GPUPhaseStarting
		setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionFalse, "InfrastructureStarting",
			"waiting for provider infrastructure to start"))
		if err := r.persistStatus(ctx, g); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: gpuProvisioningRequeue}, nil
	}
}

// observeRunning is the running-infrastructure branch: it gates Ready on the HTTP
// readiness endpoint actually responding, per the spec's readinessPath.
func (r *GPUReconciler) observeRunning(ctx context.Context, g *v1alpha1.GPU) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	if g.Status.URL == "" {
		g.Status.Phase = v1alpha1.GPUPhaseStarting
		setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionFalse, "EndpointPending",
			"infrastructure running, endpoint not yet resolved"))
		if err := r.persistStatus(ctx, g); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: gpuProvisioningRequeue}, nil
	}

	token, err := r.tokenFromConnectionSecret(ctx, g)
	if err != nil {
		return r.fail(ctx, g, "ConnectionSecretError", err)
	}

	probeURL := g.Status.URL + g.Spec.Endpoint.ReadinessPath
	if probeErr := r.CheckReady(ctx, probeURL, token); probeErr != nil {
		logger.V(1).Info("gpu endpoint not ready", "gpu", g.Name, "url", probeURL, "err", probeErr)
		g.Status.Phase = v1alpha1.GPUPhaseStarting
		setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionFalse, "EndpointNotReady",
			fmt.Sprintf("readiness probe failed: %v", probeErr)))
		if err := r.persistStatus(ctx, g); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: gpuProvisioningRequeue}, nil
	}

	if _, err := r.ensureConnectionSecret(ctx, g, g.Status.URL); err != nil {
		return r.fail(ctx, g, "ConnectionSecretError", err)
	}
	g.Status.Phase = v1alpha1.GPUPhaseReady
	setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionTrue, "EndpointReady",
		"inference endpoint responded to the readiness probe"))
	if err := r.persistStatus(ctx, g); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: r.readyRequeue(g)}, nil
}

// expire releases the allocation after MaxRuntime and parks the resource in the
// Expired phase. It does not delete the resource or auto-reprovision.
func (r *GPUReconciler) expire(ctx context.Context, g *v1alpha1.GPU, provider gpu.Provider) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("gpu allocation reached MaxRuntime, releasing", "gpu", g.Name, "instanceID", g.Status.InstanceID)

	if g.Status.InstanceID != "" {
		if err := provider.Destroy(ctx, g.Status.InstanceID); err != nil {
			g.Status.Phase = v1alpha1.GPUPhaseTerminating
			setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionFalse, "TerminationFailed",
				fmt.Sprintf("failed to release allocation: %v", err)))
			_ = r.persistStatus(ctx, g)
			return ctrl.Result{RequeueAfter: gpuCleanupRequeue}, nil
		}
	}

	g.Status.InstanceID = ""
	g.Status.URL = ""
	g.Status.Phase = v1alpha1.GPUPhaseExpired
	setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionFalse, "Expired",
		"allocation released after reaching MaxRuntime"))
	if err := r.persistStatus(ctx, g); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// finalize releases the external allocation before allowing deletion to proceed.
// Cleanup is best-effort but blocking: if the allocation cannot be released the
// finalizer is kept and a condition explains the stall (orphan-cost protection).
func (r *GPUReconciler) finalize(ctx context.Context, g *v1alpha1.GPU) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(g, gpuFinalizer) {
		return ctrl.Result{}, nil
	}

	if g.Status.InstanceID != "" {
		provider, err := r.providerFor(ctx, g)
		if err != nil {
			logger.Error(err, "cannot build provider for cleanup; keeping finalizer", "gpu", g.Name)
			setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionFalse, "CleanupBlocked",
				fmt.Sprintf("cannot release allocation %s: %v", g.Status.InstanceID, err)))
			_ = r.persistStatus(ctx, g)
			return ctrl.Result{RequeueAfter: gpuCleanupRequeue}, nil
		}
		if err := provider.Destroy(ctx, g.Status.InstanceID); err != nil {
			logger.Error(err, "failed to release allocation; keeping finalizer", "gpu", g.Name)
			setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionFalse, "CleanupFailed",
				fmt.Sprintf("failed to release allocation %s: %v", g.Status.InstanceID, err)))
			_ = r.persistStatus(ctx, g)
			return ctrl.Result{RequeueAfter: gpuCleanupRequeue}, nil
		}
		logger.Info("released gpu allocation during deletion", "gpu", g.Name, "instanceID", g.Status.InstanceID)
	}

	controllerutil.RemoveFinalizer(g, gpuFinalizer)
	if err := r.Update(ctx, g); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
	}
	return ctrl.Result{}, nil
}

// fail records a failure phase and requeues on a slow cadence. It returns a nil
// error (with RequeueAfter) so controller-runtime does not hot-loop on backoff.
func (r *GPUReconciler) fail(ctx context.Context, g *v1alpha1.GPU, reason string, cause error) (ctrl.Result, error) {
	logf.FromContext(ctx).Error(cause, "gpu reconcile failed", "gpu", g.Name, "reason", reason)
	g.Status.Phase = v1alpha1.GPUPhaseFailed
	setCondition(&g.Status.Conditions, r.condition(g, metav1.ConditionFalse, reason, cause.Error()))
	if err := r.persistStatus(ctx, g); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: gpuFailedRequeue}, nil
}

// ensureConnectionSecret creates or updates the per-allocation connection Secret,
// returning its stable endpoint token. The token is generated once and reused so
// it stays consistent with what the runtime was started with. The provider API
// key is never written here.
func (r *GPUReconciler) ensureConnectionSecret(ctx context.Context, g *v1alpha1.GPU, url string) (string, error) {
	name := g.Name + connectionSecretSuffix
	secret := &corev1.Secret{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: g.Namespace}, secret)

	if apierrors.IsNotFound(err) {
		token, tErr := generateToken()
		if tErr != nil {
			return "", tErr
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: g.Namespace},
			StringData: map[string]string{"url": url, "apiKey": token},
		}
		if err := controllerutil.SetControllerReference(g, secret, r.Scheme); err != nil {
			return "", err
		}
		if err := r.Create(ctx, secret); err != nil {
			return "", fmt.Errorf("failed to create connection secret: %w", err)
		}
		r.setConnectionSecretRef(g, name)
		return token, nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to read connection secret: %w", err)
	}

	token := string(secret.Data["apiKey"])
	if token == "" {
		if token, err = generateToken(); err != nil {
			return "", err
		}
	}
	if string(secret.Data["url"]) != url || string(secret.Data["apiKey"]) != token {
		secret.StringData = map[string]string{"url": url, "apiKey": token}
		if err := r.Update(ctx, secret); err != nil {
			return "", fmt.Errorf("failed to update connection secret: %w", err)
		}
	}
	r.setConnectionSecretRef(g, name)
	return token, nil
}

func (r *GPUReconciler) tokenFromConnectionSecret(ctx context.Context, g *v1alpha1.GPU) (string, error) {
	secret := &corev1.Secret{}
	name := g.Name + connectionSecretSuffix
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: g.Namespace}, secret); err != nil {
		return "", client.IgnoreNotFound(err)
	}
	return string(secret.Data["apiKey"]), nil
}

func (r *GPUReconciler) setConnectionSecretRef(g *v1alpha1.GPU, name string) {
	if g.Status.ConnectionSecretRef == nil || g.Status.ConnectionSecretRef.Name != name {
		g.Status.ConnectionSecretRef = &corev1.LocalObjectReference{Name: name}
	}
}

// persistStatus stamps ObservedGeneration and writes the status subresource.
func (r *GPUReconciler) persistStatus(ctx context.Context, g *v1alpha1.GPU) error {
	g.Status.ObservedGeneration = g.Generation
	if err := r.Status().Update(ctx, g); err != nil {
		return fmt.Errorf("failed to update gpu status: %w", err)
	}
	return nil
}

func (r *GPUReconciler) condition(g *v1alpha1.GPU, status metav1.ConditionStatus, reason, msg string) metav1.Condition {
	return metav1.Condition{
		Type:               readyConditionType,
		Status:             status,
		Reason:             reason,
		Message:            msg,
		ObservedGeneration: g.Generation,
		LastTransitionTime: metav1.NewTime(r.Now()),
	}
}

// readyRequeue wakes the reconciler again to re-probe readiness and, at the
// latest, to enforce MaxRuntime.
func (r *GPUReconciler) readyRequeue(g *v1alpha1.GPU) time.Duration {
	d := gpuReadyMaxRequeue
	if g.Status.ExpiresAt != nil {
		if untilExpiry := g.Status.ExpiresAt.Sub(r.Now()); untilExpiry < d {
			d = untilExpiry
		}
	}
	if d < time.Second {
		d = time.Second
	}
	return d
}

func (r *GPUReconciler) applyDefaults() {
	if r.NewProvider == nil {
		r.NewProvider = gpu.NewProvider
	}
	if r.CheckReady == nil {
		r.CheckReady = defaultCheckReady
	}
	if r.Now == nil {
		r.Now = time.Now
	}
}

// shouldWatchNamespace mirrors the other reconcilers' WATCH_NAMESPACE_SELECTOR gate.
func (r *GPUReconciler) shouldWatchNamespace(ctx context.Context, namespace string) bool {
	watchNamespaceSelector := os.Getenv("WATCH_NAMESPACE_SELECTOR")
	if watchNamespaceSelector == "" {
		return true
	}
	labelSelector, err := labels.Parse(watchNamespaceSelector)
	if err != nil {
		return true
	}
	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
		return false
	}
	return labelSelector.Matches(labels.Set(ns.Labels))
}

// SetupWithManager sets up the controller with the Manager.
func (r *GPUReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.applyDefaults()
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GPU{}).
		Owns(&corev1.Secret{}).
		Named("gpu").
		Complete(r)
}

func endpointPort(g *v1alpha1.GPU) int32 {
	if g.Spec.Endpoint.Port != 0 {
		return g.Spec.Endpoint.Port
	}
	return 8080
}

func generateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate endpoint token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// defaultCheckReady performs the real HTTP readiness probe, treating any 2xx as
// ready. A non-empty token is sent as a bearer credential.
func defaultCheckReady(ctx context.Context, url, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	httpClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return fmt.Errorf("readiness probe %s returned status %d", url, resp.StatusCode)
}
