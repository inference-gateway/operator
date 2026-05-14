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
