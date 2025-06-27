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
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	runtime "k8s.io/apimachinery/pkg/runtime"
	types "k8s.io/apimachinery/pkg/types"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "github.com/inference-gateway/operator/api/v1alpha1"
)

// A2AReconciler reconciles a A2A object
type A2AReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=a2as,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=a2as/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.inference-gateway.com,resources=a2as/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *A2AReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	if !r.shouldWatchNamespace(ctx, req.Namespace) {
		logger.V(1).Info("Skipping Gateway in namespace not matching watch criteria", "namespace", req.Namespace)
		return ctrl.Result{}, nil
	}

	var a2a v1alpha1.A2A
	if err := r.Get(ctx, req.NamespacedName, &a2a); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !a2a.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	deploy := &appsv1.Deployment{}
	deployName := a2a.Name
	err := r.Get(ctx, client.ObjectKey{Namespace: a2a.Namespace, Name: deployName}, deploy)
	if err != nil {
		deploy = buildA2ADeployment(&a2a)
		if err := r.Create(ctx, deploy); err != nil {
			logger.Error(err, "failed to create deployment")
			return ctrl.Result{}, err
		}
		logger.Info("created deployment", "name", deployName)
	} else {
		wantImage := a2a.Spec.Image
		gotImage := deploy.Spec.Template.Spec.Containers[0].Image
		if gotImage != wantImage {
			deploy.Spec.Template.Spec.Containers[0].Image = wantImage
			if err := r.Update(ctx, deploy); err != nil {
				logger.Error(err, "failed to update deployment image")
				return ctrl.Result{}, err
			}
			logger.Info("updated deployment image", "name", deployName, "image", wantImage)
		}
	}

	svc := &corev1.Service{}
	svcName := a2a.Name
	err = r.Get(ctx, client.ObjectKey{Namespace: a2a.Namespace, Name: svcName}, svc)
	if err != nil {
		svc = buildA2AService(&a2a)
		if err := r.Create(ctx, svc); err != nil {
			logger.Error(err, "failed to create service")
			return ctrl.Result{}, err
		}
		logger.Info("created service", "name", svcName)
	}

	card, err := fetchAgentCard(svc)
	if err != nil {
		logger.Info("failed to fetch agent card", "error", err.Error())
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	card.SkillsNames = card.Skills.SkillsNames()

	patch := client.MergeFrom(a2a.DeepCopy())
	a2a.Status.Card = *card
	if err := r.Status().Patch(ctx, &a2a, patch); err != nil {
		logger.Error(err, "unable to update a2a status.card")
		return ctrl.Result{}, err
	}
	logger.Info("updated a2a status.card", "version", card.Version)
	return ctrl.Result{}, nil
}

// buildA2AService returns a Service for the given A2A resource.
func buildA2AService(a2a *v1alpha1.A2A) *corev1.Service {
	labels := map[string]string{
		"app": a2a.Name,
	}
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a2a.Name,
			Namespace: a2a.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       8080,
				TargetPort: intstrFromInt(8080),
			}},
		},
	}
}

// fetchAgentCard retrieves the agent card from the given base URL and unmarshals it into an A2ACard.
func fetchAgentCard(svc *corev1.Service) (*v1alpha1.Card, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://" + svc.Name + "." + svc.Namespace + ".svc.cluster.local:8080" + "/.well-known/agent.json")
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("unexpected status: " + resp.Status)
	}
	var card v1alpha1.Card
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return nil, err
	}
	return &card, nil
}

// buildA2ADeployment returns a Deployment for the given A2A resource.
func buildA2ADeployment(a2a *v1alpha1.A2A) *appsv1.Deployment {
	labels := map[string]string{
		"app": a2a.Name,
	}

	var env []corev1.EnvVar
	if a2a.Spec.Env != nil {
		env = *a2a.Spec.Env
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      a2a.Name,
			Namespace: a2a.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Env:   env,
						Name:  "agent",
						Image: a2a.Spec.Image,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
						}},
					}},
				},
			},
		},
	}
}

func int32Ptr(i int32) *int32 { return &i }

// intstrFromInt returns an IntOrString for a port.
func intstrFromInt(i int) intstr.IntOrString {
	return intstr.IntOrString{Type: intstr.Int, IntVal: int32(i)}
}

// shouldWatchNamespace checks if the operator should watch resources in the given namespace
// based on WATCH_NAMESPACE_SELECTOR environment variable
func (r *A2AReconciler) shouldWatchNamespace(ctx context.Context, namespace string) bool {
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
func (r *A2AReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.A2A{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Named("a2a").
		Complete(r)
}
