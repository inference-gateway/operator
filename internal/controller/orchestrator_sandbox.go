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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/api/meta"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	ptr "k8s.io/utils/ptr"
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
	sandboxReasonPodSucceeded = "PodSucceeded"
	sandboxReasonExpired      = "SandboxExpired"
)

// What the operator records on a Sandbox: the pod template its pod runs, and
// when the daemon idled out and whether every agent was done by then.
const (
	podTemplateHashAnnotation = "inference-gateway.com/pod-template-hash"
	idleSinceAnnotation       = "inference-gateway.com/idle-since"
	agentsDoneAnnotation      = "inference-gateway.com/agents-done"
)

// agentsDoneMessage is the termination message an idle daemon leaves when
// every agent was done, which starts spec.sandbox.retention.
const agentsDoneMessage = "agents-done=true"

// The Sandbox's volumes and where the daemon finds them.
const (
	sandboxWorkspaceVolume    = "workspace"
	sandboxWorkspaceMountPath = "/workspace"
	sandboxStateVolume        = "state"
	sandboxStateMountPath     = "/home/infer/.infer/projects"
	defaultSandboxStorage     = "5Gi"
	daemonUID                 = int64(1000)
)

const sandboxCRDMissingMessage = "runtime: Sandbox needs agent-sandbox (agents.x-k8s.io/v1beta1) in the cluster: " +
	"install it from https://github.com/kubernetes-sigs/agent-sandbox/releases and restart the operator"

var sandboxGVK = schema.GroupVersionKind{Group: "agents.x-k8s.io", Version: "v1beta1", Kind: "Sandbox"}

// errOrchestratorExpired reports that the Orchestrator was deleted because its
// Sandbox expired, so there is nothing left to reconcile.
var errOrchestratorExpired = errors.New("the orchestrator's sandbox expired")

// +kubebuilder:rbac:groups=agents.x-k8s.io,resources=sandboxes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get

// sandboxKind is an empty agent-sandbox Sandbox, the type the controller watches.
func sandboxKind() *unstructured.Unstructured {
	sandbox := &unstructured.Unstructured{}
	sandbox.SetGroupVersionKind(sandboxGVK)
	return sandbox
}

// sandboxOf is the Sandbox the Orchestrator's daemon runs in, named after it.
func sandboxOf(orch *corev1alpha1.Orchestrator) *unstructured.Unstructured {
	sandbox := sandboxKind()
	sandbox.SetName(orch.Name)
	sandbox.SetNamespace(orch.Namespace)
	return sandbox
}

// sandboxCRDInstalled reports whether the cluster serves the agent-sandbox
// Sandbox kind. Any error but a missing kind fails, so a discovery hiccup at
// startup restarts the operator instead of turning the runtime off.
func sandboxCRDInstalled(mapper meta.RESTMapper) (bool, error) {
	_, err := mapper.RESTMapping(sandboxGVK.GroupKind(), sandboxGVK.Version)
	if meta.IsNoMatchError(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("looking up the agent-sandbox Sandbox kind: %w", err)
	}
	return true, nil
}

func runsInSandbox(spec *corev1alpha1.OrchestratorSpec) bool {
	return spec.Runtime == corev1alpha1.RuntimeSandbox
}

func wantsSuspended(spec *corev1alpha1.OrchestratorSpec) bool {
	return spec.Sandbox != nil && spec.Sandbox.OperatingMode == corev1alpha1.SandboxSuspended
}

// sandboxExpiry is when spec.sandbox.shutdownTime deletes the Orchestrator,
// zero when it never does.
func sandboxExpiry(spec *corev1alpha1.OrchestratorSpec) time.Time {
	if !runsInSandbox(spec) || spec.Sandbox == nil || spec.Sandbox.ShutdownTime == nil {
		return time.Time{}
	}
	return spec.Sandbox.ShutdownTime.Time
}

