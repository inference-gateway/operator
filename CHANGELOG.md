# Changelog

All notable changes to this project will be documented in this file.

## [0.17.0](https://github.com/inference-gateway/operator/compare/v0.16.3...v0.17.0) (2026-07-05)

## [0.16.3](https://github.com/inference-gateway/operator/compare/v0.16.2...v0.16.3) (2026-05-25)

### 👷 CI

* **claude:** Add maintainer skill ([ae34e22](https://github.com/inference-gateway/operator/commit/ae34e22132f54ff5486ae402dd76563fd611a210))
* **deps:** Bump anthropics/claude-code-action  v1.0.131 -> v1.0.133 ([dce81a2](https://github.com/inference-gateway/operator/commit/dce81a279c99a3ecc027e6b2c9adb644ce7160e3))
* **deps:** Bump the github-actions group with 2 updates ([#77](https://github.com/inference-gateway/operator/issues/77)) ([4047604](https://github.com/inference-gateway/operator/commit/404760414e49a60b153df38ab6782aea23d825d7))
* **deps:** Update Claude Code Action to version 1.0.131 ([dd25be1](https://github.com/inference-gateway/operator/commit/dd25be1c424ab9e55ea90c31e778288af0a4ef0c))

### 🔧 Miscellaneous

* **deps:** Bump dev dependecies to latest ([add8210](https://github.com/inference-gateway/operator/commit/add8210b1be0e2875a6e30a7bce7b5166d24d098))
* **deps:** Bump dev dependencies ([7954ec8](https://github.com/inference-gateway/operator/commit/7954ec808ef628eb4577b3a9d17e676291f9fed9))
* **deps:** Bump prettier to version 3.8.3 ([864dd49](https://github.com/inference-gateway/operator/commit/864dd499467663f0a762182664581e3032ea5ac9))
* **flox:** Bump dev dependecies to latest ([7e08dfc](https://github.com/inference-gateway/operator/commit/7e08dfceb9267c38c009824fbb5b5cc0b133fa3e))
* **license:** Update license to Apache 2.0 ([5d58997](https://github.com/inference-gateway/operator/commit/5d589976aaed157a4bf9f812c12f7b304d745be3))
* Replace em dashes with normal dashes ([1fbb15a](https://github.com/inference-gateway/operator/commit/1fbb15a369eef9801647c05c6961d497f50ed475))
* Update license template ([dc08e56](https://github.com/inference-gateway/operator/commit/dc08e564514f211c77d8f871fbbf4ca45d99e458))

## [0.16.2](https://github.com/inference-gateway/operator/compare/v0.16.1...v0.16.2) (2026-05-22)

### 👷 CI

* **claude:** Simplify conditions for triggering Claude Code actions ([95db957](https://github.com/inference-gateway/operator/commit/95db957070909584a300ca38c13b2f8b209f3b2e))
* **deps:** Update claude-code-action to version 1.0.130 ([f397353](https://github.com/inference-gateway/operator/commit/f39735328bbaa4a263e29ed09976443cc4c270eb))

### 📚 Documentation

* **release:** Add operator installation instructions to release notes ([#74](https://github.com/inference-gateway/operator/issues/74)) ([5fd1d3e](https://github.com/inference-gateway/operator/commit/5fd1d3ef97b0aaeaa5b328ee138d73b8ccc8c8d5)), closes [#73](https://github.com/inference-gateway/operator/issues/73)

### 🔧 Miscellaneous

* **dependabot:** Add ignore rule for golang versions >=1.26.3 ([987149f](https://github.com/inference-gateway/operator/commit/987149fd8fa5154bf88c6ad14b0825da28c6cbbf))
* **dependabot:** Update golang and ubuntu version ignore rules in dependabot configuration ([bbd4df6](https://github.com/inference-gateway/operator/commit/bbd4df61d756180072c70857ee59879d3711550f))
* **deps:** Bump anthropics/claude-code-action ([#72](https://github.com/inference-gateway/operator/issues/72)) ([1865a45](https://github.com/inference-gateway/operator/commit/1865a45d939d1fe9ab05cc95a2126eea6afcc102))
* **deps:** Bump sigs.k8s.io/gateway-api from 1.2.1 to 1.5.1 in the gomod group ([#71](https://github.com/inference-gateway/operator/issues/71)) ([e4dac43](https://github.com/inference-gateway/operator/commit/e4dac4322f441de1a56fb0b7b427ea540a0e0511))
* **deps:** Update claude-code version to 2.1.141 and infer.flake to v0.109.11 ([365c104](https://github.com/inference-gateway/operator/commit/365c104ff2e8b9fe15577ebf5ef850513877d35a))

## [0.16.1](https://github.com/inference-gateway/operator/compare/v0.16.0...v0.16.1) (2026-05-21)

### ♻️ Improvements

* **examples:** Rename mcp-memory-server binary to mcp-server ([1bb090f](https://github.com/inference-gateway/operator/commit/1bb090f07e550af9e06ba69e2d9a0b86bfa60e01))

### 🐛 Bug Fixes

* **agent:** Populate status.card so kubectl get agents renders metadata ([#70](https://github.com/inference-gateway/operator/issues/70)) ([87fa392](https://github.com/inference-gateway/operator/commit/87fa392ed58c7217569983fb7d06d4ba9435a8b3))
* **mcp:** Create the MCP config properly as described in the cli repo ([1ba2e52](https://github.com/inference-gateway/operator/commit/1ba2e5257d040a3905dccae9100a1fec74defbc4))

## [0.16.0](https://github.com/inference-gateway/operator/compare/v0.15.0...v0.16.0) (2026-05-21)

### ✨ Features

* **mcp:** Add label-selector service discovery for MCP servers ([#62](https://github.com/inference-gateway/operator/issues/62)) ([e5eddda](https://github.com/inference-gateway/operator/commit/e5eddda01ca9343e1b43b7b0d5ef412675c346b7))

### ♻️ Improvements

* **examples:** Replace broken MCP image with in-tree Go server ([#65](https://github.com/inference-gateway/operator/issues/65)) ([60bedd0](https://github.com/inference-gateway/operator/commit/60bedd01ee9427a764d5ccfebccc32b385d44f4d)), closes [#63](https://github.com/inference-gateway/operator/issues/63)
* **gateway:** Migrate routing from Ingress to Gateway API ([#60](https://github.com/inference-gateway/operator/issues/60)) ([2fcaa0c](https://github.com/inference-gateway/operator/commit/2fcaa0c356eb8e0c2cd649796fc5075329a6530b))

### 🔧 Miscellaneous

* **deps:** Use explicit go version ([0f6db1c](https://github.com/inference-gateway/operator/commit/0f6db1c3b49226ab6f298fba6dd90562b088e115))
* **examples-deps:** Bump the go_modules group across 1 directory with 3 updates ([#66](https://github.com/inference-gateway/operator/issues/66)) ([e2a4d3d](https://github.com/inference-gateway/operator/commit/e2a4d3dafe38a5531ceccc1f212f2a5fd4e7e38c))
* **examples-deps:** Bump the go_modules group across 1 directory with 3 updates ([#68](https://github.com/inference-gateway/operator/issues/68)) ([017fd99](https://github.com/inference-gateway/operator/commit/017fd99c7f18fecc7a91a3523abd7ca389ad1ac5))
* **examples-deps:** Bump the go_modules group across 1 directory with 7 updates ([#64](https://github.com/inference-gateway/operator/issues/64)) ([6aeb13d](https://github.com/inference-gateway/operator/commit/6aeb13d8b70f50c4dede908459e0696f1fd5e48f))
* **examples-deps:** Bump the go_modules group across 1 directory with 7 updates ([#67](https://github.com/inference-gateway/operator/issues/67)) ([7889588](https://github.com/inference-gateway/operator/commit/788958808e433859f9d131ac9feee94eec77934d))
* Update k3s image version and add Envoy Gateway installation tasks ([a61aa2a](https://github.com/inference-gateway/operator/commit/a61aa2acb2737468d77bf2ea269947e481f28110))

## [0.15.0](https://github.com/inference-gateway/operator/compare/v0.14.2...v0.15.0) (2026-05-20)

### ✨ Features

* **agent:** Support resource requests/limits on Agent CRD ([#59](https://github.com/inference-gateway/operator/issues/59)) ([fbc9b12](https://github.com/inference-gateway/operator/commit/fbc9b12e955d470cc160eeb2ddd46cad2f664064)), closes [#58](https://github.com/inference-gateway/operator/issues/58)

### ♻️ Improvements

* Remove devcontainer environment ([f03d7f9](https://github.com/inference-gateway/operator/commit/f03d7f947db271d9425f2926086aac7a9c2b7ef2))
* Remove old copilot-instructions.md file ([9c9c28e](https://github.com/inference-gateway/operator/commit/9c9c28e60b3eabe8c6feea78424a0fe66a5627fc))

### 👷 CI

* **dependabot:** Add dependabot to help with dependecies upgrades ([6a714ab](https://github.com/inference-gateway/operator/commit/6a714ab6ce39af29875ac060c50b521ed92b6917))
* **deps:** Bump the github-actions group with 3 updates ([#55](https://github.com/inference-gateway/operator/issues/55)) ([7f3f8f1](https://github.com/inference-gateway/operator/commit/7f3f8f160572b523da3d0cd63a936a34b7460978))
* **deps:** Update workflow actions to use official GitHub actions for task and golangci-lint installation ([239816c](https://github.com/inference-gateway/operator/commit/239816c44c02314c10079da916d10aa3cb7d7121))
* Enable display report for Claude Code action ([88448df](https://github.com/inference-gateway/operator/commit/88448df867688e7eb4319136fb973f789bfef14f))

### 🔧 Miscellaneous

* Add CODEOWNERS ([636f738](https://github.com/inference-gateway/operator/commit/636f7389ac9a1225833fc9f6c7bf2198f698c12a))
* Add flox lock file ([1928cab](https://github.com/inference-gateway/operator/commit/1928cab3b66c90f2802d4bdc2622587cd95fa750))
* **deps:** Bump dev dependecies to latest ([26dcd23](https://github.com/inference-gateway/operator/commit/26dcd23f9231314ca15d2e8c46524dd17d586be0))
* **deps:** Bump golang to version 1.26.2 ([8a64a4f](https://github.com/inference-gateway/operator/commit/8a64a4f715f54f0950639f7f3336695d6201e91c))
* **dev-deps:** Add infer to flox environment ([05ca18a](https://github.com/inference-gateway/operator/commit/05ca18a676350af2faf594abcc880ce0d4b5808e))
* Fix TASK_VERSION format in CI workflow ([4dfd466](https://github.com/inference-gateway/operator/commit/4dfd4661433e7a1fc2c1d6c2393e7a380fce4529))
* Remove outdated issue templates for bug reports, feature requests, and refactor requests ([dcab50a](https://github.com/inference-gateway/operator/commit/dcab50a54a96c9d19deb150dca7a484d5681e745))

### 🔨 Miscellaneous

* **deps:** Bump anthropics/claude-code-action ([#57](https://github.com/inference-gateway/operator/issues/57)) ([08c0aa0](https://github.com/inference-gateway/operator/commit/08c0aa04bd4ef9a22df9c1dbf4b35dee2ae8ee01))
* **deps:** Bump the gomod group with 2 updates ([#56](https://github.com/inference-gateway/operator/issues/56)) ([2ca47a2](https://github.com/inference-gateway/operator/commit/2ca47a23f21922f1fca93caf9a183c41043f41a8))
* **deps:** Bump the gomod group with 6 updates ([#54](https://github.com/inference-gateway/operator/issues/54)) ([5b76864](https://github.com/inference-gateway/operator/commit/5b76864dc8dfc777c5ea42be477e0ce444fe4a03))

## [0.14.2](https://github.com/inference-gateway/operator/compare/v0.14.1...v0.14.2) (2026-05-07)

### 🐛 Bug Fixes

* **ci:** Update golangci-lint installation script and version to v2.12.2 ([4386e83](https://github.com/inference-gateway/operator/commit/4386e83a4164d3b0c420dfbabcb665269bfc9131))
* **workflow:** Update Claude Code action to v1.0.114 and refine system prompt ([fb13700](https://github.com/inference-gateway/operator/commit/fb13700d59162702d053794c8a3fd60c155344d0))

### 👷 CI

* Bump actions/create-github-app-token to latest ([ee5d483](https://github.com/inference-gateway/operator/commit/ee5d4835e4cd2f4128328be2fd04573793b4e6b8))
* **deps:** Bump golangci-lint to latest ([c3ad612](https://github.com/inference-gateway/operator/commit/c3ad612bbf271255303be56a52773507c2f81a3c))

## [0.14.1](https://github.com/inference-gateway/operator/compare/v0.14.0...v0.14.1) (2026-04-29)

### ♻️ Improvements

* **examples:** Add Redis storage backend for persistent conversations ([#53](https://github.com/inference-gateway/operator/issues/53)) ([3496ff9](https://github.com/inference-gateway/operator/commit/3496ff9dbcd859b774b660da629ae55c58b2129e))

## [0.14.0](https://github.com/inference-gateway/operator/compare/v0.13.0...v0.14.0) (2026-04-29)

### ✨ Features

* **orchestrator:** Add Kubernetes service discovery of A2A Agents ([#51](https://github.com/inference-gateway/operator/issues/51)) ([c1d9c09](https://github.com/inference-gateway/operator/commit/c1d9c09642290a4c7990b198eb393c0da21f5103)), closes [#49](https://github.com/inference-gateway/operator/issues/49)

### ♻️ Improvements

* **agent:** Simplify Agent CRD with sane defaults and A2A_AGENT_CLIENT_* env vars ([#50](https://github.com/inference-gateway/operator/issues/50)) ([efafae9](https://github.com/inference-gateway/operator/commit/efafae90cc47c95df9f7ff3578c2fa3fd9d244b0)), closes [#44](https://github.com/inference-gateway/operator/issues/44)
* **examples:** Reorganize examples into dedicated directories with READMEs ([#48](https://github.com/inference-gateway/operator/issues/48)) ([53a67f6](https://github.com/inference-gateway/operator/commit/53a67f60c53850b6300b3c5bc1890b412da0a0bb)), closes [#45](https://github.com/inference-gateway/operator/issues/45)
* **gateway:** Remove deprecated A2A wiring from Gateway controller and CRD ([#47](https://github.com/inference-gateway/operator/issues/47)) ([82449a2](https://github.com/inference-gateway/operator/commit/82449a23796c0a4c68c16f7b6eda3c8ee1128ca9)), closes [#46](https://github.com/inference-gateway/operator/issues/46)

## [0.13.0](https://github.com/inference-gateway/operator/compare/v0.12.4...v0.13.0) (2026-04-29)

### ✨ Features

* Implement Orchestrator custom resource for chat platform integrations ([#43](https://github.com/inference-gateway/operator/issues/43)) ([9f1877d](https://github.com/inference-gateway/operator/commit/9f1877d009d3765bd461ead9b10abea4d82d7b18))

### 📚 Documentation

* Bump the operator version to latest ([cb6eb40](https://github.com/inference-gateway/operator/commit/cb6eb40376d0ca78bcf4c52e6bde5cdd0e2b3826))
* Reference the usage of go 1.26+ ([6440ec5](https://github.com/inference-gateway/operator/commit/6440ec53ef5630627239d35df10b7b33a2b72896))

## [0.12.4](https://github.com/inference-gateway/operator/compare/v0.12.3...v0.12.4) (2026-04-26)

### 👷 CI

* **deps:** Bump docker buildx action to v4 ([ccc2d3d](https://github.com/inference-gateway/operator/commit/ccc2d3d390a62ecdbf2bfba9115680e710afd198))
* **deps:** Bump goreleaser to version 2.15.2 ([be720da](https://github.com/inference-gateway/operator/commit/be720da43875149a781acd22c872d24a60f22a81))
* **fix:** Bump cosign-installer to version 4.1.1 ([e3b8a69](https://github.com/inference-gateway/operator/commit/e3b8a6940096cf58b94d5cfe1f9165205aec6bad))

## [0.12.3](https://github.com/inference-gateway/operator/compare/v0.12.2...v0.12.3) (2026-04-26)

### 🐛 Bug Fixes

* Add version prefix ([ee9e4c0](https://github.com/inference-gateway/operator/commit/ee9e4c0cfd545c80889529230e2ba69947e85f93))

## [0.12.2](https://github.com/inference-gateway/operator/compare/v0.12.1...v0.12.2) (2026-04-26)

### 👷 CI

* Bump all actions to their latest ([7673221](https://github.com/inference-gateway/operator/commit/76732216fa87cb67fe8275e0f55241e55a6bf28e))

### 📚 Documentation

* Add Flox environment as recommended dev setup, refresh versions ([183a5f5](https://github.com/inference-gateway/operator/commit/183a5f5f4383c7635a188ea18eca9f5bfbeb9803))

### 🔨 Miscellaneous

* **deps:** Bump aquasecurity/trivy-action ([#38](https://github.com/inference-gateway/operator/issues/38)) ([4f47ce1](https://github.com/inference-gateway/operator/commit/4f47ce14198f9e6d9f6098fc3d256b286b79cd66))
* **deps:** Bump go.opentelemetry.io/otel/sdk ([#37](https://github.com/inference-gateway/operator/issues/37)) ([c4706ea](https://github.com/inference-gateway/operator/commit/c4706ea69e2103b72f414ac22787e8f6a68c6758))
* **deps:** Bump the go_modules group across 1 directory with 3 updates ([#41](https://github.com/inference-gateway/operator/issues/41)) ([a92a4d7](https://github.com/inference-gateway/operator/commit/a92a4d7257f3cac535510395b9043d4aeaed1e11))
* **deps:** Bump to kubebuilder v4.13.1 dependency baseline ([#40](https://github.com/inference-gateway/operator/issues/40)) ([4ed5893](https://github.com/inference-gateway/operator/commit/4ed5893ddb810acb0120eb98182456b70336f0ba))

## [0.12.1](https://github.com/inference-gateway/operator/compare/v0.12.0...v0.12.1) (2025-08-01)

### ♻️ Improvements

* **api:** Rename A2A Custom Resource Definition to Agent ([#36](https://github.com/inference-gateway/operator/issues/36)) ([e68cadf](https://github.com/inference-gateway/operator/commit/e68cadfc33019d07fd32531f746931a48afba650))

### 🔧 Miscellaneous

* **issue-templates:** Add bug report, feature request, and refactor request templates ([dd77567](https://github.com/inference-gateway/operator/commit/dd77567b1aac9c737c5fbe01cbbd44fc580c9fb0))

## [0.12.0](https://github.com/inference-gateway/operator/compare/v0.11.1...v0.12.0) (2025-07-29)

### ✨ Features

* **gateway:** Add ServiceAccount for RBAC configuration to Gateway CRD ([#33](https://github.com/inference-gateway/operator/issues/33)) ([dda8cc6](https://github.com/inference-gateway/operator/commit/dda8cc65ccb2deff9b300c7e1e48430006c50004))

### ♻️ Improvements

* **a2a:** Remove A2A_SERVICE_DISCOVERY_ENDPOINTS environment variable ([#34](https://github.com/inference-gateway/operator/issues/34)) ([987171c](https://github.com/inference-gateway/operator/commit/987171c82c2383b4945ebd2ee2844c2bd8e1d995)), closes [#29](https://github.com/inference-gateway/operator/issues/29)
* **controller:** Rename ENABLE_AUTH environment variable to AUTH_ENABLE ([#30](https://github.com/inference-gateway/operator/issues/30)) ([86fe434](https://github.com/inference-gateway/operator/commit/86fe43417148859006bccd19b1590f249f2f4e15)), closes [#28](https://github.com/inference-gateway/operator/issues/28)
* **gateway:** Rename ENABLE_TELEMETRY to TELEMETRY_ENABLE ([#31](https://github.com/inference-gateway/operator/issues/31)) ([adbaefc](https://github.com/inference-gateway/operator/commit/adbaefce0fee88503b077f5c18b70a31046abcd5)), closes [#26](https://github.com/inference-gateway/operator/issues/26)

### 🔨 Miscellaneous

* **deps:** Bump the go_modules group across 1 directory with 2 updates ([#24](https://github.com/inference-gateway/operator/issues/24)) ([76d5a51](https://github.com/inference-gateway/operator/commit/76d5a51b6620f0d7d67f26ed051a232494b959bd))

## [0.11.1](https://github.com/inference-gateway/operator/compare/v0.11.0...v0.11.1) (2025-07-29)

### 🐛 Bug Fixes

* **a2a:** Rename A2A_SERVICE_DISCOVERY_ENABLED to A2A_SERVICE_DISCOVERY_ENABLE ([#27](https://github.com/inference-gateway/operator/issues/27)) ([78ed5c0](https://github.com/inference-gateway/operator/commit/78ed5c071de902b4e8f36d378c57a1f953aca4b9)), closes [#25](https://github.com/inference-gateway/operator/issues/25)

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
