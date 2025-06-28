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

	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	types "k8s.io/apimachinery/pkg/types"
	reconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

var _ = Describe("MCP Controller", func() {
	Context("Reconciliation scenarios", func() {
		type testCase struct {
			setup          func(ctx context.Context, nn types.NamespacedName)
			expectError    bool
			expectNotFound bool
		}

		ctx := context.Background()

		DescribeTable("should handle various MCP reconciliation cases",
			func(name string, tc testCase) {
				safeName := func(s string) string {
					name := "test-" + s
					name = regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(strings.ToLower(name), "-")
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
					Name:      "test-" + safeName(name),
					Namespace: "default",
				}

				tc.setup(ctx, nn)

				reconciler := &MCPReconciler{
					Client: k8sClient,
					Scheme: k8sClient.Scheme(),
				}

				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: nn})

				if tc.expectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
				}

				resource := &v1alpha1.MCP{}
				getErr := k8sClient.Get(ctx, nn, resource)

				if tc.expectNotFound {
					Expect(errors.IsNotFound(getErr)).To(BeTrue())
				} else {
					Expect(getErr).NotTo(HaveOccurred())
				}

				_ = k8sClient.Delete(ctx, resource)
			},

			Entry("resource exists and reconciles successfully", "exists", testCase{
				setup: func(ctx context.Context, nn types.NamespacedName) {
					Expect(k8sClient.Create(ctx, &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nn.Name,
							Namespace: nn.Namespace,
						},
					})).To(Succeed())
				},
			}),

			Entry("resource does not exist", "missing", testCase{
				setup:          func(ctx context.Context, nn types.NamespacedName) {},
				expectNotFound: true,
			}),

			Entry("resource marked for deletion", "deleting", testCase{
				setup: func(ctx context.Context, nn types.NamespacedName) {
					Expect(k8sClient.Create(ctx, &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:              nn.Name,
							Namespace:         nn.Namespace,
							DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
						},
					})).To(Succeed())
				},
			}),

			Entry("resource with invalid spec", "invalid-spec", testCase{
				setup: func(ctx context.Context, nn types.NamespacedName) {
					Expect(k8sClient.Create(ctx, &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nn.Name,
							Namespace: nn.Namespace,
						},
						Spec: v1alpha1.MCPSpec{},
					})).To(Succeed())
				},
			}),

			Entry("resource already deleted before reconcile", "already-deleted", testCase{
				setup: func(ctx context.Context, nn types.NamespacedName) {
					mcp := &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      nn.Name,
							Namespace: nn.Namespace,
						},
					}
					Expect(k8sClient.Create(ctx, mcp)).To(Succeed())
					Expect(k8sClient.Delete(ctx, mcp)).To(Succeed())
				},
				expectNotFound: true,
			}),

			Entry("resource with finalizer", "has-finalizer", testCase{
				setup: func(ctx context.Context, nn types.NamespacedName) {
					Expect(k8sClient.Create(ctx, &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:       nn.Name,
							Namespace:  nn.Namespace,
							Finalizers: []string{"test.core.finalizer/mcp"},
						},
					})).To(Succeed())
				},
			}),
		)
	})
})

