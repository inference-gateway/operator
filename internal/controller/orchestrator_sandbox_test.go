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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	unstructured "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	types "k8s.io/apimachinery/pkg/types"
	ptr "k8s.io/utils/ptr"
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

	key := func(name string) types.NamespacedName { return types.NamespacedName{Name: name, Namespace: "default"} }

	create := func(name string, labels map[string]string, spec corev1alpha1.OrchestratorSpec) *corev1alpha1.Orchestrator {
		orch := &corev1alpha1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: labels}, Spec: spec}
		Expect(k8sClient.Create(ctx, orch)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, orch))).To(Succeed()) })
		return orch
	}

	reconcileWith := func(name string, sandboxAvailable bool) ctrl.Result {
		reconciler := &OrchestratorReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), SandboxAvailable: sandboxAvailable, PodReader: k8sClient}
		result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: key(name)})
		Expect(err).NotTo(HaveOccurred())
		return result
	}
	reconcile := func(name string) ctrl.Result { return reconcileWith(name, true) }

	getSandbox := func(name string) *unstructured.Unstructured {
		sandbox := sandboxKind()
		Expect(k8sClient.Get(ctx, key(name), sandbox)).To(Succeed())
		return sandbox
	}

	getOrchestrator := func(name string) *corev1alpha1.Orchestrator {
		orch := &corev1alpha1.Orchestrator{}
		Expect(k8sClient.Get(ctx, key(name), orch)).To(Succeed())
		return orch
	}

	updateOrchestrator := func(name string, mutate func(*corev1alpha1.OrchestratorSpec)) {
		orch := getOrchestrator(name)
		mutate(&orch.Spec)
		Expect(k8sClient.Update(ctx, orch)).To(Succeed())
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

	reportSandbox := func(name string, conditions ...map[string]any) {
		sandbox := getSandbox(name)
		for _, condition := range conditions {
			condition["observedGeneration"] = sandbox.GetGeneration()
			condition["lastTransitionTime"] = time.Now().UTC().Format(time.RFC3339)
			if _, ok := condition["reason"]; !ok {
				condition["reason"] = "Test"
			}
			condition["message"] = "reported by the test"
		}
		list := make([]any, 0, len(conditions))
		for _, condition := range conditions {
			list = append(list, condition)
		}
		Expect(unstructured.SetNestedSlice(sandbox.Object, list, "status", "conditions")).To(Succeed())
		Expect(k8sClient.Status().Update(ctx, sandbox)).To(Succeed())
	}
	trueCondition := func(conditionType string) map[string]any {
		return map[string]any{"type": conditionType, "status": "True"}
	}

	sandboxSpecField := func(name, field string) any {
		value, _, _ := unstructured.NestedFieldNoCopy(getSandbox(name).Object, "spec", field)
		return value
	}

	It("renders a Sandbox owned by the Orchestrator whose pod has no API access and no privileges", func() {
		orch := create("sandboxed", map[string]string{"inference-gateway.com/workspace": "k8s-dev"},
			sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{RuntimeClassName: ptr.To("gvisor")}))
		reconcile("sandboxed")

		sandbox := getSandbox("sandboxed")
		Expect(metav1.IsControlledBy(sandbox, orch)).To(BeTrue())
		Expect(sandbox.GetLabels()).To(HaveKeyWithValue("inference-gateway.com/workspace", "k8s-dev"))
		Expect(sandbox.Object["spec"]).To(SatisfyAll(
			HaveKeyWithValue("failedPodPolicy", "Recreate"),
			HaveKeyWithValue("operatingMode", "Running"),
			Not(HaveKey("shutdownTime")),
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
			HaveKeyWithValue("automountServiceAccountToken", false),
			HaveKeyWithValue("restartPolicy", "OnFailure"),
			HaveKeyWithValue("runtimeClassName", "gvisor"),
			Not(HaveKey("serviceAccountName")),
		))
		Expect(podSpec["securityContext"]).To(SatisfyAll(
			HaveKeyWithValue("runAsNonRoot", true),
			HaveKeyWithValue("runAsUser", int64(1000)),
			HaveKeyWithValue("seccompProfile", HaveKeyWithValue("type", "RuntimeDefault")),
		))
		containers, _, _ := unstructured.NestedSlice(podSpec, "containers")
		container := containers[0].(map[string]any)
		Expect(container["securityContext"]).To(SatisfyAll(
			HaveKeyWithValue("allowPrivilegeEscalation", false),
			HaveKeyWithValue("capabilities", HaveKeyWithValue("drop", ConsistOf("ALL"))),
		))
		mounts, _, _ := unstructured.NestedSlice(container, "volumeMounts")
		Expect(mounts).To(ContainElements(
			HaveKeyWithValue("mountPath", "/workspace"),
			HaveKeyWithValue("mountPath", "/home/infer/.infer/projects"),
		))

		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("sandboxed"), &appsv1.Deployment{}))).To(BeTrue())
	})

	It("sizes both volumes from spec.sandbox.storage, which is fixed afterwards", func() {
		storage := resource.MustParse("2Gi")
		create("sized", nil, sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{Storage: &storage}))
		reconcile("sized")

		claims, _, _ := unstructured.NestedSlice(getSandbox("sized").Object, "spec", "volumeClaimTemplates")
		for _, claim := range claims {
			size, _, _ := unstructured.NestedString(claim.(map[string]any), "spec", "resources", "requests", "storage")
			Expect(size).To(Equal("2Gi"))
		}

		orch := getOrchestrator("sized")
		bigger := resource.MustParse("4Gi")
		orch.Spec.Sandbox.Storage = &bigger
		err := k8sClient.Update(ctx, orch)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.sandbox.storage is fixed"))
	})

	It("reports readiness for the Sandbox's current generation, its pod, and why it is not ready", func() {
		create("watched", nil, sandboxSpec(nil))
		reconcile("watched")
		orch := getOrchestrator("watched")
		Expect(readyCondition(orch).Reason).To(Equal("SandboxNotReady"))
		Expect(orch.Status.PodName).To(BeEmpty())

		reportSandbox("watched", map[string]any{"type": "Ready", "status": "False", "reason": "ImagePullBackOff"})
		reconcile("watched")
		Expect(readyCondition(getOrchestrator("watched")).Message).To(ContainSubstring("ImagePullBackOff"))

		reportSandbox("watched", trueCondition("Ready"))
		reconcile("watched")
		orch = getOrchestrator("watched")
		Expect(orch.Status.Ready).To(BeTrue())
		Expect(readyCondition(orch).Reason).To(Equal("SandboxReady"))
		Expect(orch.Status.PodName).To(Equal("watched"), "agent-sandbox names the pod after the Sandbox")

		updateOrchestrator("watched", func(spec *corev1alpha1.OrchestratorSpec) {
			spec.Env = &[]corev1.EnvVar{{Name: "EXTRA", Value: "1"}}
		})
		reconcile("watched")
		Expect(getOrchestrator("watched").Status.Ready).To(BeFalse(),
			"a Ready condition about the previous spec does not vouch for the new one")
	})

	It("cycles a running pod through Suspended when its template changes", func() {
		create("rolling", nil, sandboxSpec(nil))
		reconcile("rolling")
		firstHash := getSandbox("rolling").GetAnnotations()["inference-gateway.com/pod-template-hash"]
		reportSandbox("rolling", trueCondition("Ready"))

		updateOrchestrator("rolling", func(spec *corev1alpha1.OrchestratorSpec) {
			spec.Env = &[]corev1.EnvVar{{Name: "EXTRA", Value: "1"}}
		})
		reconcile("rolling")
		Expect(sandboxSpecField("rolling", "operatingMode")).To(Equal("Suspended"))
		Expect(getSandbox("rolling").GetAnnotations()).To(HaveKeyWithValue("inference-gateway.com/pod-template-hash", firstHash))

		reportSandbox("rolling", trueCondition("Suspended"))
		reconcile("rolling")
		sandbox := getSandbox("rolling")
		Expect(sandbox.Object["spec"]).To(HaveKeyWithValue("operatingMode", "Running"))
		Expect(sandbox.GetAnnotations()["inference-gateway.com/pod-template-hash"]).NotTo(Equal(firstHash))
	})

	It("suspends the Sandbox, then resumes it once the suspension completed", func() {
		create("parked", nil, sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{OperatingMode: corev1alpha1.SandboxSuspended}))
		reconcile("parked")
		Expect(sandboxSpecField("parked", "operatingMode")).To(Equal("Suspended"))
		Expect(readyCondition(getOrchestrator("parked")).Reason).To(Equal("SandboxSuspended"))

		updateOrchestrator("parked", func(spec *corev1alpha1.OrchestratorSpec) {
			spec.Sandbox.OperatingMode = corev1alpha1.SandboxRunning
		})
		reconcile("parked")
		Expect(sandboxSpecField("parked", "operatingMode")).To(Equal("Suspended"), "the Sandbox resumed before its suspension completed")

		reportSandbox("parked", trueCondition("Suspended"))
		reconcile("parked")
		Expect(sandboxSpecField("parked", "operatingMode")).To(Equal("Running"))
	})

	It("parks the Orchestrator when its daemon idles out, and starts the retention once every agent is done", func() {
		create("idle", nil, sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{Retention: &metav1.Duration{Duration: time.Hour}}))
		reconcile("idle")

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "idle", Namespace: "default"},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "orchestrator", Image: "img"}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func() { Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, pod))).To(Succeed()) })
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name:  "orchestrator",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Message: "agents-done=true"}},
		}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		reportSandbox("idle", map[string]any{"type": "Finished", "status": "True", "reason": "PodSucceeded"})

		reconcile("idle")
		Expect(getOrchestrator("idle").Spec.Sandbox.OperatingMode).To(Equal(corev1alpha1.SandboxSuspended))
		sandbox := getSandbox("idle")
		Expect(sandbox.GetAnnotations()).To(SatisfyAll(
			HaveKeyWithValue("inference-gateway.com/agents-done", "true"),
			HaveKey("inference-gateway.com/idle-since"),
		))
		Expect(sandbox.Object["spec"]).To(SatisfyAll(
			HaveKeyWithValue("operatingMode", "Suspended"),
			HaveKeyWithValue("shutdownPolicy", "Retain"),
		))
		shutdown, err := time.Parse(time.RFC3339, sandboxSpecField("idle", "shutdownTime").(string))
		Expect(err).NotTo(HaveOccurred())
		Expect(shutdown).To(BeTemporally("~", time.Now().Add(time.Hour), time.Minute))

		updateOrchestrator("idle", func(spec *corev1alpha1.OrchestratorSpec) {
			spec.Sandbox.OperatingMode = corev1alpha1.SandboxRunning
		})
		reportSandbox("idle", trueCondition("Suspended"))
		reconcile("idle")
		sandbox = getSandbox("idle")
		Expect(sandbox.Object["spec"]).To(SatisfyAll(
			HaveKeyWithValue("operatingMode", "Running"),
			Not(HaveKey("shutdownTime")),
		))
		Expect(sandbox.GetAnnotations()).NotTo(HaveKey("inference-gateway.com/idle-since"))
	})

	It("expires the Sandbox at spec.sandbox.shutdownTime and deletes the Orchestrator once it reports so", func() {
		ahead := metav1.NewTime(time.Now().Add(time.Hour).Truncate(time.Second))
		create("expiring", nil, sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{ShutdownTime: &ahead}))
		Expect(reconcile("expiring").RequeueAfter).To(BeNumerically("~", time.Hour, time.Minute))
		Expect(sandboxSpecField("expiring", "shutdownTime")).To(Equal(ahead.UTC().Format(time.RFC3339)))
		Expect(sandboxSpecField("expiring", "shutdownPolicy")).To(Equal("Retain"))

		reportSandbox("expiring", map[string]any{"type": "Ready", "status": "False", "reason": "SandboxExpired"})
		reconcile("expiring")
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("expiring"), &corev1alpha1.Orchestrator{}))).To(BeTrue())
	})

	It("deletes the Orchestrator once spec.sandbox.shutdownTime passed, even with nothing to run", func() {
		passed := metav1.NewTime(time.Now().Add(-time.Minute))
		spec := sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{ShutdownTime: &passed})
		spec.Tools = corev1alpha1.OrchestratorToolsSpec{}
		create("overdue", nil, spec)
		reconcile("overdue")
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("overdue"), &corev1alpha1.Orchestrator{}))).To(BeTrue())
	})

	It("reports SandboxCRDMissing and creates nothing when agent-sandbox is not installed", func() {
		create("no-agent-sandbox", nil, sandboxSpec(nil))
		reconcileWith("no-agent-sandbox", false)

		condition := readyCondition(getOrchestrator("no-agent-sandbox"))
		Expect(condition.Reason).To(Equal("SandboxCRDMissing"))
		Expect(condition.Message).To(ContainSubstring("agent-sandbox"))
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("no-agent-sandbox"), &appsv1.Deployment{}))).To(BeTrue())
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key("no-agent-sandbox"), sandboxKind()))).To(BeTrue())
	})

	It("rejects changing spec.runtime, since a switch would drop the Sandbox's volumes", func() {
		create("fixed-runtime", nil, sandboxSpec(nil))
		orch := getOrchestrator("fixed-runtime")
		orch.Spec.Runtime = corev1alpha1.RuntimeDeployment
		err := k8sClient.Update(ctx, orch)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.runtime is fixed"))
	})

	It("rejects spec.sandbox without runtime: Sandbox", func() {
		spec := sandboxSpec(&corev1alpha1.OrchestratorSandboxSpec{})
		spec.Runtime = corev1alpha1.RuntimeDeployment
		err := k8sClient.Create(ctx, &corev1alpha1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: "misplaced", Namespace: "default"}, Spec: spec})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("spec.sandbox needs spec.runtime: Sandbox"))
	})
})

