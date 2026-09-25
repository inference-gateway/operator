<div align="center">

# Inference Gateway Operator

> **⚠️ EARLY STAGE PROJECT WARNING**  
> This project is currently in its early development stages. Breaking changes are expected and the API may change significantly between releases. Use with caution in production environments and expect potential migration requirements when upgrading versions.

**A Kubernetes operator for automating the deployment and management of Inference Gateway instances**

[![Go Version](https://img.shields.io/github/go-mod/go-version/inference-gateway/operator?style=flat-square)](https://golang.org/)
[![License: Apache 2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg?style=flat-square)](https://www.apache.org/licenses/LICENSE-2.0)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.35+-blue.svg?style=flat-square&logo=kubernetes)](https://kubernetes.io/)
[![Docker](https://img.shields.io/badge/Docker-Available-blue.svg?style=flat-square&logo=docker)](https://hub.docker.com/)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-3.0-green.svg?style=flat-square)](https://swagger.io/specification/)

[![Latest Release](https://img.shields.io/github/v/release/inference-gateway/operator?style=flat-square&logo=github)](https://github.com/inference-gateway/operator/releases/latest)
[![Container Registry](https://img.shields.io/badge/Container-ghcr.io-blue.svg?style=flat-square&logo=github)](https://github.com/inference-gateway/operator/pkgs/container/operator)
[![Multi-Arch](https://img.shields.io/badge/Architecture-amd64%20%7C%20arm64-green?style=flat-square)](https://github.com/inference-gateway/operator/releases)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg?style=flat-square)](https://github.com/inference-gateway/operator)
[![Tests](https://img.shields.io/badge/Tests-66%25%20Coverage-yellow.svg?style=flat-square)](https://github.com/inference-gateway/operator)
[![Lint](https://img.shields.io/badge/Lint-Passing-brightgreen.svg?style=flat-square)](https://golangci-lint.run/)

---

</div>

A Kubernetes operator for automating the deployment and management of Inference Gateway instances on Kubernetes.

## Description

This Kubernetes operator extends the Kubernetes API to create, configure and manage Inference Gateway instances within a Kubernetes cluster. It provides a comprehensive CRD (Custom Resource Definition) that allows you to declaratively manage:

- **Gateway Deployment**: Automated deployment with configurable replicas, resources, and Horizontal Pod Autoscaling (HPA)
- **AI Provider Integration**: Support for OpenAI, Anthropic, Ollama, and other AI/ML providers
- **Authentication & Authorization**: OIDC integration with configurable identity providers
- **Model Context Protocol (MCP)**: Integration with MCP servers for extended AI capabilities
- **Agent-to-Agent (A2A)**: Support for distributed agent communication and orchestration
- **GPU Runtimes**: Lease externally hosted, GPU-backed inference runtimes (RunPod today) via a pluggable provider interface, exposed as an HTTP endpoint and connection Secret ([`examples/gpu`](examples/gpu))
- **Observability**: Built-in metrics, tracing, and health monitoring
- **Network Configuration**: Service and Gateway API routing (Gateway + HTTPRoute) with listener-level TLS

The operator follows cloud-native best practices and provides a unified control plane for managing both the gateway infrastructure and its associated AI workloads.

## ✨ Key Features

<div align="center">

|  🚀 **Deployment**  |  🔐 **Security**  | 📊 **Observability** | 🔗 **Integration** |
| :-----------------: | :---------------: | :------------------: | :----------------: |
|    Auto-scaling     |     OIDC Auth     |  Prometheus Metrics  |    MCP Protocol    |
|   Rolling Updates   |      TLS/SSL      | Distributed Tracing  |     A2A Agents     |
|    Health Checks    | Secret Management |  Status Monitoring   | Multiple Providers |
| Resource Management | Network Policies  |       Logging        | Custom Extensions  |

</div>

**🤖 Supported AI Providers:**

- Anthropic • Cloudflare • Cohere • DeepSeek • ElevenLabs • Google • Groq • llama.cpp • MiniMax • Mistral • Moonshot • NVIDIA • Ollama • Ollama Cloud • OpenAI • Z.ai • plus any OpenAI-compatible endpoint via `custom`

**☸️ Kubernetes Native:**

- CRDs • Controller Pattern • RBAC • Service Mesh Ready

## 📚 Table of Contents

- [🚀 Quick Start](#-quick-start)
- [📦 Installation](#-installation)
- [✅ Verification](#-verification)
- [🏷️ Namespace Scoping](#️-namespace-scoping)
- [🚀 Deploy Your First Gateway](#-deploy-your-first-gateway)
- [🤖 Deploy an Orchestrator](#-deploy-an-orchestrator)
- [🔄 Upgrade](#-upgrade)
- [🗑️ Uninstallation](#️-uninstallation)
- [🏗️ Supported Architectures](#️-supported-architectures)
- [📋 API Overview](#-api-overview)
- [⚙️ Configuration Examples](#️-configuration-examples)
- [❓ Frequently Asked Questions](#-frequently-asked-questions)
- [🏗️ Development](#️-development)
- [📊 Monitoring & Management](#-monitoring--management)
- [🔧 Troubleshooting](#-troubleshooting)
- [📖 API Reference](#-api-reference)
- [🤝 Contributing](#-contributing)
- [📄 License](#-license)

## 📋 API Overview

The `Gateway` CRD supports the following key configuration areas:

### Core Configuration

- **Replicas**: Number of gateway instances (1-100)
- **Image**: Container image and version
- **Environment**: Deployment environment (development, staging, production)
- **Resources**: CPU and memory requests/limits

### Auto-scaling (HPA)

- **Horizontal Pod Autoscaler**: Automatic scaling based on CPU, memory, or custom metrics
- **Min/Max Replicas**: Configurable scaling boundaries
- **Multiple Metrics**: CPU utilization, memory utilization, custom metrics support
- **Stabilization Windows**: Fine-tuned scaling behavior control

### Observability (Telemetry)

Powered by **OpenTelemetry** for industry-standard observability:

- **Metrics**: Prometheus metrics endpoint with OpenTelemetry SDK
  - Request counts, durations, error rates
  - Token usage tracking (prompt, completion, total)
  - Provider-specific metrics by model
  - Custom histogram boundaries for latency analysis
- **Tracing**: Distributed tracing with OpenTelemetry exporters
  - OTLP trace export to Jaeger, Zipkin, or any OTLP-compatible backend
  - Request flow visualization across services
  - Performance bottleneck identification
- **Logs**: Structured logging with correlation IDs for distributed tracing

### Authentication

- **OIDC**: OpenID Connect integration with configurable issuers
- **Provider Support**: Multiple authentication providers (oidc, jwt, basic)

### AI Providers

Support for multiple AI/ML providers with flexible configuration. The list is
generated from the canonical [`inference-gateway/schemas`](https://github.com/inference-gateway/schemas)
OpenAPI enum into [`internal/providers/zz_generated_providers.go`](internal/providers/zz_generated_providers.go),
so it never drifts from the gateway:

| `providers[].name` | Provider |
| ------------------ | -------- |
| `anthropic` | Claude API |
| `cloudflare` | Cloudflare Workers AI |
| `cohere` | Command and embedding models |
| `deepseek` | Cost-effective reasoning models |
| `elevenlabs` | Speech and audio models |
| `google` | Google AI / Gemini |
| `groq` | Fast inference with open models |
| `llamacpp` | llama.cpp server |
| `minimax` | MiniMax models |
| `mistral` | Mistral AI |
| `moonshot` | Moonshot AI (Kimi) |
| `nvidia` | NVIDIA NIM |
| `ollama` | Local model serving |
| `ollama_cloud` | Ollama Cloud |
| `openai` | OpenAI API |
| `zai` | Z.ai |

In addition, `custom` points the gateway at any OpenAI-compatible endpoint via
the `CUSTOM_API_URL` and `CUSTOM_API_KEY` environment variables.

`providers[].name` has no CRD enum: it is validated at runtime and matched
**case-insensitively**, so `OpenAI`, `openai` and `OPENAI` are equivalent. A
name outside the list above is skipped - it is left out of
`status.providerSummary`.

### Extensions

- **MCP (Model Context Protocol)**: Integration with MCP servers for tool access
  - **Service Discovery**: Automatic discovery of `MCP` CRs via Kubernetes label selectors (Gateway and Orchestrator)
  - **Dynamic Updates**: The pod's `MCP_SERVERS` / mounted `mcp.yaml` is rebuilt and rolled when the discovered set changes
- **A2A (Agent-to-Agent)**: Distributed agent communication and polling, driven by the `Orchestrator`
  - **Service Discovery**: Automatic discovery of `Agent` CRs via Kubernetes label selectors (Orchestrator only)
  - **Dynamic Agent Registration**: Discovered agents are written to the orchestrator's mounted `agents.yaml` and the pod is rolled when the set changes
- **Health Checks**: Automated health monitoring for external services

### Networking

- **Service**: Kubernetes Service configuration (ClusterIP, NodePort, LoadBalancer)
- **Routing**: Kubernetes Gateway API (`gateway.networking.k8s.io/v1`) - operator-managed `Gateway` + `HTTPRoute`, or attach to a platform-team-managed shared `Gateway` via `parentRefs`
- **TLS**: Listener-level TLS termination via cert-manager (Gateway API integration)

## 🚀 Quick Start

### Prerequisites

- `kubectl` version v1.35.4+ with access to a Kubernetes cluster
- Kubernetes cluster v1.35.4+ (supports both arm64 and amd64 architectures)
- **Kubernetes Gateway API standard-channel CRDs** - required even if you never use Gateway API routing. The operator watches `gateway.networking.k8s.io/v1` `Gateway` and `HTTPRoute`, so without these CRDs the manager fails its cache sync and exits (`CrashLoopBackOff`):

  ```bash
  kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.5.1/standard-install.yaml
  ```

  `install.yaml` does not bundle them. v1.5.1 is the version this repo tests against (`GATEWAY_API_VERSION` in `Taskfile.yaml`); newer standard-channel releases work too.

## 📦 Installation

The Inference Gateway Operator supports multiple installation methods. Choose the one that best fits your deployment strategy:

### Method 1: One-Command Installation (Recommended)

Install the operator and CRDs in one command using the latest release:

```bash
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/install.yaml
```

This command will:

- Create the `inference-gateway-system` namespace
- Install all required Custom Resource Definitions (CRDs)
- Deploy the operator with proper RBAC permissions
- Set up monitoring and metrics collection

### Method 2: Specific Version Installation

For production environments, pin to a specific version:

```bash
# Install version v0.26.0 (replace with desired version)
kubectl apply -f https://github.com/inference-gateway/operator/releases/download/v0.26.0/install.yaml
```

### Method 3: GitOps/ArgoCD-Friendly Installation

For GitOps workflows, use stable manifest URLs:

```yaml
# ArgoCD Application example
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: inference-gateway-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/inference-gateway/operator
    targetRevision: v0.26.0
    path: manifests
  destination:
    server: https://kubernetes.default.svc
    namespace: inference-gateway-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

At a release tag, `manifests/install.yaml` references that release's operator image, so pinning `targetRevision` pins the operator version as well. On `main` the manifest tracks the most recent release.

> **Use v0.26.0 or later.** Tags before v0.26.0 ship `manifests/install.yaml` with
> `ghcr.io/inference-gateway/operator:latest`, so pinning `targetRevision` to one of them pins the
> CRDs but still deploys the current operator image. That image starts the Orchestrator and GPU
> controllers, whose CRDs those older manifests do not contain, and the manager exits on startup.

### Method 4: Separate CRD Installation (Advanced)

For scenarios where you need separate control over the CRD lifecycle (for example, upgrading CRDs independently of the operator):

```bash
# Step 1: Install (or upgrade) the CRDs on their own
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/crds.yaml

# Step 2: Install the operator (install.yaml is idempotent and keeps the CRDs in sync)
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/install.yaml
```

### Method 5: Custom Namespace Installation

By default, the operator deploys to the `inference-gateway-system` namespace. To deploy to a custom namespace:

#### Option A: Simple sed replacement

```bash
# Download and modify the install.yaml
curl -L https://github.com/inference-gateway/operator/releases/latest/download/install.yaml | \
  sed 's/inference-gateway-system/my-custom-namespace/g' | \
  kubectl apply -f -
```

#### Option B: Using the development workflow

```bash
# Clone the repository
git clone https://github.com/inference-gateway/operator.git
cd operator

# Generate manifests for your custom namespace
task manifests-for-namespace NAMESPACE=my-custom-namespace

# Deploy the generated manifests
kubectl apply -f manifests/my-custom-namespace/install.yaml
```

#### Option C: GitOps with custom namespace

`manifests/install.yaml` hardcodes `namespace: inference-gateway-system` on every namespaced object
(operator Deployment, ServiceAccount, leader-election Role and RoleBinding, metrics Service) and in
the `operator-manager-rolebinding` ClusterRoleBinding subjects. A kustomize patch that only renames
the `Namespace` object therefore leaves the operator in the original namespace. Generate the
manifests instead and point ArgoCD at your own repository:

```bash
git clone https://github.com/inference-gateway/operator.git
cd operator
git checkout v0.26.0

# Rewrites the namespace across the whole manifest
task manifests-for-namespace NAMESPACE=my-custom-namespace

# Commit manifests/my-custom-namespace/ to your GitOps repository
```

```yaml
# ArgoCD Application pointing at your generated manifests
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: inference-gateway-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/my-org/my-gitops-repo
    targetRevision: main
    path: manifests/my-custom-namespace
  destination:
    server: https://kubernetes.default.svc
    namespace: my-custom-namespace
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
```

### Method 6: Development Installation

For development and testing with the latest code:

```bash
# Clone the repository
git clone https://github.com/inference-gateway/operator.git
cd operator

# Install CRDs
task install

# Build and deploy operator
task deploy IMG=ghcr.io/inference-gateway/operator:latest
```

`task deploy` builds the image with Docker, imports it into the local k3d cluster named `dev`
(`k3d image import <IMG> -c dev`) and applies `config/environments/dev`. It needs Go 1.26.7+,
Docker, and the k3d `dev` cluster created by `task cluster:create` - it does not work against an
arbitrary `~/.kube/config` context.

## ✅ Verification

Verify the installation:

```bash
# Check if the operator is running
kubectl get pods -n inference-gateway-system

# Check if CRDs are installed
kubectl get crd | grep inference-gateway

# View operator logs
kubectl logs -n inference-gateway-system deployment/operator-inference-gateway -f
```

Expected output:

```bash
# Pods should show Running status
NAME                                          READY   STATUS    RESTARTS   AGE
operator-inference-gateway-74c9c5f5b-x4d2k    1/1     Running   0          2m

# CRDs should be listed
agents.core.inference-gateway.com          2025-06-21T17:30:00Z
gateways.core.inference-gateway.com        2025-06-21T17:30:00Z
gpus.core.inference-gateway.com            2025-06-21T17:30:00Z
mcps.core.inference-gateway.com            2025-06-21T17:30:00Z
orchestrators.core.inference-gateway.com   2025-06-21T17:30:00Z
```

## 🏷️ Namespace Scoping

The shipped operator Deployment sets `WATCH_NAMESPACE_SELECTOR=inference-gateway.com/managed=true`, so **`Gateway`, `Agent`, `MCP` and `GPU` resources are only reconciled in namespaces carrying that label**. In an unlabeled namespace the operator just logs `skipping gateway, namespace does not match WATCH_NAMESPACE_SELECTOR` and creates nothing.

Label every namespace you deploy resources into:

```bash
kubectl label namespace <namespace> inference-gateway.com/managed=true
```

The manifests under `examples/` already create their namespaces with this label. `Orchestrator` resources are not namespace-filtered.

To change the scope, edit `WATCH_NAMESPACE_SELECTOR` on the operator Deployment - any valid label selector works, and an empty value (or removing the variable) makes the operator watch **all** namespaces:

```bash
# Watch all namespaces
kubectl set env -n inference-gateway-system deployment/operator-inference-gateway WATCH_NAMESPACE_SELECTOR=
```

All `Gateway`, `Agent`, `MCP` and `GPU` examples below assume their namespace is labeled accordingly.

## 🚀 Deploy Your First Gateway

Create a simple gateway to test the installation:

```bash
# Label the target namespace so the operator reconciles it
kubectl label namespace default inference-gateway.com/managed=true

# Create a minimal gateway
cat <<EOF | kubectl apply -f -
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: my-first-gateway
  namespace: default
spec:
  replicas: 1
  environment: development
  telemetry:
    enabled: true
    metrics:
      enabled: true
      port: 9464
  providers:
    - name: OpenAI
      enabled: true
      env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: openai-secret
              key: api_key
EOF
```

**Note:** You'll need to create the `openai-secret` with your API key:

```bash
kubectl create secret generic openai-secret \
  --from-literal=api_key=your-openai-api-key-here
```

## 🤖 Deploy an Orchestrator

The `Orchestrator` CRD deploys the Inference Gateway CLI's `channels-manager`
daemon: an LLM-driven loop that reads messages from a chat channel, optionally
fans out to A2A `Agent`s and tools (incl. MCP), and replies. Telegram is the
channel currently supported; more channels are planned.

When the Telegram channel is enabled, the Deployment is forced to a singleton
(`replicas: 1`, `strategy: Recreate`) because Telegram allows only one active
`getUpdates` consumer per bot token - running two replicas would cause them
to terminate each other with `409 Conflict`. For high-availability today, run
multiple `Orchestrator` resources with different tokens and disjoint
`allowedUsers` (manual sharding); webhook mode and shared state would be
required for true scale-out and are out of scope.

```bash
# 1. Create the bot credentials secret
kubectl create namespace orchestrators
kubectl create secret generic telegram-bot-credentials -n orchestrators \
  --from-literal=token='<TELEGRAM_BOT_TOKEN>' \
  --from-literal=allowedUsers='111111111,222222222'

# 2. Apply the Orchestrator resource
cat <<EOF | kubectl apply -f -
apiVersion: core.inference-gateway.com/v1alpha1
kind: Orchestrator
metadata:
  name: orchestrator-controlled-by-telegram
  namespace: orchestrators
spec:
  image: ghcr.io/inference-gateway/cli:latest
  channels:
    telegram:
      enabled: true
      tokenSecretRef:
        name: telegram-bot-credentials
        key: token
      allowedUsersSecretRef:
        name: telegram-bot-credentials
        key: allowedUsers
      pollTimeout: 30s
  gateway:
    url: http://inference-gateway.inference-gateway.svc.cluster.local:8080
  agent:
    model: deepseek/deepseek-v4-pro
    systemPrompt: "You are a helpful assistant running as a Telegram bot."
EOF

# 3. Watch logs (requires CLI built with INFER_LOGGING_STDOUT support)
kubectl logs -n orchestrators deploy/orchestrator-controlled-by-telegram -f
```

A full end-to-end example - Gateway, two A2A worker Agents, and the Orchestrator
- with a step-by-step walkthrough is available at
[`examples/orchestrator/`](examples/orchestrator/).

## 🔄 Upgrade

To upgrade the operator to a newer version:

```bash
# Upgrade to latest version
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/install.yaml

# Or upgrade to specific version (v0.26.0 or later)
kubectl apply -f https://github.com/inference-gateway/operator/releases/download/v0.26.0/install.yaml
```

The operator Deployment uses the `Recreate` strategy, so an upgrade terminates the running operator
pod before starting the new one. Existing Gateway, Agent, MCP, Orchestrator and GPU workloads keep
serving during that gap; only reconciliation pauses until the new pod is ready.

## 🗑️ Uninstallation

To completely remove the operator:

```bash
# Delete all Gateway instances first
kubectl delete gateway --all --all-namespaces

# Uninstall the operator
kubectl delete -f https://github.com/inference-gateway/operator/releases/latest/download/install.yaml
```

## 🏗️ Supported Architectures

The operator supports multi-architecture deployments:

- **linux/amd64** - Intel/AMD 64-bit processors
- **linux/arm64** - ARM 64-bit processors (Apple Silicon, AWS Graviton, etc.)

Container images are automatically selected based on your cluster's node architecture.

### ⚙️ Example Configurations

> Every `Gateway`, `Agent`, `MCP` and `GPU` example below only reconciles if its namespace is labeled `inference-gateway.com/managed=true` - see [Namespace Scoping](#️-namespace-scoping).

#### Minimal Gateway

```yaml
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: simple-gateway
  namespace: default
spec:
  replicas: 1
  environment: development
  telemetry:
    enabled: true
    metrics:
      enabled: true
      port: 9464
  providers:
    - name: OpenAI
      enabled: true
      env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: openai-secret
              key: api_key
```

#### Production Gateway with Authentication

```yaml
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: production-gateway
  namespace: inference-gateway
spec:
  replicas: 3
  image: "ghcr.io/inference-gateway/inference-gateway:0.23.6"
  environment: production

  auth:
    enabled: true
    provider: oidc
    oidc:
      issuerUrl: "https://auth.company.com/realms/ai"
      clientId: "inference-gateway"
      audiences:
        - "api://inference-gateway"
      clientSecretRef:
        name: auth-secrets
        key: client-secret

  providers:
    - name: OpenAI
      enabled: true
      env:
        - name: OPENAI_API_KEY
          valueFrom:
            secretKeyRef:
              name: ai-secrets
              key: openai-key

  resources:
    requests:
      cpu: "500m"
      memory: "512Mi"
    limits:
      cpu: "2000m"
      memory: "2Gi"

  gatewayAPI:
    enabled: true
    gateway:
      gatewayClassName: envoy
      tls:
        enabled: true
        issuer: letsencrypt-prod
        secretName: ai-gateway-tls
    httpRoute:
      hostnames:
        - "ai-gateway.company.com"
```

#### A2A Service Discovery Configuration

A2A agent discovery is an **`Orchestrator`** feature — the `Gateway` does not
discover or run A2A agents. The `Orchestrator` discovers `Agent` CRs by label
selector and writes them into `~/.infer/agents.yaml` inside its pod:

```yaml
apiVersion: core.inference-gateway.com/v1alpha1
kind: Orchestrator
metadata:
  name: orchestrator-with-service-discovery
  namespace: default
spec:
  a2a:
    enabled: true
    # Static agent URLs are kept alongside discovered ones.
    agents:
      - "http://static-agent.agents.svc.cluster.local:8080"
    # Automatic discovery of Agent CRs by label selector.
    serviceDiscovery:
      enabled: true
      namespace: "agents" # Namespace to search (defaults to the Orchestrator's own namespace)
      selector:
        matchLabels:
          agent-group: group1
```

**Service Discovery Features:**

- **Automatic Agent Discovery**: Discovers `Agent` CRs based on their Kubernetes labels
- **Dynamic Configuration**: Discovered agents are written to the orchestrator's mounted `agents.yaml`; the pod is rolled when the set changes
- **Label-Based Selection**: Uses a configurable label selector to identify `Agent` CRs
- **Namespace Scoping**: Can search for `Agent` CRs in a specific namespace (defaults to the Orchestrator's namespace)

**Agent Requirements:**

For an `Agent` CR to be discovered, it must carry labels matching the selector:

```yaml
apiVersion: core.inference-gateway.com/v1alpha1
kind: Agent
metadata:
  name: my-agent
  namespace: agents
  labels:
    agent-group: group1 # matches spec.a2a.serviceDiscovery.selector
spec:
  # Agent configuration
```

#### MCP OAuth Protected Resource Metadata (RFC 9728)

When `spec.mcp.expose` is on, the gateway serves OAuth 2.0 Protected Resource
Metadata at `GET /.well-known/oauth-protected-resource/mcp`, so an MCP client
with no token discovers the IdP from the `401` on `/mcp` (required by MCP
`2026-07-28`). The operator-managed `HTTPRoute` matches the `/` path prefix, so
that document is routed to the gateway Service wherever `/mcp` is - no extra
route needed.

`spec.mcp.resourceUrl` pins the canonical public `/mcp` URL the gateway puts in
the document's `resource` and in the challenge's `resource_metadata`. It is
emitted as `MCP_RESOURCE_URL`:

```yaml
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: gateway-with-mcp-auth
  namespace: inference-gateway
spec:
  mcp:
    enabled: true
    expose: true
    resourceUrl: "https://api.example.com/mcp"
  gatewayAPI:
    enabled: true
    gateway:
      tls:
        enabled: true
    httpRoute:
      hostnames:
        - api.example.com
```

When `resourceUrl` is omitted the operator defaults it to
`<scheme>://<first httpRoute hostname>/mcp` (`https` when
`gatewayAPI.gateway.tls.enabled`), so the example above works without the
explicit value. Nothing is emitted when routing is disabled or the hostname is
a wildcard, and the gateway then derives the URL from the inbound request
scheme (honouring `X-Forwarded-Proto`) and `Host` - set `resourceUrl`
explicitly whenever the ingress rewrites either, or clients are handed a URL
they cannot reach.

#### MCP Service Discovery Configuration

`MCP` Custom Resources can be discovered automatically by both `Gateway` and
`Orchestrator` via the same label-selector pattern used for A2A agents. Opt-in
on the MCP side is via plain `metadata.labels` - no extra field on `MCPSpec`.

**Gateway-side discovery** - discovered MCPs are unioned with the static
`spec.mcp.servers[]` list, deduped on URL, sorted, and exposed via the gateway
pod's `MCP_SERVERS` env var as `name=url` entries. The name (`spec.mcp.servers[].name`
for static servers, `metadata.name` for discovered `MCP` CRs) becomes the tool
namespace, so tools reach the model as `mcp_<name>_<tool>` - for example
`mcp_time_get_current_time` rather than a long host-derived alias. To be used as an
alias a name must match `^[a-z0-9_-]+$`, be unique across static and discovered
servers, and must not be the reserved alias `tools`; a name that fails these rules is
rendered as a bare URL and the gateway falls back to deriving the alias from the host.

```yaml
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: gateway-with-mcp-discovery
  namespace: inference-gateway
spec:
  mcp:
    enabled: true
    # Static entries still work alongside discovery.
    servers:
      - name: external-mcp
        url: "https://mcp.example.com/mcp"
    serviceDiscovery:
      enabled: true
      namespace: mcp # defaults to the Gateway's own namespace
      selector:
        matchLabels:
          mcp-group: group1
```

**Orchestrator-side discovery** - discovered MCPs are rendered into an
`mcp.yaml` ConfigMap mounted at `~/.infer/mcp.yaml` inside the orchestrator
pod. A content-hash pod annotation rolls the singleton when the set changes:

```yaml
apiVersion: core.inference-gateway.com/v1alpha1
kind: Orchestrator
metadata:
  name: orchestrator
  namespace: orchestrator
spec:
  mcp:
    enabled: true
    servers: [] # static URLs, optional
    serviceDiscovery:
      enabled: true
      namespace: mcp
      selector:
        matchLabels:
          mcp-group: group1
```

**MCP CR labeling** - tag each `MCP` you want discovered with a matching label:

```yaml
apiVersion: core.inference-gateway.com/v1alpha1
kind: MCP
metadata:
  name: my-mcp
  namespace: mcp
  labels:
    mcp-group: group1 # picked up by both selectors above
spec:
  image: ghcr.io/your-org/your-mcp-server:latest
  server:
    port: 8080
    # Endpoint path the operator appends when building the in-cluster URL.
    # Defaults to "/mcp"; override if your server listens elsewhere.
    path: "/mcp"
```

Visibility:

```bash
kubectl get gateway       # MCPS column shows static + discovered MCP servers
kubectl get orchestrator  # MCPS column shows the discovered count
```

For a runnable end-to-end demo (Gateway + Orchestrator + two discovered MCP
servers built with `metoro-io/mcp-golang`), see
[`examples/orchestrator/`](examples/orchestrator/).

#### Complete Configuration

See [`examples/gateway-complete/gateway.yaml`](examples/gateway-complete/gateway.yaml) for a comprehensive configuration example with all features enabled.

### 🚀 Advanced Configuration

For production deployments, use the complete configuration examples:

```bash
# Deploy production-ready gateway with authentication
kubectl apply -f https://raw.githubusercontent.com/inference-gateway/operator/main/examples/gateway-complete/gateway.yaml

# Deploy minimal gateway for development
kubectl apply -f https://raw.githubusercontent.com/inference-gateway/operator/main/examples/gateway-minimal/gateway.yaml
```

For an A2A service discovery example — `Agent` CRs discovered by the
`Orchestrator`, not the `Gateway` — see
[`examples/orchestrator/`](examples/orchestrator/).

### ✅ Configuration Validation

The operator includes comprehensive validation:

- **Replica limits**: 1-100 replicas
- **Port ranges**: Valid port numbers (1024-65535 for server ports)
- **Environment values**: Restricted to development, staging, production
- **Resource limits**: Proper CPU/memory specifications

### 📊 Monitoring and Status

Check Gateway status and health:

```bash
# Check gateway resources
kubectl get gateways -A

# Get detailed gateway status
kubectl describe gateway my-first-gateway

# Check generated resources (everything the operator creates for a Gateway is
# labeled app=<gateway-name>)
kubectl get deployments,services,configmaps -l app=my-first-gateway

# View Gateway logs
kubectl logs -l app=my-first-gateway -f
```

`status` reports:

- `url`: the address the gateway is reachable at (HTTPRoute hostname when Gateway
  API routing is enabled, otherwise the in-cluster service FQDN)
- `providerSummary`: comma-separated list of configured providers
- `serviceAccountName`: the ServiceAccount the gateway pods run as
- `mcpServers`: sorted `<name>=<url>` entries the pod is configured with
- `mcpServerCount`: number of static plus discovered MCP servers

For replica counts and rollout health, check the Deployment directly
(`kubectl get deployment my-first-gateway`) - the Gateway status does not
mirror them.

## ❓ Frequently Asked Questions

### Do I need to install CRDs separately?

**It depends on your installation method:**

- **One-command installation (Recommended)**: No! CRDs are included automatically:

  ```bash
  kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/install.yaml
  ```

- **GitOps/ArgoCD installations**: No! Use the `manifests/` directory which includes CRDs:

  ```yaml
  source:
    repoURL: https://github.com/inference-gateway/operator
    path: manifests # Includes both CRDs and operator
  ```

- **Custom namespace installations**: No! When you substitute the namespace in `install.yaml`, the CRDs are still included:

  ```bash
  curl -L https://github.com/inference-gateway/operator/releases/latest/download/install.yaml | \
    sed 's/inference-gateway-system/my-namespace/g' | kubectl apply -f -
  ```

- **Advanced scenarios**: You can install CRDs separately for more control:
  ```bash
  kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/crds.yaml
  ```

### What architectures are supported?

The operator supports both **arm64** and **amd64** architectures:

- Container images are built for both platforms
- Kubernetes automatically selects the correct image for your nodes
- Works on Apple Silicon (M1/M2), AWS Graviton, Intel/AMD processors

### How do I check if the installation was successful?

Run these commands to verify your installation:

```bash
# 1. Check operator pods
kubectl get pods -n inference-gateway-system

# 2. Verify CRDs are installed
kubectl get crd | grep inference-gateway

# 3. Test creating a Gateway resource
kubectl get gateways --all-namespaces
```

### Can I install in a different namespace?

**Yes!** The operator defaults to `inference-gateway-system` but can be deployed to any namespace:

**Quick Method:**

```bash
curl -L https://github.com/inference-gateway/operator/releases/latest/download/install.yaml | \
  sed 's/inference-gateway-system/my-namespace/g' | \
  kubectl apply -f -
```

**Development Method:**

```bash
# Generate manifests for custom namespace
task manifests-for-namespace NAMESPACE=my-namespace
kubectl apply -f manifests/my-namespace/install.yaml
```

**GitOps Method:** Use Kustomize patches in your ArgoCD Application or Flux Kustomization.

### How do I upgrade the operator?

Simply reapply the installation with a newer version:

```bash
# Upgrade to latest
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/install.yaml

# Or upgrade to specific version (v0.26.0 or later)
kubectl apply -f https://github.com/inference-gateway/operator/releases/download/v0.26.0/install.yaml
```

The operator Deployment uses the `Recreate` strategy, so the old operator pod is terminated before
the new one starts. Running Gateway instances are unaffected; reconciliation pauses briefly.

### What happens to my Gateways if I delete the operator?

Your Gateway resources will remain in the cluster but will no longer be managed. To completely clean up:

```bash
# 1. Delete all Gateway instances first
kubectl delete gateway --all --all-namespaces

# 2. Then uninstall the operator
kubectl delete -f https://github.com/inference-gateway/operator/releases/latest/download/install.yaml
```

## 🏗️ Development

### Prerequisites for Development

The recommended way to get a complete toolchain is via the project's [Flox](https://flox.dev) environment - `flox activate` provides Go, `task`, `kubectl`, `kubebuilder`, `kustomize`, `golangci-lint`, `k3d`, `ctlptl`, `gh`, and `prettier` at the pinned versions used by CI. See [CONTRIBUTING.md](CONTRIBUTING.md#setting-up-your-environment) for alternatives (flox environment or manual install).

If installing manually:

- Task runner (`task`)
- Go 1.26+
- Docker
- k3d (via `ctlptl`) or similar local Kubernetes cluster

### Development Workflow

```sh
# Run tests
task test

# Run linting
task lint

# Generate code and manifests (including install.yaml)
task generate manifests

# Build locally
task build

# Run against local cluster
task run
```

**Note:** The `task manifests` command automatically generates:

- CRDs in `config/crd/bases/`
- Installation manifests in `manifests/install.yaml`
- CRD-only manifests in `manifests/crds.yaml`

These files are version-controlled and included in releases.

### Testing

The operator includes comprehensive unit and integration tests:

```sh
# Run all tests
task test

# Run e2e tests (requires running cluster)
task test:e2e
```

## 📊 Monitoring & Management

### 🔍 OpenTelemetry Observability

The Inference Gateway provides enterprise-grade observability through OpenTelemetry:

**Metrics Collection:**

```bash
# Access Prometheus metrics. The metrics port is exposed on the Service named
# after the Gateway, and only when both telemetry.enabled and
# telemetry.metrics.enabled are true.
curl http://my-gateway:9464/metrics

# Key metrics include:
# - llm_requests_total: Request counts by provider/model
# - llm_tokens_*: Token usage tracking
# - llm_request_duration_seconds: Request latency histograms
# - llm_latency_*: Detailed timing breakdowns
```

**Distributed Tracing:**

```yaml
# Configure OTLP trace export - OTLP is the only supported exporter
telemetry:
  enabled: true
  traces:
    exporter:
      otlp:
        endpoint: "http://otel-collector:4318"
        protocol: "http/protobuf" # or "grpc" (e.g. http://otel-collector:4317)
```

`TELEMETRY_TRACING_ENABLED` and `TELEMETRY_TRACING_OTLP_ENDPOINT` are only set
on the gateway container when `telemetry.enabled` is `true` **and**
`telemetry.traces.exporter.otlp` is present. Non-OTLP collector endpoints (for
example Jaeger's `:14268/api/traces`) are not supported - point the exporter at
an OTLP receiver instead.

**Supported Backends:**

- **Metrics**: Prometheus, Grafana, any OpenTelemetry-compatible backend
- **Tracing**: Jaeger, Zipkin, Lightstep, Honeycomb, Datadog (via OTLP)
- **Logs**: Structured JSON with trace correlation for any log aggregation system

### 📈 Monitoring Gateway Health

```sh
# Check gateway status
kubectl get gateway my-gateway -o yaml

# Check deployment health
kubectl get deployment my-gateway

# Check service endpoints
kubectl get service my-gateway

# View the inline model-routing config (only created when spec.routing.config is set)
kubectl get configmap my-gateway-routing -o yaml
```

### 🔧 Troubleshooting

Common issues and solutions:

1. **Nothing happens after applying a Gateway/Agent/MCP/GPU**: the namespace is missing the `inference-gateway.com/managed=true` label - see [Namespace Scoping](#️-namespace-scoping)
2. **Operator pod in CrashLoopBackOff**: the Kubernetes Gateway API standard CRDs are missing - see [Prerequisites](#prerequisites)
3. **Gateway not starting**: Check image pull policy and secrets
4. **Authentication failures**: Verify OIDC configuration and secrets
5. **Provider connection issues**: Check network policies and secret references
6. **Resource constraints**: Review resource requests/limits

### Upgrade Process

The operator supports rolling upgrades:

1. Update the Gateway spec with new image version
2. Operator automatically performs rolling update
3. Monitor status for completion

### Cleanup

**Delete gateway instances:**

```sh
kubectl delete gateway --all
```

**Uninstall operator:**

```sh
task undeploy
task uninstall
```

## 📖 API Reference

For complete API reference, see the generated CRD documentation or use:

```sh
kubectl explain gateway.spec
kubectl explain gateway.spec.providers
kubectl explain gateway.spec.auth
# etc.
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes with tests
4. Run `task lint test`
5. Submit a pull request

## 📄 License

This project is licensed under the Apache 2.0 License - see the LICENSE file for details.
