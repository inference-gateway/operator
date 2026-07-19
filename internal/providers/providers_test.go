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

package providers

import (
	"os"
	"reflect"
	"sort"
	"testing"

	"github.com/inference-gateway/operator/internal/providers/schema"
)

func TestIsSupported(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "known provider", in: "openai", want: true},
		{name: "case-insensitive", in: "OpenAI", want: true},
		{name: "google is not dropped", in: "google", want: true},
		{name: "mistral", in: "mistral", want: true},
		{name: "ollama_cloud", in: "ollama_cloud", want: true},
		{name: "llamacpp", in: "llamacpp", want: true},
		{name: "llamacpp is case-insensitive", in: "Llamacpp", want: true},
		{name: "custom passthrough", in: "custom", want: true},
		{name: "custom is case-insensitive", in: "Custom", want: true},
		{name: "unknown provider", in: "totally-made-up", want: false},
		{name: "empty", in: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupported(tt.in); got != tt.want {
				t.Errorf("IsSupported(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestSupportedProvidersHasNoDuplicates(t *testing.T) {
	seen := make(map[string]struct{}, len(SupportedProviders))
	for _, p := range SupportedProviders {
		if _, dup := seen[p]; dup {
			t.Errorf("SupportedProviders contains duplicate %q", p)
		}
		seen[p] = struct{}{}
	}
}

// TestSupportedProvidersMatchSchema is the shared-types divergence guard. It
// fails when this package's SupportedProviders disagrees with the canonical
// schema's components.schemas.Provider.enum, i.e. when the operator and gateway
// have drifted apart. In CI the schema must be reachable and any mismatch is a
// hard failure; locally the test skips when the schema cannot be fetched
// (offline) rather than producing a false failure.
func TestSupportedProvidersMatchSchema(t *testing.T) {
	source := schema.Source()

	want, err := schema.FetchProviders(source)
	if err != nil {
		if isCI() {
			t.Fatalf("fetch canonical schema (%s): %v", source, err)
		}
		t.Skipf("skipping schema drift check (offline?): %v", err)
	}

	got := append([]string(nil), SupportedProviders...)
	sort.Strings(got)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("operator SupportedProviders has drifted from the canonical schema.\n"+
			"  operator: %v\n"+
			"  schema:   %v\n"+
			"run `task verify-shared-types` and commit internal/providers/zz_generated_providers.go",
			got, want)
	}
}

func isCI() bool {
	return os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != ""
}
