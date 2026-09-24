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

	corev1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/api/meta"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	handler "sigs.k8s.io/controller-runtime/pkg/handler"
)

// watchSelectorMatches reports whether the given namespace labels satisfy
// WATCH_NAMESPACE_SELECTOR. An unset or unparsable selector matches everything.
func watchSelectorMatches(nsLabels map[string]string) bool {
	selector := os.Getenv("WATCH_NAMESPACE_SELECTOR")
	if selector == "" {
		return true
	}

	parsed, err := labels.Parse(selector)
	if err != nil {
		return true
	}

	return parsed.Matches(labels.Set(nsLabels))
}

// namespaceWatched reports whether the operator should reconcile resources in the
// given namespace, based on WATCH_NAMESPACE_SELECTOR.
func namespaceWatched(ctx context.Context, c client.Client, namespace string) bool {
	if os.Getenv("WATCH_NAMESPACE_SELECTOR") == "" {
		return true
	}

	ns := &corev1.Namespace{}
	if err := c.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
		return false
	}

	return watchSelectorMatches(ns.Labels)
}

// namespaceHandler enqueues every object of the given list type living in a
// Namespace once that Namespace matches WATCH_NAMESPACE_SELECTOR, so resources
// created before the namespace was labelled are reconciled instead of staying
// stuck at the skipped-namespace check.
func namespaceHandler(c client.Client, newList func() client.ObjectList) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(namespaceMapper(c, newList))
}

// namespaceMapper maps a Namespace event to reconcile requests for every object
// of the given list type in it, once the Namespace matches the watch selector.
func namespaceMapper(c client.Client, newList func() client.ObjectList) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []ctrl.Request {
		ns, ok := obj.(*corev1.Namespace)
		if !ok || !watchSelectorMatches(ns.Labels) {
			return nil
		}

		list := newList()
		if err := c.List(ctx, list, client.InNamespace(ns.Name)); err != nil {
			return nil
		}

		items, err := meta.ExtractList(list)
		if err != nil {
			return nil
		}

		requests := make([]ctrl.Request, 0, len(items))
		for _, item := range items {
			o, ok := item.(client.Object)
			if !ok {
				continue
			}
			requests = append(requests, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(o)})
		}
		return requests
	}
}