var _ = DescribeTable("sandboxRollout",
	func(suspend bool, mode, recordedHash string, conditions []string, expectedMode, expectedHash string) {
		sandbox := sandboxKind()
		sandbox.SetAnnotations(map[string]string{podTemplateHashAnnotation: recordedHash})
		if mode != "" {
			sandbox.Object["spec"] = map[string]any{"operatingMode": mode}
		}
		list := make([]any, 0, len(conditions))
		for _, conditionType := range conditions {
			list = append(list, map[string]any{"type": conditionType, "status": "True"})
		}
		sandbox.Object["status"] = map[string]any{"conditions": list}

		gotMode, gotHash := sandboxRollout(suspend, sandbox, "new")
		Expect(gotMode).To(Equal(expectedMode))
		Expect(gotHash).To(Equal(expectedHash))
	},
	Entry("a new Sandbox runs the current template", false, "", "", nil, "Running", "new"),
	Entry("a new suspended Sandbox records the current template", true, "", "", nil, "Suspended", "new"),
	Entry("the spec suspends a running Sandbox", true, "Running", "new", []string{"Ready"}, "Suspended", "new"),
	Entry("a ready Sandbox on the current template keeps running", false, "Running", "new", []string{"Ready"}, "Running", "new"),
	Entry("a changed template cycles the pod through Suspended", false, "Running", "old", []string{"Ready"}, "Suspended", "old"),
	Entry("a suspension in progress completes first", false, "Suspended", "old", nil, "Suspended", "old"),
	Entry("a completed suspension resumes on the current template", false, "Suspended", "old", []string{"Suspended"}, "Running", "new"),
)

