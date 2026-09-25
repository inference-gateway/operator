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
	"os"
	"path/filepath"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	yaml "sigs.k8s.io/yaml"
)

// TestExamplesUseKnownFields strict-decodes every CR under examples/ into its
// API type. The CRD schemas do not preserve unknown fields, so a manifest that
// references a removed or renamed field (e.g. spec.ingress, or spec.routing
// used for Gateway API exposure) is rejected by kubectl's field validation.
// This catches that drift without a cluster.
func TestExamplesUseKnownFields(t *testing.T) {
	byKind := map[string]func() runtime.Object{
		"Gateway":      func() runtime.Object { return &Gateway{} },
		"Agent":        func() runtime.Object { return &Agent{} },
		"MCP":          func() runtime.Object { return &MCP{} },
		"Orchestrator": func() runtime.Object { return &Orchestrator{} },
	}

	manifests, err := filepath.Glob("../../examples/*/*.yaml")
	if err != nil {
		t.Fatalf("glob examples: %v", err)
	}
	if len(manifests) == 0 {
		t.Fatal("no example manifests found")
	}

	for _, path := range manifests {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read %s: %v", path, err)
			}
			for i, doc := range strings.Split(string(data), "\n---") {
				if strings.TrimSpace(doc) == "" {
					continue
				}
				var meta metav1.TypeMeta
				if err := yaml.Unmarshal([]byte(doc), &meta); err != nil {
					t.Fatalf("doc %d: %v", i, err)
				}
				newObj, ok := byKind[meta.Kind]
				if !ok || !strings.HasPrefix(meta.APIVersion, GroupVersion.Group+"/") {
					continue
				}
				if err := yaml.UnmarshalStrict([]byte(doc), newObj()); err != nil {
					t.Errorf("doc %d (%s): %v", i, meta.Kind, err)
				}
			}
		})
	}
}
