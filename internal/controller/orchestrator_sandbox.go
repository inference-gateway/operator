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
	"maps"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

// The agent-sandbox API the Sandbox runtime drives, and what it reads back.
const (
	sandboxPodNameAnnotation  = "agents.x-k8s.io/pod-name"
	sandboxConditionReady     = "Ready"
	sandboxConditionSuspended = "Suspended"
	sandboxConditionFinished  = "Finished"
)

// The Sandbox's volumes and where the daemon finds them.
const (
	sandboxWorkspaceVolume    = "workspace"
	sandboxWorkspaceMountPath = "/workspace"
	sandboxStateVolume        = "state"
	sandboxStateMountPath     = "/home/infer/.infer/projects"
	defaultSandboxStorage     = "5Gi"
	daemonFSGroup             = int64(1000)
)

// The env vars a sandboxed daemon finds its own Orchestrator through, to
// suspend itself before it idles out.
const (
	envDaemonOrchestrator = "INFER_DAEMON_KUBERNETES_ORCHESTRATOR"
	envDaemonNamespace    = "INFER_DAEMON_KUBERNETES_NAMESPACE"
)

const sandboxCRDMissingMessage = "runtime: Sandbox needs agent-sandbox (agents.x-k8s.io/v1beta1) in the cluster: " +
	"install it from https://github.com/kubernetes-sigs/agent-sandbox/releases and restart the operator"

var sandboxGVK = schema.GroupVersionKind{Group: "agents.x-k8s.io", Version: "v1beta1", Kind: "Sandbox"}

// +kubebuilder:rbac:groups=agents.x-k8s.io,resources=sandboxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete

// newSandbox returns an empty agent-sandbox Sandbox, named after orch when given.
func newSandbox(orch *corev1alpha1.Orchestrator) *unstructured.Unstructured {
	sandbox := &unstructured.Unstructured{}
	sandbox.SetGroupVersionKind(sandboxGVK)
	if orch != nil {
		sandbox.SetName(orch.Name)
		sandbox.SetNamespace(orch.Namespace)
	}
	return sandbox
}

// sandboxCRDInstalled reports whether the cluster serves the agent-sandbox
// Sandbox kind, which the operator only watches when it does.
func sandboxCRDInstalled(mapper meta.RESTMapper) bool {
	_, err := mapper.RESTMapping(sandboxGVK.GroupKind(), sandboxGVK.Version)
	return err == nil
}

func runsInSandbox(spec *corev1alpha1.OrchestratorSpec) bool {
	return spec.Runtime == corev1alpha1.RuntimeSandbox
}

// sandboxExpiry is when spec.sandbox.shutdownTime deletes the Orchestrator,
// zero when it never does.
func sandboxExpiry(spec *corev1alpha1.OrchestratorSpec) time.Time {
	if !runsInSandbox(spec) || spec.Sandbox == nil || spec.Sandbox.ShutdownTime == nil {
		return time.Time{}
	}
	return spec.Sandbox.ShutdownTime.Time
}

// reconcileSandboxWorkload runs the daemon in the Orchestrator's Sandbox under
// its own ServiceAccount, replacing a Deployment the Deployment runtime left.
func (r *OrchestratorReconciler) reconcileSandboxWorkload(ctx context.Context, orch *corev1alpha1.Orchestrator, template corev1.PodTemplateSpec) (workloadState, error) {
	if err := r.deleteOwned(ctx, orch, orchestratorDeploymentKey(orch)); err != nil {
		return workloadState{}, err
	}
	if err := r.reconcileDaemonIdentity(ctx, orch); err != nil {
		return workloadState{}, err
	}
	sandbox, err := r.reconcileSandbox(ctx, orch, sandboxPodTemplate(orch, template))
	if err != nil {
		return workloadState{}, err
	}
	return sandboxState(sandbox), nil
}

// reconcileSandbox creates the Sandbox, or patches its pod and operating mode.
// Its volume claims stay as created, so a storage change needs a new Sandbox.
func (r *OrchestratorReconciler) reconcileSandbox(ctx context.Context, orch *corev1alpha1.Orchestrator, template corev1.PodTemplateSpec) (*unstructured.Unstructured, error) {
	podTemplate, err := sandboxPodTemplateObject(template)
	if err != nil {
		return nil, err
	}
	found := newSandbox(orch)
	err = r.Get(ctx, client.ObjectKeyFromObject(found), found)
	if apierrors.IsNotFound(err) {
		return r.createSandbox(ctx, orch, podTemplate)
	}
	if err != nil {
		return nil, err
	}
	desired := found.DeepCopy()
	desired.Object["spec"].(map[string]any)["podTemplate"] = podTemplate
	desired.Object["spec"].(map[string]any)["operatingMode"] = string(sandboxOperatingMode(orch.Spec.Sandbox, found))
	if err := r.Patch(ctx, desired, client.MergeFrom(found)); err != nil {
		return nil, err
	}
	return desired, nil
}