// reconcileSandboxWorkload runs the daemon in the Orchestrator's Sandbox. It
// parks the Orchestrator when its daemon idled out, rolls the pod onto a changed
// template, and deletes the Orchestrator once the Sandbox expired.
func (r *OrchestratorReconciler) reconcileSandboxWorkload(ctx context.Context, orch *corev1alpha1.Orchestrator, template corev1.PodTemplateSpec, now time.Time) (workloadState, error) {
	podTemplate, templateHash, err := sandboxPodTemplateObject(sandboxPodTemplate(orch, template))
	if err != nil {
		return workloadState{}, err
	}
	found := sandboxOf(orch)
	err = r.Get(ctx, client.ObjectKeyFromObject(found), found)
	if apierrors.IsNotFound(err) {
		created, err := r.createSandbox(ctx, orch, podTemplate, templateHash)
		if err != nil {
			return workloadState{}, err
		}
		return sandboxState(created), nil
	}
	if err != nil {
		return workloadState{}, fmt.Errorf("reading Sandbox %s: %w", found.GetName(), err)
	}

	if sandboxReadyReason(found) == sandboxReasonExpired {
		if err := client.IgnoreNotFound(r.Delete(ctx, orch)); err != nil {
			return workloadState{}, fmt.Errorf("deleting the expired Orchestrator %s: %w", orch.Name, err)
		}
		return workloadState{}, errOrchestratorExpired
	}

	desired := found.DeepCopy()
	if !wantsSuspended(&orch.Spec) && daemonIdledOut(found) {
		if err := r.parkIdleDaemon(ctx, orch, desired, now); err != nil {
			return workloadState{}, err
		}
	}
	if err := applySandboxSpec(desired, orch, podTemplate, templateHash); err != nil {
		return workloadState{}, err
	}
	if err := r.Patch(ctx, desired, client.MergeFrom(found)); err != nil {
		return workloadState{}, fmt.Errorf("updating Sandbox %s: %w", desired.GetName(), err)
	}
	return sandboxState(desired), nil
}

func (r *OrchestratorReconciler) createSandbox(ctx context.Context, orch *corev1alpha1.Orchestrator, podTemplate map[string]any, templateHash string) (*unstructured.Unstructured, error) {
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
	sandbox := sandboxOf(orch)
	sandbox.SetLabels(orch.Labels)
	sandbox.Object["spec"] = map[string]any{
		"volumeClaimTemplates": []any{claim(sandboxWorkspaceVolume), claim(sandboxStateVolume)},
		"failedPodPolicy":      "Recreate",
	}
	if err := applySandboxSpec(sandbox, orch, podTemplate, templateHash); err != nil {
		return nil, err
	}
	if err := controllerutil.SetControllerReference(orch, sandbox, r.Scheme); err != nil {
		return nil, err
	}
	if err := r.Create(ctx, sandbox); err != nil {
		return nil, fmt.Errorf("creating Sandbox %s: %w", sandbox.GetName(), err)
	}
	return sandbox, nil
}

// applySandboxSpec sets what the operator drives on the Sandbox: the pod
// template, the operating mode a rollout needs, and the expiry.
func applySandboxSpec(sandbox *unstructured.Unstructured, orch *corev1alpha1.Orchestrator, podTemplate map[string]any, templateHash string) error {
	mode, recordedHash := sandboxRollout(wantsSuspended(&orch.Spec), sandbox, templateHash)
	annotations := sandbox.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[podTemplateHashAnnotation] = recordedHash
	if mode == string(corev1alpha1.SandboxRunning) {
		delete(annotations, idleSinceAnnotation)
		delete(annotations, agentsDoneAnnotation)
	}
	sandbox.SetAnnotations(annotations)

	fields := map[string]any{"podTemplate": podTemplate, "operatingMode": mode}
	if shutdown := sandboxShutdownTime(orch.Spec.Sandbox, annotations); shutdown != nil {
		fields["shutdownTime"] = shutdown.UTC().Format(time.RFC3339)
		fields["shutdownPolicy"] = "Retain"
	} else {
		unstructured.RemoveNestedField(sandbox.Object, "spec", "shutdownTime")
		unstructured.RemoveNestedField(sandbox.Object, "spec", "shutdownPolicy")
	}
	for field, value := range fields {
		if err := unstructured.SetNestedField(sandbox.Object, value, "spec", field); err != nil {
			return fmt.Errorf("setting spec.%s on Sandbox %s: %w", field, sandbox.GetName(), err)
		}
	}
	return nil
}

// sandboxRollout is the operating mode the Sandbox should be in, and the pod
// template hash its pod runs. A suspension completes before a resume, and a
// running pod whose template changed is cycled through Suspended, because
// agent-sandbox applies a new template only to the pod it creates next.
func sandboxRollout(suspend bool, sandbox *unstructured.Unstructured, templateHash string) (string, string) {
	running, suspended := string(corev1alpha1.SandboxRunning), string(corev1alpha1.SandboxSuspended)
	recorded := sandbox.GetAnnotations()[podTemplateHashAnnotation]
	current, _, _ := unstructured.NestedString(sandbox.Object, "spec", "operatingMode")
	switch {
	case current == "":
		if suspend {
			return suspended, templateHash
		}
		return running, templateHash
	case suspend:
		return suspended, templateHash
	case current == suspended && !sandboxConditionTrue(sandbox, sandboxConditionSuspended):
		return suspended, recorded
	case current == suspended:
		return running, templateHash
	case recorded != templateHash:
		return suspended, recorded
	default:
		return running, recorded
	}
}

