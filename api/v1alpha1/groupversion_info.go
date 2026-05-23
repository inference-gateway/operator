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

// Package v1alpha1 contains API Schema definitions for the core v1alpha1 API group.
// +kubebuilder:object:generate=true
// +groupName=core.inference-gateway.com
package v1alpha1

import (
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "core.inference-gateway.com", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &schemeBuilder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// schemeBuilder collects functions that register types with a runtime.Scheme.
// It is a minimal, local replacement for sigs.k8s.io/controller-runtime/pkg/scheme.Builder,
// which was deprecated in controller-runtime v0.24 to keep api packages free of
// controller-runtime dependencies.
type schemeBuilder struct {
	GroupVersion schema.GroupVersion
	runtime.SchemeBuilder
}

// Register registers the given objects with the SchemeBuilder under the
// configured GroupVersion. It mirrors the behavior of the deprecated
// controller-runtime scheme.Builder.Register.
func (b *schemeBuilder) Register(objects ...runtime.Object) *schemeBuilder {
	b.SchemeBuilder.Register(func(scheme *runtime.Scheme) error {
		for _, obj := range objects {
			gvk := b.GroupVersion.WithKind(reflect.TypeOf(obj).Elem().Name())
			scheme.AddKnownTypeWithName(gvk, obj)
		}
		metav1.AddToGroupVersion(scheme, b.GroupVersion)
		return nil
	})
	return b
}

// AddToScheme adds all registered types to the given scheme.
func (b *schemeBuilder) AddToScheme(s *runtime.Scheme) error {
	return b.SchemeBuilder.AddToScheme(s)
}
