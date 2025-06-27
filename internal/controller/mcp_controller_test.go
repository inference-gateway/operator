/*
Copyright (c) 2025 Inference Gateway

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/

package controller

import (
	"context"
	"regexp"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

var _ = Describe("MCP Controller", func() {
	Context("Reconciliation scenarios", func() {
		type testCase struct {
			name           string
			setup          func(ctx context.Context, nn types.NamespacedName)
			expectError    bool
			expectNotFound bool
		}

		ctx := context.Background()

		cases := []testCase{
			{
				name: "resource exists and reconciles successfully",
				setup: func(ctx context.Context, nn types.NamespacedName) {
					resource := &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nn.Name,
							Namespace: nn.Namespace,
						},
					}
					Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				},
				expectError:    false,
				expectNotFound: false,
			},
			{
				name:           "resource does not exist",
				setup:          func(ctx context.Context, nn types.NamespacedName) {},
				expectError:    false,
				expectNotFound: true,
			},
			{
				name: "resource marked for deletion",
				setup: func(ctx context.Context, nn types.NamespacedName) {
					resource := &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:              nn.Name,
							Namespace:         nn.Namespace,
							DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
						},
					}
					Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				},
				expectError:    false,
				expectNotFound: false,
			},
			{
				name: "resource with invalid spec",
				setup: func(ctx context.Context, nn types.NamespacedName) {
					resource := &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nn.Name,
							Namespace: nn.Namespace,
						},
						Spec: v1alpha1.MCPSpec{},
					}
					Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				},
				expectError:    false,
				expectNotFound: false,
			},
			{
				name: "resource already deleted before reconcile",
				setup: func(ctx context.Context, nn types.NamespacedName) {
					resource := &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nn.Name,
							Namespace: nn.Namespace,
						},
					}
					Expect(k8sClient.Create(ctx, resource)).To(Succeed())
					Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
				},
				expectError:    false,
				expectNotFound: true,
			},
			{
				name: "resource with finalizer",
				setup: func(ctx context.Context, nn types.NamespacedName) {
					resource := &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:       nn.Name,
							Namespace:  nn.Namespace,
							Finalizers: []string{"test.core.finalizer/mcp"},
						},
					}
					Expect(k8sClient.Create(ctx, resource)).To(Succeed())
				},
				expectError:    false,
				expectNotFound: false,
			},
		}

		for _, tc := range cases {
			It("should handle "+tc.name, func() {
				safeName := func(s string) string {
					name := "test-" + s
					name = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(
						strings.ToLower(name), "-")
					if len(name) > 63 {
						name = name[:63]
					}
					name = strings.Trim(name, "-")
					if name == "" {
						name = "test"
					}
					return name
				}

				nn := types.NamespacedName{
					Name:      "test-" + safeName(tc.name),
					Namespace: "default",
				}

				tc.setup(ctx, nn)

				controllerReconciler := &MCPReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: nn,
				})

				switch {
				case tc.expectError:
					Expect(err).To(HaveOccurred())
					return
				default:
					Expect(err).NotTo(HaveOccurred())
				}

				resource := &v1alpha1.MCP{}
				getErr := k8sClient.Get(ctx, nn, resource)
				switch {
				case tc.expectNotFound:
					Expect(errors.IsNotFound(getErr)).To(BeTrue())
					return
				default:
					Expect(getErr).NotTo(HaveOccurred())
				}

				_ = k8sClient.Delete(ctx, resource)
			})
		}
	})
})
