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
	"fmt"
	"os"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	controllerutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

// MCPReconciler reconciles a MCP object
type MCPReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=mcps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=mcps/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=mcps/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *MCPReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	if !r.shouldWatchNamespace(ctx, req.Namespace) {
		logger.V(1).Info("Skipping Gateway in namespace not matching watch criteria", "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	var mcp v1alpha1.MCP
	if err := r.Get(ctx, req.NamespacedName, &mcp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !mcp.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	_, err := r.reconcileDeployment(ctx, &mcp)
	if err != nil {
		logger.Error(err, "Failed to reconcile MCP deployment", "name", mcp.Name, "namespace", mcp.Namespace)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MCPReconciler) reconcileDeployment(ctx context.Context, mcp *v1alpha1.MCP) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)
	if mcp == nil {
		logger.Error(nil, "MCP is nil, cannot reconcile deployment")
		return nil, fmt.Errorf("MCP is nil")
	}

	deployment := r.buildDeployment(mcp)
	if deployment == nil {
		return nil, fmt.Errorf("failed to build deployment for MCP %s/%s", mcp.Namespace, mcp.Name)
	}

	if err := controllerutil.SetControllerReference(mcp, deployment, r.Scheme); err != nil {
		return nil, err
	}

	return r.createOrUpdateDeployment(ctx, mcp, deployment)
}

func (r *MCPReconciler) buildDeployment(mcp *v1alpha1.MCP) *appsv1.Deployment {
	if mcp == nil {
		return nil
	}

	const defaultImage = "node:lts"
	const defaultPort int32 = 3000

	image := mcp.Spec.Image
	if image == "" {
		image = defaultImage
	}

	port := defaultPort
	if mcp.Spec.Server != nil {
		if mcp.Spec.Server.Port != 0 {
			port = mcp.Spec.Server.Port
		}
	}

	pkg := mcp.Spec.Package
	if pkg == "" {
		pkg = "mcp-default-package"
	}

	deployment := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mcp.Name,
			Namespace: mcp.Namespace,
			Labels: map[string]string{
				"app": mcp.Name,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": mcp.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": mcp.Name,
					},
				},
				Spec: corev1.PodSpec{},
			},
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
			},
		},
	}

	if mcp.Spec.Replicas != nil && mcp.Spec.HPA == nil {
		deployment.Spec.Replicas = mcp.Spec.Replicas
	}

	container := corev1.Container{
		Name:  "mcp",
		Image: image,
		Ports: []corev1.ContainerPort{
			{
				Name:          "http",
				ContainerPort: port,
			},
		},
		Env: []corev1.EnvVar{
			{
				Name:  "MCP_NAME",
				Value: mcp.Name,
			},
			{
				Name:  "MCP_NAMESPACE",
				Value: mcp.Namespace,
			},
		},
		Command: []string{"npx"},
		Args: []string{
			"-y",
			pkg,
			"--port",
			fmt.Sprintf("%d", port),
		},
	}

	containers := []corev1.Container{container}

	if mcp.Spec.Bridge != nil {
		containers = append(containers, corev1.Container{
			Name:  "bridge",
			Image: "ghcr.io/inference-gateway/bridge:latest",
			Ports: []corev1.ContainerPort{
				{
					Name:          "http",
					ContainerPort: mcp.Spec.Bridge.Port,
				},
			},
			Env: []corev1.EnvVar{},
		})
	}

	deployment.Spec.Template.Spec.Containers = containers
	return &deployment
}

func (r *MCPReconciler) createOrUpdateDeployment(ctx context.Context, mcp *v1alpha1.MCP, deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)

	found := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating deployment", "Deployment.Name", deployment.Name)
		if err = r.Create(ctx, deployment); err != nil {
			return nil, err
		}
		return deployment, nil
	} else if err != nil {
		return nil, err
	}

	return r.updateDeploymentIfNeeded(ctx, mcp, deployment, found)
}

func (r *MCPReconciler) updateDeploymentIfNeeded(ctx context.Context, _ *v1alpha1.MCP, deployment *appsv1.Deployment, found *appsv1.Deployment) (*appsv1.Deployment, error) {
	logger := logf.FromContext(ctx)

	if reflect.DeepEqual(&deployment.Spec, &found.Spec) {
		logger.V(1).Info("deployment up-to-date, no update needed", "Deployment.Name", found.Name)
		return found, nil
	}

	found.Spec = deployment.Spec

	if err := r.Update(ctx, found); err != nil {
		logger.Error(err, "failed to update deployment", "Deployment.Name", found.Name)
		return nil, err
	}

	logger.Info("updated deployment", "Deployment.Name", found.Name)
	return found, nil

}

// shouldWatchNamespace checks if the operator should watch resources in the given namespace
// based on WATCH_NAMESPACE_SELECTOR environment variable
func (r *MCPReconciler) shouldWatchNamespace(ctx context.Context, namespace string) bool {
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
func (r *MCPReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MCP{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("mcp").
		Complete(r)
}
