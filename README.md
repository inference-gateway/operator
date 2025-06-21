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

[![Build Status](https://img.shields.io/badge/Build-Passing-brightgreen.svg?style=flat-square)](https://github.com/inference-gateway/operator)
[![Tests](https://img.shields.io/badge/Tests-61.2%25%20Coverage-yellow.svg?style=flat-square)](https://github.com/inference-gateway/operator)
[![Lint](https://img.shields.io/badge/Lint-Passing-brightgreen.svg?style=flat-square)](https://golangci-lint.run/)

---

</div>

A Kubernetes operator for automating the deployment and management of Inference Gateway instances on Kubernetes.

## Description

This Kubernetes operator extends the Kubernetes API to create, configure and manage Inference Gateway instances within a Kubernetes cluster. It provides a comprehensive CRD (Custom Resource Definition) that allows you to declaratively manage:

- **Gateway Deployment**: Automated deployment with configurable replicas, resources, and scaling
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
- [📋 API Overview](#-api-overview)
- [⚙️ Configuration Examples](#️-configuration-examples)
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

- go version v1.23.0+
- docker version 17.03+
- kubectl version v1.11.3+
- Access to a Kubernetes v1.11.3+ cluster

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

### 🚀 Deployment

> **📦 Quick Deploy:** Get started in minutes with pre-built images and Helm charts.

**Build and push your image:**

```sh
task docker-build docker-push IMG=<registry>/inference-gateway-operator:tag
```

**Install the CRDs:**

```sh
task install
```

**Deploy the operator:**

```sh
task deploy IMG=<registry>/inference-gateway-operator:tag
```

**Create Gateway instances:**

```sh
# Deploy minimal gateway
kubectl apply -f examples/gateway-minimal.yaml

# Deploy complete gateway
kubectl apply -f examples/gateway-complete.yaml
```

### ✅ Verification

Check the operator and gateway status:

```sh
# Check operator deployment
kubectl get deployment -n inference-gateway-operator-system

# Check gateway resources
kubectl get gateways -A

# Check generated resources
kubectl get deployments,services,configmaps -l app.kubernetes.io/managed-by=inference-gateway-operator
```

### 🔍 Configuration Validation

The operator includes comprehensive validation:

- **Replica limits**: 1-100 replicas
- **Port ranges**: Valid port numbers (1024-65535 for server ports)
- **Environment values**: Restricted to development, staging, production
- **Provider types**: Validated against supported provider list
- **Resource limits**: Proper CPU/memory specifications

### Status and Observability

The operator provides detailed status information:

```sh
kubectl describe gateway my-gateway
```

Status includes:

- Ready and available replica counts
- Deployment conditions and health
- Current phase (Pending, Running, Failed, Unknown)
- Detailed error messages

## 🏗️ Development

### Prerequisites for Development

- Task runner (`task`)
- Go 1.23+
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

# Generate code and manifests
task generate manifests

# Build locally
task build

# Run against local cluster
task run
```

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