// daemonIdledOut reports that the daemon's pod exited 0, which the daemon only
// does when it idles out. agent-sandbox leaves a succeeded pod terminal.
func daemonIdledOut(sandbox *unstructured.Unstructured) bool {
	status, reason, _ := sandboxCondition(sandbox, sandboxConditionFinished)
	return status == string(metav1.ConditionTrue) && reason == sandboxReasonPodSucceeded
}

// parkIdleDaemon suspends the Orchestrator whose daemon idled out, the way a
// client would, so a client resumes it by setting Running again. It records on
// the Sandbox when, and whether the daemon reported every agent done.
func (r *OrchestratorReconciler) parkIdleDaemon(ctx context.Context, orch *corev1alpha1.Orchestrator, sandbox *unstructured.Unstructured, now time.Time) error {
	agentsDone, err := r.daemonReportedAgentsDone(ctx, sandbox)
	if err != nil {
		return err
	}
	patch := client.MergeFrom(orch.DeepCopy())
	if orch.Spec.Sandbox == nil {
		orch.Spec.Sandbox = &corev1alpha1.OrchestratorSandboxSpec{}
	}
	orch.Spec.Sandbox.OperatingMode = corev1alpha1.SandboxSuspended
	if err := r.Patch(ctx, orch, patch); err != nil {
		return fmt.Errorf("suspending the idle Orchestrator %s: %w", orch.Name, err)
	}
	annotations := sandbox.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[idleSinceAnnotation] = now.UTC().Format(time.RFC3339)
	annotations[agentsDoneAnnotation] = strconv.FormatBool(agentsDone)
	sandbox.SetAnnotations(annotations)
	return nil
}

// daemonReportedAgentsDone reads the termination message the idle daemon left
// on its pod. A pod already gone counts as agents not done, which only keeps
// the Orchestrator longer.
func (r *OrchestratorReconciler) daemonReportedAgentsDone(ctx context.Context, sandbox *unstructured.Unstructured) (bool, error) {
	pod := &corev1.Pod{}
	key := client.ObjectKey{Namespace: sandbox.GetNamespace(), Name: sandboxPodName(sandbox)}
	if err := r.PodReader.Get(ctx, key, pod); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading the idle daemon's pod %s: %w", key.Name, err)
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name == orchestratorContainerName && status.State.Terminated != nil {
			return strings.TrimSpace(status.State.Terminated.Message) == agentsDoneMessage, nil
		}
	}
	return false, nil
}

// sandboxShutdownTime is when the Sandbox expires: spec.sandbox.shutdownTime,
// or sooner once the daemon idled out with every agent done and the retention
// ran out. Nil when it never does.
func sandboxShutdownTime(spec *corev1alpha1.OrchestratorSandboxSpec, annotations map[string]string) *time.Time {
	var shutdown *time.Time
	if spec != nil && spec.ShutdownTime != nil {
		shutdown = &spec.ShutdownTime.Time
	}
	idleSince, err := time.Parse(time.RFC3339, annotations[idleSinceAnnotation])
	if err != nil || annotations[agentsDoneAnnotation] != "true" || spec == nil || spec.Retention == nil {
		return shutdown
	}
	retained := idleSince.Add(spec.Retention.Duration)
	if shutdown == nil || retained.Before(*shutdown) {
		return &retained
	}
	return shutdown
}