var _ = DescribeTable("sandboxShutdownTime",
	func(shutdown, retention time.Duration, idle, agentsDone bool, expected time.Duration) {
		now := time.Now().Truncate(time.Second)
		spec := &corev1alpha1.OrchestratorSandboxSpec{}
		if shutdown > 0 {
			spec.ShutdownTime = &metav1.Time{Time: now.Add(shutdown)}
		}
		if retention > 0 {
			spec.Retention = &metav1.Duration{Duration: retention}
		}
		annotations := map[string]string{}
		if idle {
			annotations[idleSinceAnnotation] = now.UTC().Format(time.RFC3339)
			annotations[agentsDoneAnnotation] = map[bool]string{true: "true", false: "false"}[agentsDone]
		}

		got := sandboxShutdownTime(spec, annotations)
		if expected == 0 {
			Expect(got).To(BeNil())
			return
		}
		Expect(got).NotTo(BeNil())
		Expect(*got).To(BeTemporally("==", now.Add(expected)))
	},
	Entry("never without a shutdown time or an idle daemon", time.Duration(0), time.Hour, false, false, time.Duration(0)),
	Entry("at spec.sandbox.shutdownTime", 2*time.Hour, time.Duration(0), false, false, 2*time.Hour),
	Entry("a retention after the daemon idled out with every agent done", time.Duration(0), time.Hour, true, true, time.Hour),
	Entry("no retention while an agent is not done", time.Duration(0), time.Hour, true, false, time.Duration(0)),
	Entry("whichever comes first", 30*time.Minute, time.Hour, true, true, 30*time.Minute),
)