func (r *OrchestratorReconciler) createSandbox(ctx context.Context, orch *corev1alpha1.Orchestrator, podTemplate map[string]any) (*unstructured.Unstructured, error) {
	storage := resource.MustParse(defaultSandboxStorage)
	if orch.Spec.Sandbox != nil && orch.Spec.Sandbox.Storage != nil {
		storage = *orch.Spec.Sandbox.Storage
	}
	claim := func(volume string) any {
		return map[string]any{
			"metadata": map[string]any{"name": volume},
			"spec": map[string]any{
				"accessModes": []any{string(corev1.ReadWriteOnce)},
				"resources":   map[string]any{"requests": map[string]any{"storage": storage.String()}},
			},
		}
	}
	sandbox := newSandbox(orch)
	sandbox.SetLabels(orch.Labels)
	sandbox.Object["spec"] = map[string]any{
		"podTemplate":          podTemplate,
		"volumeClaimTemplates": []any{claim(sandboxWorkspaceVolume), claim(sandboxStateVolume)},
		"failedPodPolicy":      "Recreate",
		"operatingMode":        string(sandboxOperatingMode(orch.Spec.Sandbox, nil)),
	}
	if err := controllerutil.SetControllerReference(orch, sandbox, r.Scheme); err != nil {
		return nil, err
	}
	if err := r.Create(ctx, sandbox); err != nil {
		return nil, err
	}
	return sandbox, nil
}

// sandboxOperatingMode is the mode the Sandbox should be in: the spec's, except
// that a Running Sandbox whose pod finished is cycled through Suspended, and a
// suspension in progress completes first, so a fresh pod reattaches the volumes.
func sandboxOperatingMode(spec *corev1alpha1.OrchestratorSandboxSpec, found *unstructured.Unstructured) corev1alpha1.SandboxOperatingMode {
	if spec != nil && spec.OperatingMode == corev1alpha1.SandboxSuspended {
		return corev1alpha1.SandboxSuspended
	}
	if found == nil {
		return corev1alpha1.SandboxRunning
	}
	podFinished := sandboxCondition(found, sandboxConditionFinished) && !sandboxCondition(found, sandboxConditionReady)
	current, _, _ := unstructured.NestedString(found.Object, "spec", "operatingMode")
	suspending := current == string(corev1alpha1.SandboxSuspended) && !sandboxCondition(found, sandboxConditionSuspended)
	if podFinished || suspending {
		return corev1alpha1.SandboxSuspended
	}
	return corev1alpha1.SandboxRunning
}

// sandboxPodTemplate adapts the daemon's pod to its Sandbox: the Orchestrator's
// labels, the workspace and state volumes, its own ServiceAccount and Orchestrator
// to suspend itself through, and a restart policy that lets an idle daemon finish.
func sandboxPodTemplate(orch *corev1alpha1.Orchestrator, template corev1.PodTemplateSpec) corev1.PodTemplateSpec {
	labels := maps.Clone(orch.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	maps.Copy(labels, template.Labels)
	template.Labels = labels

	fsGroup := daemonFSGroup
	template.Spec.ServiceAccountName = daemonServiceAccountName(orch)
	template.Spec.RestartPolicy = corev1.RestartPolicyOnFailure
	template.Spec.SecurityContext = &corev1.PodSecurityContext{FSGroup: &fsGroup}

	container := &template.Spec.Containers[0]
	container.VolumeMounts = append(container.VolumeMounts,
		corev1.VolumeMount{Name: sandboxWorkspaceVolume, MountPath: sandboxWorkspaceMountPath},
		corev1.VolumeMount{Name: sandboxStateVolume, MountPath: sandboxStateMountPath},
	)
	container.Env = append(container.Env,
		corev1.EnvVar{Name: envDaemonOrchestrator, Value: orch.Name},
		corev1.EnvVar{Name: envDaemonNamespace, ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
		}},
	)
	return template
}

