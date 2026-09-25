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
	builder "sigs.k8s.io/controller-runtime/pkg/builder"
	client "sigs.k8s.io/controller-runtime/pkg/client"
	event "sigs.k8s.io/controller-runtime/pkg/event"
	handler "sigs.k8s.io/controller-runtime/pkg/handler"
	log "sigs.k8s.io/controller-runtime/pkg/log"
	predicate "sigs.k8s.io/controller-runtime/pkg/predicate"
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

// namespaceStartedMatching reports whether a Namespace update flipped it from
// unwatched to watched - the only transition that can unstick resources skipped
// by namespaceWatched. With no selector set every namespace always matches, so
// this is false and the watch costs nothing.
func namespaceStartedMatching(e event.UpdateEvent) bool {
	return !watchSelectorMatches(e.ObjectOld.GetLabels()) && watchSelectorMatches(e.ObjectNew.GetLabels())
}

// namespaceBecameWatched filters the Namespace watch down to that transition.
// Creates are dropped: resources cannot predate their namespace, and resources
// predating an operator restart arrive as their own Create events.
var namespaceBecameWatched = builder.WithPredicates(predicate.Funcs{
	CreateFunc:  func(event.CreateEvent) bool { return false },
	DeleteFunc:  func(event.DeleteEvent) bool { return false },
	GenericFunc: func(event.GenericEvent) bool { return false },
	UpdateFunc:  namespaceStartedMatching,
})

// namespaceMapper maps a Namespace event to reconcile requests for every object
// of the given list type in it, so resources created before the namespace was
// labelled are reconciled instead of staying stuck at the skipped-namespace check.
func namespaceMapper(c client.Client, newList func() client.ObjectList) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []ctrl.Request {
		ns, ok := obj.(*corev1.Namespace)
		if !ok || !watchSelectorMatches(ns.Labels) {
			return nil
		}

		list := newList()
		if err := c.List(ctx, list, client.InNamespace(ns.Name)); err != nil {
			log.FromContext(ctx).Error(err, "listing resources of a newly watched namespace", "namespace", ns.Name)
			return nil
		}

		items, err := meta.ExtractList(list)
		if err != nil {
			log.FromContext(ctx).Error(err, "extracting resources of a newly watched namespace", "namespace", ns.Name)
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
