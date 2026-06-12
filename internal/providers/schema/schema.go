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

// Package schema reads the provider identifiers from the canonical Inference
// Gateway OpenAPI schema (components.schemas.Provider.enum). It is the single
// reader shared by the code generator (../gen) and the drift test in the parent
// package, so both interpret the schema identically and cannot disagree about
// what "the providers" are.
package schema

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

// DefaultSource is the canonical schema location: the same file the gateway
// downloads to generate its provider constants
// (see the gateway's `task openapi:download`).
const DefaultSource = "https://raw.githubusercontent.com/inference-gateway/schemas/refs/heads/main/openapi.yaml"

// SourceEnv is the environment variable that overrides the schema source with a
// URL or a local file path. It exists so generation and the drift test can run
// hermetically against a vendored copy when needed.
const SourceEnv = "OPENAPI_SCHEMA_SOURCE"

// Source returns the configured schema source, honoring SourceEnv and falling
// back to DefaultSource.
func Source() string {
	if s := strings.TrimSpace(os.Getenv(SourceEnv)); s != "" {
		return s
	}
	return DefaultSource
}

// openAPI is the minimal projection of the schema this package cares about.
type openAPI struct {
	Components struct {
		Schemas struct {
			Provider struct {
				Enum []string `json:"enum"`
			} `json:"Provider"`
		} `json:"schemas"`
	} `json:"components"`
}

// Providers parses raw OpenAPI YAML (or JSON) and returns the sorted provider
// identifiers from components.schemas.Provider.enum.
func Providers(raw []byte) ([]string, error) {
	var doc openAPI
	// sigs.k8s.io/yaml converts YAML to JSON, so the JSON struct tags above apply.
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse schema: %w", err)
	}
	enum := doc.Components.Schemas.Provider.Enum
	if len(enum) == 0 {
		return nil, fmt.Errorf("no providers found at components.schemas.Provider.enum")
	}
	out := append([]string(nil), enum...)
	sort.Strings(out)
	return out, nil
}

// Load reads raw schema bytes from source, which may be an http(s) URL or a
// local file path. HTTP fetches are retried a few times to tolerate transient
// network blips in CI.
func Load(source string) ([]byte, error) {
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return loadHTTP(source)
	}
	return os.ReadFile(source)
}

func loadHTTP(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status %s fetching %s", resp.Status, url)
			continue
		}
		return body, nil
	}
	return nil, lastErr
}

// FetchProviders loads and parses the provider identifiers from source.
func FetchProviders(source string) ([]string, error) {
	raw, err := Load(source)
	if err != nil {
		return nil, err
	}
	return Providers(raw)
}