// sandboxPodTemplateObject renders the pod template the way the Sandbox CRD
// stores it: labels, annotations and the pod spec.
func sandboxPodTemplateObject(template corev1.PodTemplateSpec) (map[string]any, error) {
	spec, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&template.Spec)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"metadata": map[string]any{"labels": anyMap(template.Labels), "annotations": anyMap(template.Annotations)},
		"spec":     spec,
	}, nil
}

func anyMap(values map[string]string) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

// sandboxState reports the Sandbox's readiness, and the pod it runs once ready.
func sandboxState(sandbox *unstructured.Unstructured) workloadState {
	mode, _, _ := unstructured.NestedString(sandbox.Object, "spec", "operatingMode")
	switch {
	case sandboxCondition(sandbox, sandboxConditionReady):
		return workloadState{ready: true, reason: "SandboxReady", message: "the daemon's sandbox is ready", podName: sandboxPodName(sandbox)}
	case mode == string(corev1alpha1.SandboxSuspended):
		return workloadState{reason: "SandboxSuspended", message: "the daemon's sandbox is suspended, set spec.sandbox.operatingMode to Running to resume it"}
	default:
		return workloadState{reason: "SandboxNotReady", message: "the daemon's sandbox is not ready yet"}
	}
}

// sandboxPodName is the pod agent-sandbox names in its annotation, else the
// pod named after the Sandbox, which is all agent-sandbox v1.0.8 creates.
func sandboxPodName(sandbox *unstructured.Unstructured) string {
	if name := sandbox.GetAnnotations()[sandboxPodNameAnnotation]; name != "" {
		return name
	}
	return sandbox.GetName()
}

func sandboxCondition(sandbox *unstructured.Unstructured, conditionType string) bool {
	conditions, _, _ := unstructured.NestedSlice(sandbox.Object, "status", "conditions")
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if ok && condition["type"] == conditionType {
			return condition["status"] == string(metav1.ConditionTrue)
		}
	}
	return false
}

func daemonServiceAccountName(orch *corev1alpha1.Orchestrator) string {
	return orch.Name + "-daemon"
}

// reconcileDaemonIdentity gives the sandboxed daemon a ServiceAccount that may
// get and patch its own Orchestrator and nothing else, so it can suspend itself.
func (r *OrchestratorReconciler) reconcileDaemonIdentity(ctx context.Context, orch *corev1alpha1.Orchestrator) error {
	name := daemonServiceAccountName(orch)
	objectMeta := metav1.ObjectMeta{Name: name, Namespace: orch.Namespace, Labels: map[string]string{"app": orch.Name}}

	account := &corev1.ServiceAccount{ObjectMeta: objectMeta}
	if err := r.ensureOwned(ctx, orch, account, func() {}); err != nil {
		return err
	}
	role := &rbacv1.Role{ObjectMeta: objectMeta}
	if err := r.ensureOwned(ctx, orch, role, func() {
		role.Rules = []rbacv1.PolicyRule{{
			APIGroups:     []string{corev1alpha1.GroupVersion.Group},
			Resources:     []string{"orchestrators"},
			ResourceNames: []string{orch.Name},
			Verbs:         []string{"get", "patch"},
		}}
	}); err != nil {
		return err
	}
	binding := &rbacv1.RoleBinding{ObjectMeta: objectMeta}
	return r.ensureOwned(ctx, orch, binding, func() {
		binding.RoleRef = rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: name}
		binding.Subjects = []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: name, Namespace: orch.Namespace}}
	})
}

// ensureOwned creates or updates obj with mutate applied and the Orchestrator
// as its controller, so garbage collection removes it with the Orchestrator.
func (r *OrchestratorReconciler) ensureOwned(ctx context.Context, orch *corev1alpha1.Orchestrator, obj client.Object, mutate func()) error {
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, obj, func() error {
		mutate()
		return controllerutil.SetControllerReference(orch, obj, r.Scheme)
	})
	return err
}

// deleteOwned deletes obj when the Orchestrator controls it, and leaves
// anything else of that name alone.
func (r *OrchestratorReconciler) deleteOwned(ctx context.Context, orch *corev1alpha1.Orchestrator, obj client.Object) error {
	if err := r.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !metav1.IsControlledBy(obj, orch) {
		return nil
	}
	return client.IgnoreNotFound(r.Delete(ctx, obj))
}