var _ = Describe("MCPReconciler", func() {
	Context("reconcileHPA", func() {
		var (
			ctx        context.Context
			reconciler *MCPReconciler
			baseMCP    *v1alpha1.MCP
		)

		BeforeEach(func() {
			ctx = context.Background()
			reconciler = &MCPReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}

			baseMCP = &v1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mcp",
					Namespace: "default",
				},
				Spec: v1alpha1.MCPSpec{},
			}
		})

		DescribeTable("HPA reconciliation cases",
			func(modifyMCP func(m *v1alpha1.MCP), deploymentFunc func() *appsv1.Deployment, expectNil, expectErr, expectDefaults bool) {
				mcp := baseMCP.DeepCopy()
				if modifyMCP != nil {
					modifyMCP(mcp)
				}

				Expect(k8sClient.Create(ctx, mcp)).To(Succeed())
				DeferCleanup(func() { _ = k8sClient.Delete(ctx, mcp) })

				var deployment *appsv1.Deployment
				if deploymentFunc != nil {
					deployment = deploymentFunc()
				}

				hpa, err := reconciler.reconcileHPA(ctx, mcp, deployment)

				if expectErr {
					Expect(err).To(HaveOccurred())
					Expect(hpa).To(BeNil())
					return
				}

				Expect(err).NotTo(HaveOccurred())

				if expectNil {
					Expect(hpa).To(BeNil())
				} else {
					Expect(hpa).NotTo(BeNil())
					Expect(hpa.Spec.ScaleTargetRef.Name).To(Equal("test-mcp"))
					if expectDefaults {
						Expect(hpa.Spec.MinReplicas).NotTo(BeNil())
						Expect(*hpa.Spec.MinReplicas).To(Equal(int32(1)))
						Expect(hpa.Spec.MaxReplicas).To(Equal(int32(10)))
						Expect(hpa.Spec.Metrics).To(HaveLen(1))
						Expect(hpa.Spec.Metrics[0].Type).To(Equal(autoscalingv2.ResourceMetricSourceType))
						Expect(hpa.Spec.Metrics[0].Resource.Name).To(Equal(corev1.ResourceCPU))
					}
				}
			},

			Entry("returns nil if HPA spec is nil", nil, nil, true, false, false),

			Entry("returns nil if HPA is disabled",
				func(m *v1alpha1.MCP) {
					m.Spec.HPA = &v1alpha1.HPASpec{Enabled: false}
				}, nil, true, false, false),

			Entry("returns error if deployment is nil and HPA is enabled",
				func(m *v1alpha1.MCP) {
					m.Spec.Image = "test:image"
					m.Spec.HPA = &v1alpha1.HPASpec{Enabled: true}
				}, nil, false, true, false),

			Entry("returns nil if HPA is enabled but no image is set (deployment would be nil)",
				func(m *v1alpha1.MCP) {
					m.Spec.HPA = &v1alpha1.HPASpec{Enabled: true}
				}, nil, false, true, false),

			Entry("uses default HPA config if config is nil",
				func(m *v1alpha1.MCP) {
					m.Spec.Image = "test:image"
					m.Spec.HPA = &v1alpha1.HPASpec{Enabled: true, Config: nil}
				}, func() *appsv1.Deployment {
					return &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-mcp",
							Namespace: "default",
						},
					}
				}, false, false, true),
		)
	})

	Context("Service reconciliation", func() {
		var (
			ctx        context.Context
			reconciler *MCPReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			reconciler = &MCPReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		DescribeTable("reconcileService scenarios",
			func(mcpFunc func() *v1alpha1.MCP, expectErr bool, expectNil bool, validateService func(*corev1.Service)) {
				mcp := mcpFunc()
				DeferCleanup(func() {
					if mcp != nil {
						_ = k8sClient.Delete(ctx, mcp)
					}
				})

				service, err := reconciler.reconcileService(ctx, mcp)

				if expectErr {
					Expect(err).To(HaveOccurred())
					Expect(service).To(BeNil())
					return
				}

				Expect(err).NotTo(HaveOccurred())

				if expectNil {
					Expect(service).To(BeNil())
				} else {
					Expect(service).NotTo(BeNil())
					if validateService != nil {
						validateService(service)
					}
				}
			},

			Entry("creates service with default port for basic MCP",
				func() *v1alpha1.MCP {
					mcp := &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-service-basic",
							Namespace: "default",
						},
						Spec: v1alpha1.MCPSpec{
							Image: "test:image",
						},
					}
					Expect(k8sClient.Create(ctx, mcp)).To(Succeed())
					return mcp
				}, false, false, func(service *corev1.Service) {
					Expect(service.Name).To(Equal("test-service-basic-service"))
					Expect(service.Namespace).To(Equal("default"))
					Expect(service.Spec.Ports).To(HaveLen(1))
					Expect(service.Spec.Ports[0].Port).To(Equal(int32(3000)))
					Expect(service.Spec.Ports[0].Name).To(Equal("http"))
					Expect(service.Spec.Selector).To(HaveKeyWithValue("app", "test-service-basic"))
				}),

			Entry("creates service with custom port",
				func() *v1alpha1.MCP {
					mcp := &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-service-custom-port",
							Namespace: "default",
						},
						Spec: v1alpha1.MCPSpec{
							Image: "test:image",
							Server: &v1alpha1.MCPServerSpec{
								Port: 8080,
							},
						},
					}
					Expect(k8sClient.Create(ctx, mcp)).To(Succeed())
					return mcp
				}, false, false, func(service *corev1.Service) {
					Expect(service.Spec.Ports[0].Port).To(Equal(int32(8080)))
				}),

			Entry("returns error for nil MCP",
				func() *v1alpha1.MCP {
					return nil
				}, true, true, nil),
		)

		DescribeTable("buildServiceURL scenarios",
			func(mcpFunc func() *v1alpha1.MCP, serviceFunc func() *corev1.Service, expectedURL string) {
				mcp := mcpFunc()
				service := serviceFunc()

				url := reconciler.buildServiceURL(mcp, service)
				Expect(url).To(Equal(expectedURL))
			},

			Entry("builds HTTP URL with default port",
				func() *v1alpha1.MCP {
					return &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-url",
							Namespace: "default",
						},
						Spec: v1alpha1.MCPSpec{
							Image: "test:image",
						},
					}
				},
				func() *corev1.Service {
					return &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-url-service",
							Namespace: "default",
						},
					}
				},
				"http://test-url-service.default.svc.cluster.local:3000"),

			Entry("builds HTTPS URL when TLS is enabled",
				func() *v1alpha1.MCP {
					return &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-url-tls",
							Namespace: "default",
						},
						Spec: v1alpha1.MCPSpec{
							Image: "test:image",
							Server: &v1alpha1.MCPServerSpec{
								Port: 8443,
								TLS: &v1alpha1.MCPTLSConfig{
									Enabled:    true,
									SecretName: "tls-secret",
								},
							},
						},
					}
				},
				func() *corev1.Service {
					return &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-url-tls-service",
							Namespace: "default",
						},
					}
				},
				"https://test-url-tls-service.default.svc.cluster.local:8443"),

			Entry("builds URL with custom port",
				func() *v1alpha1.MCP {
					return &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-url-custom",
							Namespace: "default",
						},
						Spec: v1alpha1.MCPSpec{
							Image: "test:image",
							Server: &v1alpha1.MCPServerSpec{
								Port: 9090,
							},
						},
					}
				},
				func() *corev1.Service {
					return &corev1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-url-custom-service",
							Namespace: "production",
						},
					}
				},
				"http://test-url-custom-service.production.svc.cluster.local:9090"),

			Entry("returns empty string for nil service",
				func() *v1alpha1.MCP {
					return &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-url-nil",
							Namespace: "default",
						},
					}
				},
				func() *corev1.Service {
					return nil
				},
				""),
		)

		DescribeTable("isDeploymentReady scenarios",
			func(deploymentFunc func() *appsv1.Deployment, expectedReady bool) {
				deployment := deploymentFunc()
				isReady := reconciler.isDeploymentReady(deployment)
				Expect(isReady).To(Equal(expectedReady))
			},

			Entry("returns false for nil deployment",
				func() *appsv1.Deployment {
					return nil
				}, false),

			Entry("returns false for deployment with no ready replicas",
				func() *appsv1.Deployment {
					replicas := int32(3)
					return &appsv1.Deployment{
						Spec: appsv1.DeploymentSpec{
							Replicas: &replicas,
						},
						Status: appsv1.DeploymentStatus{
							Replicas:      3,
							ReadyReplicas: 0,
						},
					}
				}, false),

			Entry("returns false when ready replicas < total replicas",
				func() *appsv1.Deployment {
					replicas := int32(3)
					return &appsv1.Deployment{
						Spec: appsv1.DeploymentSpec{
							Replicas: &replicas,
						},
						Status: appsv1.DeploymentStatus{
							Replicas:      3,
							ReadyReplicas: 2,
						},
					}
				}, false),

			Entry("returns false when status replicas != spec replicas",
				func() *appsv1.Deployment {
					replicas := int32(3)
					return &appsv1.Deployment{
						Spec: appsv1.DeploymentSpec{
							Replicas: &replicas,
						},
						Status: appsv1.DeploymentStatus{
							Replicas:      2,
							ReadyReplicas: 2,
						},
					}
				}, false),

			Entry("returns true when all replicas are ready",
				func() *appsv1.Deployment {
					replicas := int32(3)
					return &appsv1.Deployment{
						Spec: appsv1.DeploymentSpec{
							Replicas: &replicas,
						},
						Status: appsv1.DeploymentStatus{
							Replicas:      3,
							ReadyReplicas: 3,
						},
					}
				}, true),

			Entry("returns true for single replica deployment",
				func() *appsv1.Deployment {
					replicas := int32(1)
					return &appsv1.Deployment{
						Spec: appsv1.DeploymentSpec{
							Replicas: &replicas,
						},
						Status: appsv1.DeploymentStatus{
							Replicas:      1,
							ReadyReplicas: 1,
						},
					}
				}, true),
		)
	})

	Context("Status updates", func() {
		var (
			ctx        context.Context
			reconciler *MCPReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			reconciler = &MCPReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		It("should update status with URL and ready state", func() {
			mcp := &v1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-status-update",
					Namespace:  "default",
					Generation: 1,
				},
				Spec: v1alpha1.MCPSpec{
					Image: "test:image",
					Server: &v1alpha1.MCPServerSpec{
						Port: 8080,
					},
				},
			}
			Expect(k8sClient.Create(ctx, mcp)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, mcp) })

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-status-update-service",
					Namespace: "default",
				},
			}

			replicas := int32(1)
			deployment := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
				Status: appsv1.DeploymentStatus{
					Replicas:      1,
					ReadyReplicas: 1,
				},
			}

			err := reconciler.updateStatus(ctx, mcp, service, deployment)
			Expect(err).NotTo(HaveOccurred())

			updatedMCP := &v1alpha1.MCP{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-status-update",
				Namespace: "default",
			}, updatedMCP)).To(Succeed())

			Expect(updatedMCP.Status.URL).To(Equal("http://test-status-update-service.default.svc.cluster.local:8080"))
			Expect(updatedMCP.Status.Ready).To(BeTrue())
			Expect(updatedMCP.Status.ObservedGeneration).To(Equal(int64(1)))
		})

		It("should not update status if nothing changed", func() {
			mcp := &v1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "test-no-status-change",
					Namespace:  "default",
					Generation: 1,
				},
				Spec: v1alpha1.MCPSpec{
					Image: "test:image",
				},
				Status: v1alpha1.MCPStatus{
					URL:                "http://test-no-status-change-service.default.svc.cluster.local:3000",
					Ready:              false,
					ObservedGeneration: 1,
				},
			}
			Expect(k8sClient.Create(ctx, mcp)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, mcp) })

			service := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-no-status-change-service",
					Namespace: "default",
				},
			}

			replicas := int32(1)
			deployment := &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Replicas: &replicas,
				},
				Status: appsv1.DeploymentStatus{
					Replicas:      1,
					ReadyReplicas: 0, // Not ready
				},
			}

			err := reconciler.updateStatus(ctx, mcp, service, deployment)
			Expect(err).NotTo(HaveOccurred())

			updatedMCP := &v1alpha1.MCP{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-no-status-change",
				Namespace: "default",
			}, updatedMCP)).To(Succeed())

			Expect(updatedMCP.Status.URL).To(Equal("http://test-no-status-change-service.default.svc.cluster.local:3000"))
			Expect(updatedMCP.Status.Ready).To(BeFalse())
			Expect(updatedMCP.Status.ObservedGeneration).To(Equal(int64(1)))
		})
	})

	Context("buildService", func() {
		var reconciler *MCPReconciler

		BeforeEach(func() {
			reconciler = &MCPReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		DescribeTable("buildService scenarios",
			func(mcpFunc func() *v1alpha1.MCP, expectNil bool, validateService func(*corev1.Service)) {
				mcp := mcpFunc()
				service := reconciler.buildService(mcp)

				if expectNil {
					Expect(service).To(BeNil())
				} else {
					Expect(service).NotTo(BeNil())
					if validateService != nil {
						validateService(service)
					}
				}
			},

			Entry("returns nil for nil MCP",
				func() *v1alpha1.MCP {
					return nil
				}, true, nil),

			Entry("builds service with default configuration",
				func() *v1alpha1.MCP {
					return &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-build-default",
							Namespace: "default",
						},
						Spec: v1alpha1.MCPSpec{
							Image: "test:image",
						},
					}
				}, false, func(service *corev1.Service) {
					Expect(service.Name).To(Equal("test-build-default-service"))
					Expect(service.Namespace).To(Equal("default"))
					Expect(service.Labels).To(HaveKeyWithValue("app", "test-build-default"))
					Expect(service.Spec.Selector).To(HaveKeyWithValue("app", "test-build-default"))
					Expect(service.Spec.Type).To(Equal(corev1.ServiceTypeClusterIP))
					Expect(service.Spec.Ports).To(HaveLen(1))
					Expect(service.Spec.Ports[0].Name).To(Equal("http"))
					Expect(service.Spec.Ports[0].Port).To(Equal(int32(3000)))
					Expect(service.Spec.Ports[0].Protocol).To(Equal(corev1.ProtocolTCP))
				}),

			Entry("builds service with custom port",
				func() *v1alpha1.MCP {
					return &v1alpha1.MCP{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-build-custom",
							Namespace: "custom-ns",
						},
						Spec: v1alpha1.MCPSpec{
							Image: "test:image",
							Server: &v1alpha1.MCPServerSpec{
								Port: 9999,
							},
						},
					}
				}, false, func(service *corev1.Service) {
					Expect(service.Name).To(Equal("test-build-custom-service"))
					Expect(service.Namespace).To(Equal("custom-ns"))
					Expect(service.Spec.Ports[0].Port).To(Equal(int32(9999)))
				}),
		)
	})

	Context("Service integration", func() {
		var (
			ctx        context.Context
			reconciler *MCPReconciler
		)

		BeforeEach(func() {
			ctx = context.Background()
			reconciler = &MCPReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}
		})

		It("should create Service during full reconciliation", func() {
			mcp := &v1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-integration-service",
					Namespace: "default",
				},
				Spec: v1alpha1.MCPSpec{
					Image: "test:image",
					Server: &v1alpha1.MCPServerSpec{
						Port: 8080,
					},
				},
			}
			Expect(k8sClient.Create(ctx, mcp)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, mcp) })

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-integration-service",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-integration-service-service",
				Namespace: "default",
			}, service)).To(Succeed())

			Expect(service.Spec.Ports).To(HaveLen(1))
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(8080)))
			Expect(service.Spec.Selector).To(HaveKeyWithValue("app", "test-integration-service"))

			deployment := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-integration-service",
				Namespace: "default",
			}, deployment)).To(Succeed())

			updatedMCP := &v1alpha1.MCP{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-integration-service",
				Namespace: "default",
			}, updatedMCP)).To(Succeed())

			Expect(updatedMCP.Status.URL).To(Equal("http://test-integration-service-service.default.svc.cluster.local:8080"))
			Expect(updatedMCP.Status.ObservedGeneration).To(Equal(mcp.Generation))

			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, service)
				_ = k8sClient.Delete(ctx, deployment)
			})
		})

		It("should update existing Service when MCP spec changes", func() {
			mcp := &v1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-service-update",
					Namespace: "default",
				},
				Spec: v1alpha1.MCPSpec{
					Image: "test:image",
					Server: &v1alpha1.MCPServerSpec{
						Port: 8080,
					},
				},
			}
			Expect(k8sClient.Create(ctx, mcp)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, mcp) })

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-service-update",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			service := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-service-update-service",
				Namespace: "default",
			}, service)).To(Succeed())
			Expect(service.Spec.Ports[0].Port).To(Equal(int32(8080)))

			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-service-update",
				Namespace: "default",
			}, mcp)).To(Succeed())
			mcp.Spec.Server.Port = 9090
			Expect(k8sClient.Update(ctx, mcp)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-service-update",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			updatedService := &corev1.Service{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-service-update-service",
				Namespace: "default",
			}, updatedService)).To(Succeed())
			Expect(updatedService.Spec.Ports[0].Port).To(Equal(int32(9090)))

			updatedMCP := &v1alpha1.MCP{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-service-update",
				Namespace: "default",
			}, updatedMCP)).To(Succeed())
			Expect(updatedMCP.Status.URL).To(Equal("http://test-service-update-service.default.svc.cluster.local:9090"))

			DeferCleanup(func() {
				_ = k8sClient.Delete(ctx, updatedService)
				deployment := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-service-update", Namespace: "default"}, deployment); err == nil {
					_ = k8sClient.Delete(ctx, deployment)
				}
			})
		})

		It("should handle TLS configuration in URL generation", func() {
			mcp := &v1alpha1.MCP{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-tls-url",
					Namespace: "default",
				},
				Spec: v1alpha1.MCPSpec{
					Image: "test:image",
					Server: &v1alpha1.MCPServerSpec{
						Port: 8443,
						TLS: &v1alpha1.MCPTLSConfig{
							Enabled:    true,
							SecretName: "tls-secret",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, mcp)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, mcp) })

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-tls-url",
					Namespace: "default",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			updatedMCP := &v1alpha1.MCP{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      "test-tls-url",
				Namespace: "default",
			}, updatedMCP)).To(Succeed())

			Expect(updatedMCP.Status.URL).To(Equal("https://test-tls-url-service.default.svc.cluster.local:8443"))

			DeferCleanup(func() {
				service := &corev1.Service{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-tls-url-service", Namespace: "default"}, service); err == nil {
					_ = k8sClient.Delete(ctx, service)
				}
				deployment := &appsv1.Deployment{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: "test-tls-url", Namespace: "default"}, deployment); err == nil {
					_ = k8sClient.Delete(ctx, deployment)
				}
			})
		})
	})
})
