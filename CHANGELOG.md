# Changelog

All notable changes to this project will be documented in this file.

## [0.5.0](https://github.com/inference-gateway/operator/compare/v0.4.0...v0.5.0) (2025-06-23)

### ✨ Features

* Enhance Ingress configuration with host and TLS options ([#3](https://github.com/inference-gateway/operator/issues/3)) ([83fb37c](https://github.com/inference-gateway/operator/commit/83fb37c30cec55e8c404233ff8b693588ef2f3ff))

### ♻️ Improvements

* Improve logging verbosity and streamline HPA reconciliation logic ([3e48073](https://github.com/inference-gateway/operator/commit/3e48073643fdde4b7a7f15cfe4638299a4e10d92))

## [0.4.0](https://github.com/inference-gateway/operator/compare/v0.3.5...v0.4.0) (2025-06-22)

### ✨ Features

* Add Horizontal Pod Auto Scaling (HPA) to Gateway ([#2](https://github.com/inference-gateway/operator/issues/2)) ([6d766e9](https://github.com/inference-gateway/operator/commit/6d766e943d10f8d20822aa0ce22cfe7751c62007))

## [0.3.5](https://github.com/inference-gateway/operator/compare/v0.3.4...v0.3.5) (2025-06-21)

### 🐛 Bug Fixes

* **examples:** Add Namespace definition to gateway-complete and gateway-minimal examples ([344840c](https://github.com/inference-gateway/operator/commit/344840c41d451dbdba3b579f53ec8725e3b74e8e))
* **tests:** Ensure test namespace is labeled and cleaned up properly ([acaba15](https://github.com/inference-gateway/operator/commit/acaba156f2f0f24fb600ec948471c350a8a2eb6d))

## [0.3.4](https://github.com/inference-gateway/operator/compare/v0.3.3...v0.3.4) (2025-06-21)

### 🐛 Bug Fixes

* **rbac:** Add permissions for namespaces to ClusterRole and Role ([bf6feff](https://github.com/inference-gateway/operator/commit/bf6feffb7cda38be001b4cdbd71c11d11e0dd56f))

## [0.3.3](https://github.com/inference-gateway/operator/compare/v0.3.2...v0.3.3) (2025-06-21)

### ♻️ Improvements

* Enhance namespace watching and metrics configuration in operator ([10b5f30](https://github.com/inference-gateway/operator/commit/10b5f3004014fcb4a34d225aef76bbb6fbca1ad1))
* **examples:** Remove operator related configurations from the examples, WATCH_NAMESPACE_SELECTOR is configured on the operator not on the consumer / user Gateway crd. ([fd139dd](https://github.com/inference-gateway/operator/commit/fd139dd7cb5a74e09dc00ad701743137cb2f98e5))

### 🐛 Bug Fixes

* Refactor operator references and update service account names in manifests ([7d33e62](https://github.com/inference-gateway/operator/commit/7d33e626103368626695f5d7c12c17d32411971c))
* Update deployment path from 'manager' to 'operator' in Taskfile ([e268fc9](https://github.com/inference-gateway/operator/commit/e268fc95e3db3254f1fa76f62586b44e9179cc40))

### ✅ Miscellaneous

* **fix:** Rename context from 'Manager' to 'Operator' in e2e tests ([823050a](https://github.com/inference-gateway/operator/commit/823050aa949e9633fbce1e8943ade0e15f41f88c))

## [0.3.2](https://github.com/inference-gateway/operator/compare/v0.3.1...v0.3.2) (2025-06-21)

### 🐛 Bug Fixes

* Rename 'manager' to 'operator' in Dockerfiles and Kubernetes manifests ([76f1309](https://github.com/inference-gateway/operator/commit/76f130936f736331cc706e9e3b33d77f7d51772b))

## [0.3.1](https://github.com/inference-gateway/operator/compare/v0.3.0...v0.3.1) (2025-06-21)

### 🐛 Bug Fixes

* Keep it simple - refactor artifact uploads and add CRD manifest ([15352ef](https://github.com/inference-gateway/operator/commit/15352ef4d22524d4fa970f45dca94c3f36cf0332))
* Update namespace references from 'operator-system' to 'inference-gateway-system' ([627dfe7](https://github.com/inference-gateway/operator/commit/627dfe73e184852e554efb1d7c9511731bbdb755))

## [0.3.0](https://github.com/inference-gateway/operator/compare/v0.2.1...v0.3.0) (2025-06-21)

### ✨ Features

* Add CRD and installation manifests for Inference Gateway ([997ca32](https://github.com/inference-gateway/operator/commit/997ca32af714464fd687651f29bab06571c462a9))

### 🐛 Bug Fixes

* Correct emoji in the Table of Contents for Installation section ([5713913](https://github.com/inference-gateway/operator/commit/5713913464802b37da8a5bd8be347405fc38a97d))

## [0.2.1](https://github.com/inference-gateway/operator/compare/v0.2.0...v0.2.1) (2025-06-21)

### 🐛 Bug Fixes

* Remove darwin and arm support from builds in goreleaser configuration ([c914265](https://github.com/inference-gateway/operator/commit/c91426521959850320615f58a1cf545387ade584))
* Remove standalone binaries artifact upload step from GitHub Actions workflow, keep only the container push ([dd5c03e](https://github.com/inference-gateway/operator/commit/dd5c03ebffc4f2d4540fc39d99b8cbbc7310343c))

## [0.2.0](https://github.com/inference-gateway/operator/compare/v0.1.1...v0.2.0) (2025-06-21)

### ✨ Features

* Add GitHub Actions workflow for artifact management and container signing ([c8a228d](https://github.com/inference-gateway/operator/commit/c8a228dc2a558bf9d1bba876ad473a0c5dac008a))

### 👷 CI

* Update Go setup and linter versions in lint.yml ([6e0033f](https://github.com/inference-gateway/operator/commit/6e0033feba073a25ed3357532b0ec8c713abe103))

### 🔧 Miscellaneous

* Add .editorconfig for consistent code formatting ([f3f7292](https://github.com/inference-gateway/operator/commit/f3f7292e8150fe293d2c32eccc50902254e455a0))

## [0.1.1](https://github.com/inference-gateway/operator/compare/v0.1.0...v0.1.1) (2025-06-21)

### ♻️ Improvements

* Rename task commands for cluster management in Taskfile and update E2E test script ([1631bd4](https://github.com/inference-gateway/operator/commit/1631bd42b77cc4d798e7925b8ade2d10b06ee7c6))

### 🔧 Miscellaneous

* Update Go version to 1.24 in CONTRIBUTING.md and README.md ([4e42997](https://github.com/inference-gateway/operator/commit/4e42997f1230ad573bba8387f6772f80b58ddf1a))
* Update Go version to 1.24 in Dockerfile and go.mod ([f603e65](https://github.com/inference-gateway/operator/commit/f603e65ebf86364dfe4d66b88ee4730ad142a695))
