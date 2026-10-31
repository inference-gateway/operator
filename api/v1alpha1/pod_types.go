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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
)

// PodSpec shapes the pods a workload controller creates, for sidecar injection
// (pod labels/annotations) and scheduling (node selector, tolerations, affinity).
type PodSpec struct {
	// Labels are added to the pod template, for example to opt into a sidecar
	// injector. Operator-owned labels such as app take precedence.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are added to the pod template, for example
	// sidecar.istio.io/inject or linkerd.io/inject.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// NodeSelector pins the pods to nodes with these labels.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Tolerations allow the pods onto tainted nodes, for example a GPU pool.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// Affinity constrains or spreads the pods across nodes.
	// +optional
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
}

// ApplyTo merges the user-supplied pod customization into a pod template built
// by a controller. Labels and annotations already on the template are
// operator-owned and win, so Deployment selectors keep matching.
func (p *PodSpec) ApplyTo(template *corev1.PodTemplateSpec) {
	if p == nil {
		return
	}

	template.Labels = mergeWithoutOverriding(p.Labels, template.Labels)
	template.Annotations = mergeWithoutOverriding(p.Annotations, template.Annotations)
	template.Spec.NodeSelector = p.NodeSelector
	template.Spec.Tolerations = p.Tolerations
	template.Spec.Affinity = p.Affinity
}

func mergeWithoutOverriding(base, authoritative map[string]string) map[string]string {
	if len(base) == 0 {
		return authoritative
	}

	merged := make(map[string]string, len(base)+len(authoritative))
	for k, v := range base {
		merged[k] = v
	}
	for k, v := range authoritative {
		merged[k] = v
	}
	return merged
}
