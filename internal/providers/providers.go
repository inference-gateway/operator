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

// Package providers exposes the set of AI/ML provider identifiers that the
// Inference Gateway understands.
//
// The list lives in exactly one place: the canonical schema at
// github.com/inference-gateway/schemas (openapi.yaml,
// components.schemas.Provider.enum). It is projected into SupportedProviders
// (see zz_generated_providers.go) by the generator under ./gen. The gateway
// generates its own provider constants from the same schema, so the two cannot
// silently drift: TestSupportedProvidersMatchSchema in providers_test.go fails
// when this package and the live schema disagree.
//
// Regenerate the list with `task verify-shared-types`.
package providers

//go:generate go run ./gen

import "strings"

// CustomProvider is an operator-specific passthrough provider. It is NOT part of
// the canonical schema enum: it lets users point the gateway at an arbitrary
// OpenAI-compatible endpoint via CUSTOM_API_URL / CUSTOM_API_KEY. It is accepted
// in addition to SupportedProviders for backward compatibility.
const CustomProvider = "custom"

// supportedSet is the lower-cased lookup built from SupportedProviders plus the
// operator-specific CustomProvider. Go initializes SupportedProviders (declared
// in zz_generated_providers.go) before this var, regardless of file ordering.
var supportedSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(SupportedProviders)+1)
	for _, p := range SupportedProviders {
		m[strings.ToLower(p)] = struct{}{}
	}
	m[CustomProvider] = struct{}{}
	return m
}()

// IsSupported reports whether name is a provider the operator recognizes. The
// comparison is case-insensitive so spec values such as "OpenAI" or "Custom"
// match. It returns true for every entry in SupportedProviders and for
// CustomProvider.
func IsSupported(name string) bool {
	_, ok := supportedSet[strings.ToLower(name)]
	return ok
}
