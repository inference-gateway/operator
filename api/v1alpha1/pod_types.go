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
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// PodSpec shapes the pods a workload controller creates, for sidecar injection
// (pod labels/annotations, extra containers and volumes) and scheduling
// (node selector, tolerations, affinity).
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

	// Containers are sidecars appended after the operator's own container, for
	// VPNs or proxies that ship no injector. A name that collides with an
	// operator-owned container is rejected.
	// +optional
	Containers []corev1.Container `json:"containers,omitempty"`

	// InitContainers run before the operator's container, or alongside it when
	// they set restartPolicy: Always (native sidecars).
	// +optional
	InitContainers []corev1.Container `json:"initContainers,omitempty"`

	// Volumes are added to the pod so the extra containers can mount them. A
	// name that collides with an operator-owned volume is rejected.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`
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
	template.Spec.Containers = append(template.Spec.Containers, p.Containers...)
	template.Spec.InitContainers = append(template.Spec.InitContainers, p.InitContainers...)
	template.Spec.Volumes = append(template.Spec.Volumes, p.Volumes...)
}

// ValidatePodTemplateNames rejects duplicate container, init container and
// volume names in a pod template built by a controller. Only spec.pod can
// introduce one, by reusing a name the operator already owns.
func ValidatePodTemplateNames(template *corev1.PodTemplateSpec) error {
	return errors.Join(
		firstDuplicate("spec.pod.containers", containerNames(template.Spec.Containers)),
		firstDuplicate("spec.pod.initContainers", containerNames(template.Spec.InitContainers)),
		firstDuplicate("spec.pod.volumes", volumeNames(template.Spec.Volumes)),
	)
}

func containerNames(containers []corev1.Container) []string {
	names := make([]string, 0, len(containers))
	for _, c := range containers {
		names = append(names, c.Name)
	}
	return names
}

func volumeNames(volumes []corev1.Volume) []string {
	names := make([]string, 0, len(volumes))
	for _, v := range volumes {
		names = append(names, v.Name)
	}
	return names
}

func firstDuplicate(field string, names []string) error {
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		if _, duplicate := seen[name]; duplicate {
			return fmt.Errorf("%s: %q is already used by the operator", field, name)
		}
		seen[name] = struct{}{}
	}
	return nil
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
