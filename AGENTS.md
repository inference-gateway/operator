# AGENTS.md

Kubernetes operator for Inference Gateway, built with controller-runtime and kubebuilder conventions. Go 1.26+.

## Commands

All workflows run through [Task](https://taskfile.dev) (`task --list` for the full set):

- `task build` — regenerate manifests/code, format, vet, build `bin/manager`.
- `task test` — unit/integration tests via envtest (excludes e2e), writes `cover.out`.
- `task test:coverage` — `internal/controller` coverage; prints total, writes `cover.html`.
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
- Import order (`gci`) and named imports (`importas`) are enforced (`.golangci.yml`): standard library, `github.com/onsi` (ginkgo/gomega), third-party, `github.com/inference-gateway/*`, then this module; every non-stdlib import must be named — `k8s.io/api/<group>/<version>` and `apimachinery/pkg/apis/<group>/<version>` derive `<group><version>` (`corev1`, `metav1`), everything else its last path element; kubebuilder aliases `ctrl`, `apierrors`, `utilruntime`, `clientgoscheme`, `gwapiv1`, `corev1alpha1` are pinned. Fix with `golangci-lint fmt` and `golangci-lint run --fix`.
- Write self-explanatory code: clear names and small, single-purpose functions carry the intent.
  If a block needs a comment to be understood, extract it into a well-named function or variable.
- No inline comments inside function bodies.
- Doc comments on functions, types, and modules are at most 5 lines: what it does and why, not how.
- Tool directives are not comments and stay where the tool needs them (lint suppressions, build
  tags, compiler pragmas, code generation markers).
- Keep API types in `api/v1alpha1/*_types.go`, reconcilers in `internal/controller/*_controller.go`, tests alongside.
- Run `task fmt`, `task vet`, and `task lint` before submitting. The opt-in pre-commit hook (`task precommit:activate`) also regenerates manifests/deepcopy and runs `task test`, and fails if regeneration leaves unstaged changes — commit generated output together with the API change.
- Add/update tests when changing reconciliation behavior, CRD schemas, defaults, or validation.

## Shared provider list

`internal/providers/zz_generated_providers.go` is generated from the canonical `inference-gateway/schemas` OpenAPI enum. To add/remove a provider, run `task generate:providers` and commit the result; `task verify-shared-types` fails on drift. `ProviderSpec.Name` has no CRD enum — validation is runtime via `providers.IsSupported` (case-insensitive).

When offline, the drift test skips locally but hard-fails in CI. Hand-edit the file (alphabetical, gofmt-clean) and let CI run the real check. Quick offline check: `go test ./internal/providers/ -run 'TestIsSupported|TestSupportedProvidersHasNoDuplicates' -v`.

## Commits & releases

semantic-release + conventional commits. Subjects like `feat(gateway): Add route weighting`, `fix(agent): Resolve status update error`, `docs: Update install example`. For API changes, commit regenerated output from `task generate` and `task manifests`.

The operator image tag lives in `config/environments/prod/kustomization.yaml` (`images[].newTag`), which `task manifests` renders into `manifests/install.yaml`. At release time the local semantic-release plugin `.github/semantic-release/pin-operator-image.cjs` rewrites both to the released version and `@semantic-release/git` commits them, so the release tag (and an ArgoCD `path: manifests` install pinned to it) runs that release's image while `task manifests` still reproduces the committed manifest exactly. Self-check: `node .github/semantic-release/pin-operator-image.cjs`. The pinned semantic-release dependencies live next to it in `.github/semantic-release/package.json`; the release workflow installs them with `npm install --prefix .github/semantic-release`.

## Security

Never commit secrets in samples or manifests — use Kubernetes Secrets. Keep local cluster config out of the repo. Verify generated CRDs/install manifests before release-facing changes.
