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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

var _ = Describe("Orchestrator with runtime: Sandbox", func() {
	ctx := context.Background()

	sandboxSpec := func(sandbox *corev1alpha1.OrchestratorSandboxSpec) corev1alpha1.OrchestratorSpec {
		return corev1alpha1.OrchestratorSpec{
			Image:   "img",
			Runtime: corev1alpha1.RuntimeSandbox,
			Sandbox: sandbox,
			Tools:   corev1alpha1.OrchestratorToolsSpec{Enabled: true, Schedule: true},
		}
	}

	create := func(name string, labels map[string]string, spec corev1alpha1.OrchestratorSpec) *corev1alpha1.Orchestrator {
		orch := &corev1alpha1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: labels}, Spec: spec}
		Expect(k8sClient.Create(ctx, orch)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, orch))).To(Succeed()) })
		return orch
	}

	reconcile := func(name string, sandboxAvailable bool) ctrl.Result {
		reconciler := &OrchestratorReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), SandboxAvailable: sandboxAvailable}
		result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}})
		Expect(err).NotTo(HaveOccurred())
		return result
	}

	key := func(name string) types.NamespacedName { return types.NamespacedName{Name: name, Namespace: "default"} }

	getSandbox := func(name string) *unstructured.Unstructured {
		sandbox := newSandbox(&corev1alpha1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}})
		Expect(k8sClient.Get(ctx, key(name), sandbox)).To(Succeed())
		return sandbox
	}

	getOrchestrator := func(name string) *corev1alpha1.Orchestrator {
		orch := &corev1alpha1.Orchestrator{}
		Expect(k8sClient.Get(ctx, key(name), orch)).To(Succeed())
		return orch
	}

	readyCondition := func(orch *corev1alpha1.Orchestrator) metav1.Condition {
		for _, condition := range orch.Status.Conditions {
			if condition.Type == "Ready" {
				return condition
			}
		}
		Fail("the Orchestrator reports no Ready condition")
		return metav1.Condition{}
	}

	setSandboxStatus := func(sandbox *unstructured.Unstructured, conditions ...string) {
		Expect(unstructured.SetNestedSlice(sandbox.Object, trueConditions(conditions...), "status", "conditions")).To(Succeed())
		Expect(k8sClient.Status().Update(ctx, sandbox)).To(Succeed())
	}

	It("renders a Sandbox owned by the Orchestrator with its volumes, labels and own identity", func() {
		orch := create("sandboxed", map[string]string{"inference-gateway.com/workspace": "k8s-dev"}, sandboxSpec(nil))
		reconcile("sandboxed", true)

		sandbox := getSandbox("sandboxed")
		Expect(metav1.IsControlledBy(sandbox, orch)).To(BeTrue())
		Expect(sandbox.GetLabels()).To(HaveKeyWithValue("inference-gateway.com/workspace", "k8s-dev"))
		Expect(sandbox.Object["spec"]).To(SatisfyAll(
			HaveKeyWithValue("failedPodPolicy", "Recreate"),
			HaveKeyWithValue("operatingMode", "Running"),
		))

		claims, _, _ := unstructured.NestedSlice(sandbox.Object, "spec", "volumeClaimTemplates")
		Expect(claims).To(HaveLen(2))
		for i, volume := range []string{"workspace", "state"} {
			name, _, _ := unstructured.NestedString(claims[i].(map[string]any), "metadata", "name")
			storage, _, _ := unstructured.NestedString(claims[i].(map[string]any), "spec", "resources", "requests", "storage")
			Expect(name).To(Equal(volume))
			Expect(storage).To(Equal("5Gi"))
		}

		labels, _, _ := unstructured.NestedStringMap(sandbox.Object, "spec", "podTemplate", "metadata", "labels")
		Expect(labels).To(Equal(map[string]string{"app": "sandboxed", "inference-gateway.com/workspace": "k8s-dev"}))
		podSpec, _, _ := unstructured.NestedMap(sandbox.Object, "spec", "podTemplate", "spec")
		Expect(podSpec).To(SatisfyAll(
			HaveKeyWithValue("serviceAccountName", "sandboxed-daemon"),
			HaveKeyWithValue("restartPolicy", "OnFailure"),
		))
		containers, _, _ := unstructured.NestedSlice(podSpec, "containers")
		mounts, _, _ := unstructured.NestedSlice(containers[0].(map[string]any), "volumeMounts")
		Expect(mounts).To(ContainElements(
			HaveKeyWithValue("mountPath", "/workspace"),
			HaveKeyWithValue("mountPath", "/home/infer/.infer/projects"),
		))
		env, _, _ := unstructured.NestedSlice(containers[0].(map[string]any), "env")
		Expect(env).To(ContainElement(SatisfyAll(
			HaveKeyWithValue("name", "INFER_DAEMON_KUBERNETES_ORCHESTRATOR"),
			HaveKeyWithValue("value", "sandboxed"),
		)))

		role := &rbacv1.Role{}
		Expect(k8sClient.Get(ctx, key("sandboxed-daemon"), role)).To(Succeed())
		Expect(role.Rules).To(Equal([]rbacv1.PolicyRule{{
			APIGroups: []string{"core.inference-gateway.com"}, Resources: []string{"orchestrators"},
			ResourceNames: []string{"sandboxed"}, Verbs: []string{"get", "patch"},
		}}))
		binding := &rbacv1.RoleBinding{}
		Expect(k8sClient.Get(ctx, key("sandboxed-daemon"), binding)).To(Succeed())
		Expect(binding.Subjects).To(ConsistOf(HaveField("Name", "sandboxed-daemon")))
		Expect(k8sClient.Get(ctx, key("sandboxed-daemon"), &corev1.ServiceAccount{})).To(Succeed())

		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("sandboxed"), &appsv1.Deployment{}))).To(BeTrue())
	})

	It("sizes both volumes from spec.sandbox.storage", func() {
		storage := resource.MustParse("2Gi")
		create("sized", nil, sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{Storage: &storage}))
		reconcile("sized", true)

		claims, _, _ := unstructured.NestedSlice(getSandbox("sized").Object, "spec", "volumeClaimTemplates")
		for _, claim := range claims {
			size, _, _ := unstructured.NestedString(claim.(map[string]any), "spec", "resources", "requests", "storage")
			Expect(size).To(Equal("2Gi"))
		}
	})

	It("reports the Sandbox's readiness and the pod it runs once ready", func() {
		create("watched", nil, sandboxSpec(nil))
		reconcile("watched", true)
		orch := getOrchestrator("watched")
		Expect(readyCondition(orch).Reason).To(Equal("SandboxNotReady"))
		Expect(orch.Status.PodName).To(BeEmpty())

		setSandboxStatus(getSandbox("watched"), "Ready")
		reconcile("watched", true)
		orch = getOrchestrator("watched")
		Expect(orch.Status.Ready).To(BeTrue())
		Expect(readyCondition(orch).Reason).To(Equal("SandboxReady"))
		Expect(orch.Status.PodName).To(Equal("watched"), "agent-sandbox names the pod after the Sandbox")

		sandbox := getSandbox("watched")
		sandbox.SetAnnotations(map[string]string{"agents.x-k8s.io/pod-name": "watched-pod"})
		Expect(k8sClient.Update(ctx, sandbox)).To(Succeed())
		reconcile("watched", true)
		Expect(getOrchestrator("watched").Status.PodName).To(Equal("watched-pod"))
	})

	It("suspends the Sandbox, then resumes it once the suspension completed", func() {
		create("parked", nil, sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{OperatingMode: corev1alpha1.SandboxSuspended}))
		reconcile("parked", true)
		Expect(getSandbox("parked").Object["spec"]).To(HaveKeyWithValue("operatingMode", "Suspended"))
		Expect(readyCondition(getOrchestrator("parked")).Reason).To(Equal("SandboxSuspended"))

		orch := getOrchestrator("parked")
		orch.Spec.Sandbox.OperatingMode = corev1alpha1.SandboxRunning
		Expect(k8sClient.Update(ctx, orch)).To(Succeed())
		reconcile("parked", true)
		Expect(getSandbox("parked").Object["spec"]).To(HaveKeyWithValue("operatingMode", "Suspended"),
			"the Sandbox resumed before its suspension completed")

		setSandboxStatus(getSandbox("parked"), "Suspended")
		reconcile("parked", true)
		Expect(getSandbox("parked").Object["spec"]).To(HaveKeyWithValue("operatingMode", "Running"))
	})

	It("deletes the Orchestrator once its shutdown time passes, and requeues for one ahead", func() {
		ahead := metav1.NewTime(time.Now().Add(time.Hour))
		create("expiring", nil, sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{ShutdownTime: &ahead}))
		Expect(reconcile("expiring", true).RequeueAfter).To(BeNumerically("~", time.Hour, time.Minute))

		orch := getOrchestrator("expiring")
		passed := metav1.NewTime(time.Now().Add(-time.Minute))
		orch.Spec.Sandbox.ShutdownTime = &passed
		Expect(k8sClient.Update(ctx, orch)).To(Succeed())
		reconcile("expiring", true)
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("expiring"), &corev1alpha1.Orchestrator{}))).To(BeTrue())
	})

	It("reports SandboxCRDMissing and creates nothing when agent-sandbox is not installed", func() {
		create("no-agent-sandbox", nil, sandboxSpec(nil))
		reconcile("no-agent-sandbox", false)

		condition := readyCondition(getOrchestrator("no-agent-sandbox"))
		Expect(condition.Reason).To(Equal("SandboxCRDMissing"))
		Expect(condition.Message).To(ContainSubstring("agent-sandbox"))
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("no-agent-sandbox"), &appsv1.Deployment{}))).To(BeTrue())
		sandbox := newSandbox(&corev1alpha1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: "no-agent-sandbox", Namespace: "default"}})
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("no-agent-sandbox"), sandbox))).To(BeTrue())
	})

	It("replaces the workload when the runtime changes", func() {
		spec := sandboxSpec(nil)
		spec.Runtime = corev1alpha1.RuntimeDeployment
		create("switching", nil, spec)
		reconcile("switching", true)
		Expect(k8sClient.Get(ctx, key("switching"), &appsv1.Deployment{})).To(Succeed())

		orch := getOrchestrator("switching")
		orch.Spec.Runtime = corev1alpha1.RuntimeSandbox
		Expect(k8sClient.Update(ctx, orch)).To(Succeed())
		reconcile("switching", true)
		getSandbox("switching")
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("switching"), &appsv1.Deployment{}))).To(BeTrue())

		orch = getOrchestrator("switching")
		orch.Spec.Runtime = corev1alpha1.RuntimeDeployment
		Expect(k8sClient.Update(ctx, orch)).To(Succeed())
		reconcile("switching", true)
		Expect(k8sClient.Get(ctx, key("switching"), &appsv1.Deployment{})).To(Succeed())
		sandbox := newSandbox(orch)
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("switching"), sandbox))).To(BeTrue())
	})

	It("rejects spec.sandbox without runtime: Sandbox", func() {
		spec := sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{})
		spec.Runtime = corev1alpha1.RuntimeDeployment
		err := k8sClient.Create(ctx, &corev1alpha1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: "misplaced", Namespace: "default"}, Spec: spec})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.sandbox needs spec.runtime: Sandbox"))
	})
})

