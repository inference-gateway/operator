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

// Command gen regenerates SupportedProviders in the parent providers package
// from the canonical Inference Gateway schema (components.schemas.Provider.enum).
//
// It is invoked via `go generate ./internal/providers/...` (see the //go:generate
// directive in providers.go) or `task verify-shared-types`. The schema source
// defaults to the canonical URL and can be overridden with OPENAPI_SCHEMA_SOURCE.
package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"strconv"

	"github.com/inference-gateway/operator/internal/providers/schema"
)

// outputFile is written relative to the generator's working directory. Under
// `go generate`, that is the directory of the file holding the //go:generate
// directive (internal/providers), so the file lands next to providers.go.
const outputFile = "zz_generated_providers.go"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "providers generator:", err)
		os.Exit(1)
	}
}

func run() error {
	source := schema.Source()

	enum, err := schema.FetchProviders(source)
	if err != nil {
		return fmt.Errorf("read schema from %s: %w", source, err)
	}

	out, err := render(enum)
	if err != nil {
		return err
	}

	if err := os.WriteFile(outputFile, out, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", outputFile, err)
	}

	fmt.Printf("wrote %s (%d providers) from %s\n", outputFile, len(enum), source)
	return nil
}

func render(providers []string) ([]byte, error) {
	var b bytes.Buffer

	b.WriteString(header)
	b.WriteString("var SupportedProviders = []string{\n")
	for _, p := range providers {
		fmt.Fprintf(&b, "\t%s,\n", strconv.Quote(p))
	}
	b.WriteString("}\n")

	formatted, err := format.Source(b.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format generated source: %w", err)
	}
	return formatted, nil
}

const header = `/*
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

// Code generated from inference-gateway/schemas openapi.yaml; DO NOT EDIT.
// Source: components.schemas.Provider.enum
// Regenerate with: task verify-shared-types

package providers

// SupportedProviders is the set of provider identifiers defined by the canonical
// Inference Gateway schema. It is the single source of truth shared with the
// gateway, which generates the same list from the same schema. Do not edit by
// hand; see the package documentation in providers.go.
`
