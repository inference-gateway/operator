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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
	testutil "github.com/inference-gateway/operator/internal/controller/testutil"
)

var _ = Describe("Namespace watch", func() {
	ctx := context.Background()

	It("reconciles MCPs created before their namespace was labelled", func() {
		Expect(os.Setenv("WATCH_NAMESPACE_SELECTOR", "inference-gateway.com/managed=true")).To(Succeed())
		DeferCleanup(func() { _ = os.Unsetenv("WATCH_NAMESPACE_SELECTOR") })

		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "late-label"}}
		mcp := testutil.NewMCP("example", ns.Name)
		mcp.Spec.Image = "ghcr.io/inference-gateway/mcp:latest"
		c := testutil.NewFakeClient(ns, mcp)
		r := &MCPReconciler{Client: c, Scheme: testutil.Scheme()}
		key := types.NamespacedName{Name: mcp.Name, Namespace: mcp.Namespace}

		By("skipping the MCP while the namespace lacks the label")
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		deployment := &appsv1.Deployment{}
		Expect(apierrors.IsNotFound(c.Get(ctx, key, deployment))).To(BeTrue())

		By("enqueuing nothing for an unlabelled namespace")
		Expect(namespaceRequests(ctx, c, ns)).To(BeEmpty())

		By("labelling the namespace")
		ns.Labels = map[string]string{"inference-gateway.com/managed": "true"}
		Expect(c.Update(ctx, ns)).To(Succeed())

		By("enqueuing the MCP that was skipped")
		Expect(namespaceRequests(ctx, c, ns)).To(ConsistOf(ctrl.Request{NamespacedName: key}))

		By("reconciling it into a Deployment")
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())
		Expect(c.Get(ctx, key, deployment)).To(Succeed())
	})

	It("ignores namespaces of a different list type", func() {
		Expect(os.Setenv("WATCH_NAMESPACE_SELECTOR", "inference-gateway.com/managed=true")).To(Succeed())
		DeferCleanup(func() { _ = os.Unsetenv("WATCH_NAMESPACE_SELECTOR") })

		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "other",
				Labels: map[string]string{"inference-gateway.com/managed": "true"},
			},
		}
		c := testutil.NewFakeClient(ns, testutil.NewMCP("example", "elsewhere"))
		Expect(namespaceRequests(ctx, c, ns)).To(BeEmpty())
	})
})

// namespaceRequests exercises the map func behind the MCP Namespace watch.
func namespaceRequests(ctx context.Context, c client.Client, ns *corev1.Namespace) []ctrl.Request {
	mapper := namespaceMapper(c, func() client.ObjectList { return &corev1alpha1.MCPList{} })
	return mapper(ctx, ns)
}