var _ = DescribeTable("sandboxOperatingMode",
	func(spec *corev1alpha1.OrchestratorSandboxSpec, mode string, conditions []string, expected corev1alpha1.SandboxOperatingMode) {
		var found *unstructured.Unstructured
		if mode != "" {
			found = newSandbox(nil)
			found.Object["spec"] = map[string]any{"operatingMode": mode}
			found.Object["status"] = map[string]any{"conditions": trueConditions(conditions...)}
		}
		Expect(sandboxOperatingMode(spec, found)).To(Equal(expected))
	},
	Entry("a new Sandbox runs", nil, "", nil, corev1alpha1.SandboxRunning),
	Entry("the spec suspends it", &corev1alpha1.OrchestratorSandboxSpec{OperatingMode: corev1alpha1.SandboxSuspended}, "Running", []string{"Ready"}, corev1alpha1.SandboxSuspended),
	Entry("a ready Sandbox keeps running", nil, "Running", []string{"Ready"}, corev1alpha1.SandboxRunning),
	Entry("a finished pod is cycled through Suspended", nil, "Running", []string{"Finished"}, corev1alpha1.SandboxSuspended),
	Entry("a suspension in progress completes first", nil, "Suspended", nil, corev1alpha1.SandboxSuspended),
	Entry("a completed suspension resumes", nil, "Suspended", []string{"Suspended"}, corev1alpha1.SandboxRunning),
)

// trueConditions renders Sandbox status conditions of the given types, all True.
func trueConditions(conditionTypes ...string) []any {
	conditions := make([]any, 0, len(conditionTypes))
	for _, conditionType := range conditionTypes {
		conditions = append(conditions, map[string]any{
			"type": conditionType, "status": "True", "reason": "Test", "message": "set by the test",
			"lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
		})
	}
	return conditions
}
