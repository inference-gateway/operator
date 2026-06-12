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
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
	testutil "github.com/inference-gateway/operator/internal/controller/testutil"
)

// These specs cover the issue #75 acceptance criteria that the existing suite
// did not yet assert: that a minimal Gateway produces a Service + ServiceAccount
// (alongside the already-covered Deployment), and a negative path where an
// invalid spec surfaces a status condition and leaves no partial resources.
// They use the shared testutil fixture builders to demonstrate reuse across
// controllers.

var _ = Describe("Gateway core resource generation", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	ctx := context.Background()

	It("creates a Deployment, Service and ServiceAccount for a minimal Gateway", func() {
		gateway := testutil.NewGateway("minimal-gw", "default")
		Expect(k8sClient.Create(ctx, gateway)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, gateway) })

		key := types.NamespacedName{Name: gateway.Name, Namespace: gateway.Namespace}

		By("creating the Deployment")
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &appsv1.Deployment{})
		}, timeout, interval).Should(Succeed())

		By("creating the Service")
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &corev1.Service{})
		}, timeout, interval).Should(Succeed())

		By("creating the ServiceAccount")
		Eventually(func() error {
			return k8sClient.Get(ctx, key, &corev1.ServiceAccount{})
		}, timeout, interval).Should(Succeed())

		By("recording the ServiceAccount name in status")
		Eventually(func() string {
			updated := &corev1alpha1.Gateway{}
			if err := k8sClient.Get(ctx, key, updated); err != nil {
				return ""
			}
			return updated.Status.ServiceAccountName
		}, timeout, interval).Should(Equal(gateway.Name))
	})
})

var _ = Describe("Agent invalid spec handling", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	ctx := context.Background()

	It("sets Ready=False/InvalidSpec and creates no child resources when image is empty", func() {
		agent := testutil.NewAgent("invalid-agent", "default", testutil.WithAgentImage(""))
		Expect(k8sClient.Create(ctx, agent)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, agent) })

		key := types.NamespacedName{Name: agent.Name, Namespace: agent.Namespace}
		reconciler := &AgentReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

		_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: key})
		Expect(err).NotTo(HaveOccurred())

		By("surfacing a Ready=False / InvalidSpec condition with a message")
		updated := &corev1alpha1.Agent{}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, updated)).To(Succeed())
			cond := apimeta.FindStatusCondition(updated.Status.Conditions, "Ready")
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal("InvalidSpec"))
			g.Expect(cond.Message).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		By("leaving no Deployment or Service behind")
		Consistently(func() bool {
			depErr := k8sClient.Get(ctx, key, &appsv1.Deployment{})
			svcErr := k8sClient.Get(ctx, key, &corev1.Service{})
			return apierrors.IsNotFound(depErr) && apierrors.IsNotFound(svcErr)
		}, time.Second*2, interval).Should(BeTrue())
	})
})
