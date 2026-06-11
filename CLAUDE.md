# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

Kubebuilder-scaffolded Kubernetes operator (`go.kubebuilder.io/v4` layout) that manages the Inference Gateway and its companions via CRDs in the `core.inference-gateway.com/v1alpha1` group. Go module: `github.com/inference-gateway/operator`. Target Kubernetes: v1.35+; built with Go 1.26+ and controller-runtime v0.24.

## Common commands

Everything goes through [Task](https://taskfile.dev) (`Taskfile.yaml`). The recommended way to get the full toolchain (`go`, `task`, `kubectl`, `kustomize`, `golangci-lint`, `k3d`, `ctlptl`, `controller-gen`, `prettier`, `gh`) at pinned versions is `flox activate`.

| Command | Purpose |
|---|---|
| `task generate` | Run `controller-gen` to regenerate `api/v1alpha1/zz_generated.deepcopy.go` |
| `task manifests` | Regenerate CRDs (`config/crd/bases/`), `manifests/crds.yaml`, and `manifests/install.yaml` — these are **version-controlled and shipped in releases** |
| `task fmt` / `task vet` / `task lint` / `task lint-fix` | gofmt / `go vet` / `golangci-lint run` (config in `.golangci.yml`) |
| `task test` | Unit + integration tests using envtest (auto-fetches Gateway API CRDs and the envtest binaries for K8s `1.35`) |
| `task test:e2e` | Ginkgo e2e suite — requires a running `k3d-dev` cluster created via `ctlptl` |
| `task test:e2e:focus FOCUS="..."` | Run a single Ginkgo focus from the e2e suite |
| `task cluster:create` / `task cluster:delete` | Provision/teardown the local `k3d-dev` cluster from `Cluster.yaml`, including cert-manager + Envoy Gateway + Gateway API CRDs |
| `task install` / `task uninstall` | Apply/remove CRDs to/from the current `~/.kube/config` cluster |
| `task deploy` / `task undeploy` | Build the image, `k3d image import` it, and apply `config/environments/dev` |
| `task run` | Run the controller locally against the current kubeconfig |
| `task manifests-for-namespace NAMESPACE=foo` | Rewrite `manifests/install.yaml` for a non-default namespace (default is `inference-gateway-system`) |

Run a single test package: `KUBEBUILDER_ASSETS="$(bin/setup-envtest use 1.35 -p path)" go test ./internal/controller/... -run TestControllers -ginkgo.focus="..."`.

`task build`, `task run`, `task deploy` all depend on `manifests` + `generate` + `fmt` + `vet`, so they regenerate before building. The pre-commit hook at `.githooks/pre-commit` (install via `task setup-pre-commit` or `task precommit:activate`) runs `task manifests generate lint fmt vet test` and **fails if anything is unstaged after regeneration** — so when you change API types or RBAC markers, stage the generated files too.

## Architecture

### CRDs and controllers

Four kinds, all namespaced, all under `core.inference-gateway.com/v1alpha1`:

- **`Gateway`** — the main deployable. Reconciles a `Deployment` + `Service` + optional Gateway API `Gateway`/`HTTPRoute` + optional HPA + `ServiceAccount`/RBAC, and writes `Status.URL`, `Status.MCPServers`, `Status.ProviderSummary`.
- **`Agent`** — A2A agent worker; reconciles a `Deployment` + `Service`. Discoverable by the `Orchestrator` via label selectors (the `Gateway` does **not** discover Agents).
- **`MCP`** — Model Context Protocol server; reconciles a `Deployment` + `Service`. Its `Status.URL` is the canonical URL consumed by `Gateway`/`Orchestrator`.
- **`Orchestrator`** — runs the Inference Gateway CLI's `channels-manager` daemon (Telegram-driven LLM loop). The Deployment is **always a singleton** (`replicas: 1`, `strategy: Recreate`) because Telegram only allows one `getUpdates` consumer per token. No Service is created (outbound-only).

Each reconciler lives in `internal/controller/<kind>_controller.go` with a sibling `_test.go` (Ginkgo+Gomega, envtest-backed via `suite_test.go`). The four reconcilers are wired up in `cmd/main.go`.

### Cross-CR coordination

- Both `Gateway` and `Orchestrator` implement **service discovery** via `metav1.LabelSelector` against `metadata.labels` on the target CRs (there is no opt-in field on the agent/MCP side beyond labels), but they discover different kinds: the **`Gateway` discovers `MCP` CRs only**, while the **`Orchestrator` discovers both `Agent` and `MCP` CRs**. The discovered set is unioned with the spec's static list, deduped, sorted, and fed to the workload — for the Gateway via the `MCP_SERVERS` env var; for the Orchestrator via two mounted ConfigMaps (`~/.infer/agents.yaml`, `~/.infer/mcp.yaml`). The orchestrator pod is rolled by a content-hash annotation when the rendered YAML changes.
- The Gateway controller watches `MCP` events and maps them back to affected Gateways via `mcpToGatewayRequests` in `gateway_controller.go`. Keep that mapping in sync if you add new discovery fields.
- Prefer `MCP.Status.URL` (TLS- and path-aware) over reconstructing the URL from `MCPSpec.Server`; `gatewayMCPURL` falls back to a deterministic construction only when status is empty.

### Namespace scoping

Every reconciler short-circuits via `shouldWatchNamespace(ctx, req.Namespace)`, which honors the `WATCH_NAMESPACE_SELECTOR` env var (a label selector on `Namespace` objects). Empty selector means watch everything. If you add a new reconciler, replicate this check.

### Manifests and environments

- `config/` is the source of truth for kustomize bases (`crd/`, `rbac/`, `operator/`, `metrics/`, `prometheus/`, `network-policy/`, `samples/`).
- `config/environments/{dev,prod}/` are kustomize overlays. `task deploy` applies `dev`; `task manifests` bakes `prod` into `manifests/install.yaml`.
- `manifests/install.yaml` and `manifests/crds.yaml` are checked in and shipped via release assets — **do not hand-edit them**; regenerate with `task manifests`.

### Releases

Conventional commits → semantic-release (`.releaserc.yaml`) cuts versions from `main` (latest) and `rc/*` (rc channel). The release pipeline expects the regenerated `manifests/*.yaml`, so any PR that touches `api/`, `config/rbac/`, or kubebuilder markers must also commit the regenerated manifests.
