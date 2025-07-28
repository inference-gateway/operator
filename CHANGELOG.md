# Changelog

All notable changes to this project will be documented in this file.

## [0.11.0](https://github.com/inference-gateway/operator/compare/v0.10.0...v0.11.0) (2025-07-28)

### ✨ Features

* **a2a:** Add automatic pod restart on configuration changes ([#20](https://github.com/inference-gateway/operator/issues/20)) ([307fee2](https://github.com/inference-gateway/operator/commit/307fee286f6d81b4e0de2f020e32386d50bd6584))

### ♻️ Improvements

* **a2a:** Simplify Service Discovery to use CRD-based approach ([#21](https://github.com/inference-gateway/operator/issues/21)) ([121e33b](https://github.com/inference-gateway/operator/commit/121e33bacdb08d1e0109ca77946902b201a4b17b)), closes [#19](https://github.com/inference-gateway/operator/issues/19)

### 👷 CI

* Consolidate linting and build processes into a single CI workflow ([#22](https://github.com/inference-gateway/operator/issues/22)) ([f4591e9](https://github.com/inference-gateway/operator/commit/f4591e94275ce37d63efcedfae0c0bd429bd78f8))

### 📚 Documentation

* **examples:** Add Google provider support ([#23](https://github.com/inference-gateway/operator/issues/23)) ([879e762](https://github.com/inference-gateway/operator/commit/879e762ad5477e5d159e5d0647f57f985fcfa720)), closes [#17](https://github.com/inference-gateway/operator/issues/17)

## [0.10.0](https://github.com/inference-gateway/operator/compare/v0.9.0...v0.10.0) (2025-07-27)

### ✨ Features

* **a2a:** Add service discovery configuration to Gateway CRD ([#16](https://github.com/inference-gateway/operator/issues/16)) ([e96edce](https://github.com/inference-gateway/operator/commit/e96edce476d6336d73c007ad814ac4435402fa41))

### 👷 CI

* Add Claude Code GitHub Workflow ([#15](https://github.com/inference-gateway/operator/issues/15)) ([cb6f20b](https://github.com/inference-gateway/operator/commit/cb6f20b7315513de2682d6cef82e32c28f75c2c3))

### 📚 Documentation

* Add CLAUDE.md for project guidance and development workflow ([f10a8f6](https://github.com/inference-gateway/operator/commit/f10a8f6177b8fc84c8d8d7053c9ec6c6982a5f5e))

### 🔧 Miscellaneous

* Add new line at the end of the file ([6fe02fa](https://github.com/inference-gateway/operator/commit/6fe02fa80c420bce17ea07c1cba39e2f56e6fd7d))
* Update custom instructions to include pre-commit hook installation ([e038f10](https://github.com/inference-gateway/operator/commit/e038f10b6d3ced3f7b6a8e00f7ebf0119faa6c8f))

## [0.9.0](https://github.com/inference-gateway/operator/compare/v0.8.0...v0.9.0) (2025-06-28)

### ✨ Features

* Implement MCP (Model Context Protocol) resource and controller with RBAC roles ([#12](https://github.com/inference-gateway/operator/issues/12)) ([ee8c869](https://github.com/inference-gateway/operator/commit/ee8c8690c7daf13663ff37f1e54c45b926838e01))

### 📚 Documentation

* **fix:** Correct emoji rendering in API Overview section of README ([0481b4f](https://github.com/inference-gateway/operator/commit/0481b4f104f52f8836f62827c80aef60461c31c0))

## [0.8.0](https://github.com/inference-gateway/operator/compare/v0.7.0...v0.8.0) (2025-06-27)

### ✨ Features

* Add A2AServer custom resource and controller ([#4](https://github.com/inference-gateway/operator/issues/4)) ([27d0161](https://github.com/inference-gateway/operator/commit/27d0161d8b8eb1ed0e13411463bc0f1f5d7b8b1b))

## [0.7.0](https://github.com/inference-gateway/operator/compare/v0.6.0...v0.7.0) (2025-06-26)

### ✨ Features

* Add URL field to GatewayStatus and update related configurations ([188e30a](https://github.com/inference-gateway/operator/commit/188e30a638a42cf4d5310cb1198db26a66ac6fd4))

### 🐛 Bug Fixes

* Simplify reconcileGatewayStatus and improve URL update logic ([4315965](https://github.com/inference-gateway/operator/commit/4315965df6319e8106a39ae67777878f6da0b505))

## [0.6.0](https://github.com/inference-gateway/operator/compare/v0.5.3...v0.6.0) (2025-06-25)

### ✨ Features

* Add 'enabled' field to ProviderSpec and update related configurations ([3cca5a1](https://github.com/inference-gateway/operator/commit/3cca5a152694cf9a463c8efb73ef1029c3223e25))

## [0.5.3](https://github.com/inference-gateway/operator/compare/v0.5.2...v0.5.3) (2025-06-25)

### ♻️ Improvements

* Improve the overall configurations experience of the Gateway ([#9](https://github.com/inference-gateway/operator/issues/9)) ([e06faf5](https://github.com/inference-gateway/operator/commit/e06faf5a5ae839fb4dfc803ae9d2b811271a60fa))

### 📚 Documentation

* **fix:** Update secret names and API URLs in gateway configuration files ([aa1b104](https://github.com/inference-gateway/operator/commit/aa1b104ea847d2b4e5ef486d0b5fe5a3040e3e5a))
* Update AI provider configuration to use environment variables from ConfigMap and Secret ([46162e7](https://github.com/inference-gateway/operator/commit/46162e7cc52f5d23435e053a6fbb4f3b90d540aa))

## [0.5.2](https://github.com/inference-gateway/operator/compare/v0.5.1...v0.5.2) (2025-06-23)

### 🐛 Bug Fixes

* Make deployment more configurable ([#8](https://github.com/inference-gateway/operator/issues/8)) ([f21d13d](https://github.com/inference-gateway/operator/commit/f21d13df8b93ed28fcd5e80503995f067250b661))

## [0.5.1](https://github.com/inference-gateway/operator/compare/v0.5.0...v0.5.1) (2025-06-23)

### ♻️ Improvements

* Remove redundant comment in e2e test for controller pod description ([d8c3a32](https://github.com/inference-gateway/operator/commit/d8c3a32d2b41b645002a9a045018ae933011c351))

### 🐛 Bug Fixes

* Correct symlink path for pre-commit hook activation ([84c19a2](https://github.com/inference-gateway/operator/commit/84c19a2d8443eebab12bb9a73e115b1cf0a13000))
* Make ssl configuration of nginx more explicit ([#5](https://github.com/inference-gateway/operator/issues/5)) ([8c9875c](https://github.com/inference-gateway/operator/commit/8c9875c47439ca1892d5b7ce932bb29534982020))
* Preserve existing annotations when updating deployment template ([#7](https://github.com/inference-gateway/operator/issues/7)) ([acfa571](https://github.com/inference-gateway/operator/commit/acfa57153b76342d55356d71067b795a1facdf55))

### 🔨 Miscellaneous

* Add .gitattributes to manage linguist settings for hooks and manifests ([87405d0](https://github.com/inference-gateway/operator/commit/87405d05c0d20b36c2140b4565d40bf55d9d7596))
* Add missing test task to pre-commit hook for improved code quality ([656ca7a](https://github.com/inference-gateway/operator/commit/656ca7a66356c05e59abd57bbd970943a5fe4e13))
* Add operator-sdk installation and update zsh autocompletions ([7e8a19e](https://github.com/inference-gateway/operator/commit/7e8a19e6c852688f49223b55e7e7eb2eca71d9b0))
* Add pre-commit hook and update Taskfile for activation/deactivation ([7a18069](https://github.com/inference-gateway/operator/commit/7a18069ada2d8b4af01c8aaad878797d9568bfd7))

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
