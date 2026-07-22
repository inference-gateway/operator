# Repository Guidelines

## Project Structure & Module Organization

This repository contains the Kubernetes operator for Inference Gateway. API types live in `api/v1alpha1/`; generated deepcopy code is kept alongside those types. The operator entry point is `cmd/main.go`, and reconcilers plus unit/integration-style controller tests live in `internal/controller/`. Kubernetes manifests are organized under `config/`, with generated release artifacts in `manifests/`. End-to-end tests and helpers are in `test/e2e/` and `test/utils/`. Runnable examples are under `examples/`, including Gateway, Agent, MCP, and Orchestrator samples.

## Build, Test, and Development Commands

Use `task --list` to see all available workflows.

- `task build`: regenerates manifests/code, formats, vets, and builds `bin/manager`.
- `task run`: runs the controller locally from `cmd/main.go`.
- `task test`: runs Go tests excluding e2e tests, using envtest and writing `cover.out`.
- `task test:e2e`: runs Ginkgo e2e tests against a local k3d cluster managed through `ctlptl`.
- `task lint`: runs `golangci-lint`.
- `task generate`: refreshes generated Go code after API changes.
- `task manifests`: regenerates CRDs, RBAC, and install manifests.
- `task docker-build IMG=ghcr.io/inference-gateway/operator:dev`: builds the manager image.

## Coding Style & Naming Conventions

Go code uses tabs and standard `gofmt`/`goimports` formatting. YAML and Markdown use two-space indentation, LF endings, final newlines, and trimmed trailing whitespace per `.editorconfig`. Keep API types in `api/v1alpha1/*_types.go`, reconcilers in `internal/controller/*_controller.go`, and tests in matching `*_test.go` files. Run `task fmt`, `task vet`, and `task lint` before submitting changes.

## Testing Guidelines

Controller tests use Go test with controller-runtime envtest; e2e tests use Ginkgo. Add or update tests when changing reconciliation behavior, CRD schemas, defaults, validation, or generated manifests. For focused e2e runs, use `task test:e2e:focus FOCUS="test name"`. Maintain or improve coverage where practical; `task test` updates `cover.out`.

## Shared provider list

The set of supported provider identifiers is shared with the gateway and generated from the canonical `inference-gateway/schemas` OpenAPI enum into `internal/providers/zz_generated_providers.go` (a sorted `SupportedProviders` list). To add or remove a provider, run `task generate:providers` (which runs `go generate ./internal/providers/...`) and commit the regenerated file; `task verify-shared-types` fails if it has drifted from the schema. No CRD change is needed - `ProviderSpec.Name` has no `+kubebuilder:validation:Enum`, so provider validation happens at runtime through `providers.IsSupported` (case-insensitive).

When outbound network is blocked, `task generate:providers` and the drift test `TestSupportedProvidersMatchSchema` cannot fetch the schema: the test skips locally but hard-fails when `CI` or `GITHUB_ACTIONS` is set. If you cannot regenerate, hand-edit `zz_generated_providers.go` by inserting the id in alphabetical order (keep it gofmt-clean) and let PR CI run the real drift check. `go test` and `go build` run offline, but `task test` fetches envtest binaries; for a quick offline check of the provider list run only the network-free tests: `go test ./internal/providers/ -run 'TestIsSupported|TestSupportedProvidersHasNoDuplicates' -v`.

## Commit & Pull Request Guidelines

This repo releases with semantic-release and conventional commits. Prefer messages such as `feat(gateway): Add route weighting`, `fix(agent): Resolve status update error`, or `docs: Update install example`; keep the subject concise and capitalized. PRs should describe the change, link relevant issues, mention API or manifest impacts, and include test results such as `task test` and `task lint`. For API changes, commit regenerated files from `task generate` and `task manifests`.

## Security & Configuration Tips

Do not commit secrets in samples or manifests. Use Kubernetes Secrets for tokens and credentials, and keep local cluster configuration outside the repository. Verify generated CRDs and install manifests before release-facing changes.