// sandboxPodTemplate adapts the daemon's pod to its Sandbox: the Orchestrator's
// labels, the workspace and state volumes, no Kubernetes API access, a non-root
// user without capabilities, and a restart policy that lets an idle daemon finish.
func sandboxPodTemplate(orch *corev1alpha1.Orchestrator, template corev1.PodTemplateSpec) corev1.PodTemplateSpec {
	template = *template.DeepCopy()
	labels := maps.Clone(orch.Labels)
	if labels == nil {
		labels = map[string]string{}
	}
	maps.Copy(labels, template.Labels)
	template.Labels = labels

	spec := &template.Spec
	spec.RestartPolicy = corev1.RestartPolicyOnFailure
	spec.AutomountServiceAccountToken = ptr.To(false)
	spec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot:   ptr.To(true),
		RunAsUser:      ptr.To(daemonUID),
		RunAsGroup:     ptr.To(daemonUID),
		FSGroup:        ptr.To(daemonUID),
		SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
	}
	if orch.Spec.Sandbox != nil {
		spec.RuntimeClassName = orch.Spec.Sandbox.RuntimeClassName
	}

	container := &spec.Containers[0]
	container.SecurityContext = &corev1.SecurityContext{
		AllowPrivilegeEscalation: ptr.To(false),
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
	}
	container.VolumeMounts = append(container.VolumeMounts,
		corev1.VolumeMount{Name: sandboxWorkspaceVolume, MountPath: sandboxWorkspaceMountPath},
		corev1.VolumeMount{Name: sandboxStateVolume, MountPath: sandboxStateMountPath},
	)
	return template
}

// sandboxPodTemplateObject renders the pod template the way the Sandbox CRD
// stores it, and hashes it so a change is noticed and rolled out.
func sandboxPodTemplateObject(template corev1.PodTemplateSpec) (map[string]any, string, error) {
	spec, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&template.Spec)
	if err != nil {
		return nil, "", fmt.Errorf("rendering the daemon's pod template: %w", err)
	}
	podTemplate := map[string]any{
		"metadata": map[string]any{"labels": anyMap(template.Labels), "annotations": anyMap(template.Annotations)},
		"spec":     spec,
	}
	rendered, err := json.Marshal(podTemplate)
	if err != nil {
		return nil, "", fmt.Errorf("hashing the daemon's pod template: %w", err)
	}
	sum := sha256.Sum256(rendered)
	return podTemplate, hex.EncodeToString(sum[:]), nil
}

func anyMap(values map[string]string) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

// sandboxState reports the Sandbox's readiness, and the pod it runs once
// ready. A Sandbox that is not ready passes on why, such as an image pull error.
func sandboxState(sandbox *unstructured.Unstructured) workloadState {
	mode, _, _ := unstructured.NestedString(sandbox.Object, "spec", "operatingMode")
	status, reason, message := sandboxCondition(sandbox, sandboxConditionReady)
	switch {
	case status == string(metav1.ConditionTrue):
		return workloadState{ready: true, reason: "SandboxReady", message: "the daemon's sandbox is ready", podName: sandboxPodName(sandbox)}
	case mode == string(corev1alpha1.SandboxSuspended):
		return workloadState{reason: "SandboxSuspended", message: "the daemon's sandbox is suspended, set spec.sandbox.operatingMode to Running to resume it"}
	case reason != "":
		return workloadState{reason: "SandboxNotReady", message: fmt.Sprintf("the daemon's sandbox is not ready: %s: %s", reason, message)}
	default:
		return workloadState{reason: "SandboxNotReady", message: "the daemon's sandbox is not ready yet"}
	}
}

// sandboxPodName is the pod agent-sandbox names in its deprecated annotation,
// else the pod named after the Sandbox, which is what agent-sandbox creates.
func sandboxPodName(sandbox *unstructured.Unstructured) string {
	if name := sandbox.GetAnnotations()[sandboxPodNameAnnotation]; name != "" {
		return name
	}
	return sandbox.GetName()
}

func sandboxConditionTrue(sandbox *unstructured.Unstructured, conditionType string) bool {
	status, _, _ := sandboxCondition(sandbox, conditionType)
	return status == string(metav1.ConditionTrue)
}

func sandboxReadyReason(sandbox *unstructured.Unstructured) string {
	_, reason, _ := sandboxCondition(sandbox, sandboxConditionReady)
	return reason
}

// sandboxCondition reads a condition agent-sandbox reported for the Sandbox's
// current generation. One about an older spec reads as unknown, since it says
// nothing about the change that was just made.
func sandboxCondition(sandbox *unstructured.Unstructured, conditionType string) (status, reason, message string) {
	conditions, _, _ := unstructured.NestedSlice(sandbox.Object, "status", "conditions")
	for _, raw := range conditions {
		condition, ok := raw.(map[string]any)
		if !ok || condition["type"] != conditionType {
			continue
		}
		if observed, found, _ := unstructured.NestedInt64(condition, "observedGeneration"); found && observed != sandbox.GetGeneration() {
			return "", "", ""
		}
		status, _ = condition["status"].(string)
		reason, _ = condition["reason"].(string)
		message, _ = condition["message"].(string)
		return status, reason, message
	}
	return "", "", ""
}
