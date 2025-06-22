<div align="center">

# Inference Gateway Operator

> **⚠️ EARLY STAGE PROJECT WARNING**  
> This project is currently in its early development stages. Breaking changes are expected and the API may change significantly between releases. Use with caution in production environments and expect potential migration requirements when upgrading versions.

**A Kubernetes operator for automating the deployment and management of Inference Gateway instances**

[![Go Version](https://img.shields.io/github/go-mod/go-version/inference-gateway/operator?style=flat-square)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](https://opensource.org/licenses/MIT)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-v1.32+-blue.svg?style=flat-square&logo=kubernetes)](https://kubernetes.io/)
[![Docker](https://img.shields.io/badge/Docker-Available-blue.svg?style=flat-square&logo=docker)](https://hub.docker.com/)
[![OpenAPI](https://img.shields.io/badge/OpenAPI-3.0-green.svg?style=flat-square)](https://swagger.io/specification/)

[![Latest Release](https://img.shields.io/github/v/release/inference-gateway/operator?style=flat-square&logo=github)](https://github.com/inference-gateway/operator/releases/latest)
[![Container Registry](https://img.shields.io/badge/Container-ghcr.io-blue.svg?style=flat-square&logo=github)](https://github.com/inference-gateway/operator/pkgs/container/operator)
[![Multi-Arch](https://img.shields.io/badge/Architecture-amd64%20%7C%20arm64-green?style=flat-square)](https://github.com/inference-gateway/operator/releases)
[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg?style=flat-square)](https://github.com/inference-gateway/operator)
[![Tests](https://img.shields.io/badge/Tests-69.4%25%20Coverage-yellow.svg?style=flat-square)](https://github.com/inference-gateway/operator)
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
- **Observability**: Built-in metrics, tracing, and health monitoring
- **Network Configuration**: Service and Ingress management with TLS support

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

- OpenAI • Anthropic • Ollama • Groq • Cohere • Cloudflare • DeepSeek

**☸️ Kubernetes Native:**

- CRDs • Controller Pattern • RBAC • Service Mesh Ready

## 📚 Table of Contents

- [🚀 Quick Start](#-quick-start)
- [📦 Installation](#-installation)
- [✅ Verification](#-verification)
- [🚀 Deploy Your First Gateway](#-deploy-your-first-gateway)
- [🔄 Upgrade](#-upgrade)
- [🗑️ Uninstallation](#️-uninstallation)
- [🏗️ Supported Architectures](#️-supported-architectures)
- [�📋 API Overview](#-api-overview)
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

Support for multiple AI/ML providers with flexible configuration:

- **OpenAI**: Integration with OpenAI API
- **Anthropic**: Claude API integration
- **Ollama**: Local model serving
- **Groq**: Fast inference with open models
- **Cohere**: Command and embedding models
- **Cloudflare**: Cloudflare Workers AI models
- **DeepSeek**: Cost-effective reasoning models
- **Custom Providers**: Extensible provider configuration

### Extensions

- **MCP (Model Context Protocol)**: Integration with MCP servers for tool access
- **A2A (Agent-to-Agent)**: Distributed agent communication and polling
- **Health Checks**: Automated health monitoring for external services

### Networking

- **Service**: Kubernetes Service configuration (ClusterIP, NodePort, LoadBalancer)
- **Ingress**: HTTP(S) ingress with TLS support and custom annotations
- **TLS**: Certificate management integration

## 🚀 Quick Start

### Prerequisites

- `kubectl` version v1.11.3+ with access to a Kubernetes cluster
- Kubernetes cluster v1.11.3+ (supports both arm64 and amd64 architectures)

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
# Install version v0.2.1 (replace with desired version)
kubectl apply -f https://github.com/inference-gateway/operator/releases/download/v0.2.1/install.yaml
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
    targetRevision: v0.2.1
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

### Method 4: Separate CRD Installation (Advanced)

For scenarios where you need separate control over CRDs:

```bash
# Step 1: Install CRDs only
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/crds.yaml

# Step 2: Install the operator (without CRDs)
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/namespace-install.yaml
```

### Method 5: Namespace-Scoped Installation

For multi-tenant environments where you don't want cluster-wide permissions:

```bash
# Install CRDs first (requires cluster-admin)
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/crds.yaml

# Install operator in specific namespace (namespace-scoped permissions)
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/namespace-install.yaml -n my-namespace
```

### Method 6: Custom Namespace Installation

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

```yaml
# ArgoCD Application with custom namespace
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: inference-gateway-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/inference-gateway/operator
    targetRevision: v0.2.1
    path: manifests
    kustomize:
      patches:
        - target:
            kind: Namespace
          patch: |
            - op: replace
              path: /metadata/name
              value: my-custom-namespace
  destination:
    server: https://kubernetes.default.svc
    namespace: my-custom-namespace
```

### Method 7: Development Installation

For development and testing with the latest code:

```bash
# Clone the repository
git clone https://github.com/inference-gateway/operator.git
cd operator

# Install CRDs
task install

# Build and deploy operator (requires Go 1.24+)
task deploy IMG=ghcr.io/inference-gateway/operator:latest
```

## ✅ Verification

Verify the installation:

```bash
# Check if the operator is running
kubectl get pods -n inference-gateway-system

# Check if CRDs are installed
kubectl get crd | grep inference-gateway

# View operator logs
kubectl logs -n inference-gateway-system deployment/operator-controller-manager -f
```

Expected output:

```bash
# Pods should show Running status
NAME                                         READY   STATUS    RESTARTS   AGE
operator-controller-manager-74c9c5f5b-x4d2k   2/2     Running   0          2m

# CRDs should be listed
gateways.core.inference-gateway.com   2025-06-21T17:30:00Z
```

## 🚀 Deploy Your First Gateway

Create a simple gateway to test the installation:

```bash
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
    - name: openai
      type: openai
      config:
        baseUrl: "https://api.openai.com/v1"
        authType: bearer
        tokenRef:
          name: openai-secret
          key: api-key
EOF
```

**Note:** You'll need to create the `openai-secret` with your API key:

```bash
kubectl create secret generic openai-secret \
  --from-literal=api-key=your-openai-api-key-here
```

## 🔄 Upgrade

To upgrade the operator to a newer version:

```bash
# Upgrade to latest version
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/install.yaml

# Or upgrade to specific version
kubectl apply -f https://github.com/inference-gateway/operator/releases/download/v0.2.1/install.yaml
```

The operator supports rolling upgrades and will not affect running Gateway instances.

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
    - name: openai
      type: openai
      config:
        baseUrl: "https://api.openai.com/v1"
        authType: bearer
        tokenRef:
          name: openai-secret
          key: api-key
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
  image: "ghcr.io/inference-gateway/inference-gateway:0.12.0"
  environment: production

  auth:
    enabled: true
    provider: oidc
    oidc:
      issuerUrl: "https://auth.company.com/realms/ai"
      clientId: "inference-gateway"
      clientSecretRef:
        name: auth-secrets
        key: client-secret

  providers:
    - name: openai
      type: openai
      config:
        baseUrl: "https://api.openai.com/v1"
        authType: bearer
        tokenRef:
          name: ai-secrets
          key: openai-key

  resources:
    requests:
      cpu: "500m"
      memory: "512Mi"
    limits:
      cpu: "2000m"
      memory: "2Gi"

  ingress:
    enabled: true
    className: "nginx"
    hosts:
      - host: "ai-gateway.company.com"
        paths:
          - path: "/"
            pathType: Prefix
    tls:
      - secretName: ai-gateway-tls
        hosts:
          - "ai-gateway.company.com"
```

#### Complete Configuration

See `examples/gateway-complete.yaml` for a comprehensive configuration example with all features enabled.

### 🚀 Advanced Configuration

For production deployments, use the complete configuration examples:

```bash
# Deploy production-ready gateway with authentication
kubectl apply -f https://raw.githubusercontent.com/inference-gateway/operator/main/examples/gateway-complete.yaml

# Deploy minimal gateway for development
kubectl apply -f https://raw.githubusercontent.com/inference-gateway/operator/main/examples/gateway-minimal.yaml
```

### ✅ Configuration Validation

The operator includes comprehensive validation:

- **Replica limits**: 1-100 replicas
- **Port ranges**: Valid port numbers (1024-65535 for server ports)
- **Environment values**: Restricted to development, staging, production
- **Provider types**: Validated against supported provider list
- **Resource limits**: Proper CPU/memory specifications

### 📊 Monitoring and Status

Check Gateway status and health:

```bash
# Check gateway resources
kubectl get gateways -A

# Get detailed gateway status
kubectl describe gateway my-first-gateway

# Check generated resources
kubectl get deployments,services,configmaps -l app.kubernetes.io/managed-by=inference-gateway-operator

# View Gateway logs
kubectl logs -l app.kubernetes.io/name=my-first-gateway -f
```

Status includes:

- Ready and available replica counts
- Deployment conditions and health
- Current phase (Pending, Running, Failed, Unknown)
- Detailed error messages

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

- **Namespace-scoped installations**: Yes! Install CRDs first, then the operator:

  ```bash
  kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/crds.yaml
  kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/namespace-install.yaml -n my-namespace
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

# Or upgrade to specific version
kubectl apply -f https://github.com/inference-gateway/operator/releases/download/v0.2.1/install.yaml
```

The operator supports rolling upgrades without affecting running Gateway instances.

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

- Task runner (`task`)
- Go 1.24+
- Docker
- Kind or similar local Kubernetes cluster

### Development Workflow

```sh
# Install dependencies
task install-tools

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
- Installation manifests in `dist/install.yaml`
- CRD-only manifests in `dist/crds.yaml`

These files are version-controlled and included in releases.

### Testing

The operator includes comprehensive unit and integration tests:

```sh
# Run all tests
task test

# Run with coverage
task test-coverage

# Run e2e tests (requires running cluster)
task test-e2e
```

## 📊 Monitoring & Management

### 🔍 OpenTelemetry Observability

The Inference Gateway provides enterprise-grade observability through OpenTelemetry:

**Metrics Collection:**

```bash
# Access Prometheus metrics
curl http://gateway-service:9464/metrics

# Key metrics include:
# - llm_requests_total: Request counts by provider/model
# - llm_tokens_*: Token usage tracking
# - llm_request_duration_seconds: Request latency histograms
# - llm_latency_*: Detailed timing breakdowns
```

**Distributed Tracing:**

```yaml
# Configure OTLP trace export
telemetry:
  tracing:
    enabled: true
    endpoint: "http://jaeger-collector:14268/api/traces"
    # Or use OTLP gRPC: "http://otel-collector:4317"
```

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

# View configuration
kubectl get configmap my-gateway-config -o yaml
```

### 🔧 Troubleshooting

Common issues and solutions:

1. **Gateway not starting**: Check image pull policy and secrets
2. **Authentication failures**: Verify OIDC configuration and secrets
3. **Provider connection issues**: Check network policies and secret references
4. **Resource constraints**: Review resource requests/limits

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

This project is licensed under the MIT License - see the LICENSE file for details.
