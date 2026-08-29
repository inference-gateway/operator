# AGENTS.md

Kubernetes operator for Inference Gateway, built with controller-runtime and kubebuilder conventions. Go 1.26+.

## Commands

All workflows run through [Task](https://taskfile.dev) (`task --list` for the full set):

- `task build` — regenerate manifests/code, format, vet, build `bin/manager`.
- `task test` — unit/integration tests via envtest (excludes e2e), writes `cover.out`.
- `task test:e2e` — Ginkgo e2e against a local k3d cluster (requires `ctlptl` + `k3d`).
- `task lint` — `golangci-lint`.
- `task generate` — regenerate deepcopy code after API changes.
- `task manifests` — regenerate CRDs, RBAC, and `manifests/install.yaml` + `crds.yaml`.
- `task run` — run the controller locally from `cmd/main.go`.

`task test` fetches envtest binaries and Gateway API CRDs, so it needs network on first run. `go build`/`go test` run offline.

## Layout

- `api/v1alpha1/` — CRD types (`gateway_types.go`, `agent_types.go`, `mcp_types.go`, `orchestrator_types.go`, `gpu_types.go`) plus generated deepcopy.
- `internal/controller/` — reconcilers, one per CRD, with matching `*_test.go`.
- `internal/providers/` — shared provider list (see below).
- `config/` — kustomize bases; `manifests/` — generated release artifacts (committed).
- `test/e2e/` — Ginkgo suites; `examples/` — runnable samples.

## Conventions

- Go: tabs, `gofmt`/`goimports`. YAML/Markdown: two-space indent, LF, trailing newline (`.editorconfig`).
- Keep API types in `api/v1alpha1/*_types.go`, reconcilers in `internal/controller/*_controller.go`, tests alongside.
- Run `task fmt`, `task vet`, and `task lint` before submitting.
- Add/update tests when changing reconciliation behavior, CRD schemas, defaults, or validation.

## Shared provider list

`internal/providers/zz_generated_providers.go` is generated from the canonical `inference-gateway/schemas` OpenAPI enum. To add/remove a provider, run `task generate:providers` and commit the result; `task verify-shared-types` fails on drift. `ProviderSpec.Name` has no CRD enum — validation is runtime via `providers.IsSupported` (case-insensitive).

When offline, the drift test skips locally but hard-fails in CI. Hand-edit the file (alphabetical, gofmt-clean) and let CI run the real check. Quick offline check: `go test ./internal/providers/ -run 'TestIsSupported|TestSupportedProvidersHasNoDuplicates' -v`.

## Commits & releases

semantic-release + conventional commits. Subjects like `feat(gateway): Add route weighting`, `fix(agent): Resolve status update error`, `docs: Update install example`. For API changes, commit regenerated output from `task generate` and `task manifests`.

## Security

Never commit secrets in samples or manifests — use Kubernetes Secrets. Keep local cluster config out of the repo. Verify generated CRDs/install manifests before release-facing changes.
