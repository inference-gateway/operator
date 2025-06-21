# Installation Manifests

This directory contains installation manifests for the Inference Gateway Operator, organized for different deployment scenarios.

## Available Manifests

### 📦 `install.yaml` (Recommended)

Complete installation including CRDs and operator deployment.

```bash
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/install.yaml
```

**Includes:**

- Namespace (`inference-gateway-system`)
- Custom Resource Definitions (CRDs)
- Operator deployment with cluster-wide RBAC
- Service accounts and role bindings
- Monitoring configuration

**Use when:**

- Setting up the operator for the first time
- You need cluster-wide Gateway management
- You want the simplest installation experience

---

### 🔧 `crds.yaml`

Only the Custom Resource Definitions (CRDs).

```bash
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/crds.yaml
```

**Includes:**

- Gateway CRD definition
- Validation schemas
- Conversion webhooks (if any)

**Use when:**

- Installing CRDs separately from the operator
- Upgrading CRDs independently
- Setting up multi-tenant environments
- Preparing for namespace-scoped installations

---

### 🏢 `namespace-install.yaml`

Operator deployment without CRDs (namespace-scoped permissions).

```bash
# Install CRDs first
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/crds.yaml

# Install operator in specific namespace
kubectl apply -f https://github.com/inference-gateway/operator/releases/latest/download/namespace-install.yaml -n my-namespace
```

**Includes:**

- Operator deployment
- Namespace-scoped RBAC (no ClusterRole)
- Service accounts
- ConfigMaps and Secrets

**Use when:**

- Multi-tenant Kubernetes clusters
- Limited cluster permissions
- Multiple operator instances in different namespaces
- Enhanced security isolation

## GitOps Integration

### ArgoCD Application

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: inference-gateway-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/inference-gateway/operator
    targetRevision: v0.2.1 # Pin to specific version
    path: manifests # Use this directory
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

### Flux Kustomization

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1beta2
kind: Kustomization
metadata:
  name: inference-gateway-operator
  namespace: flux-system
spec:
  interval: 10m
  sourceRef:
    kind: GitRepository
    name: inference-gateway-operator
  path: "./manifests"
  targetNamespace: inference-gateway-system
  prune: true
```

## Advanced Usage

### Custom Namespace Installation

```bash
# Create custom namespace
kubectl create namespace my-inference-system

# Install with custom namespace
curl -L https://github.com/inference-gateway/operator/releases/latest/download/install.yaml | \
  sed 's/namespace: inference-gateway-system/namespace: my-inference-system/g' | \
  kubectl apply -f -
```

### Custom Namespace Installation

The operator defaults to `inference-gateway-system` but can be deployed to any namespace:

```bash
# Method 1: Using sed replacement
curl -L https://github.com/inference-gateway/operator/releases/latest/download/install.yaml | \
  sed 's/inference-gateway-system/my-custom-namespace/g' | \
  kubectl apply -f -

# Method 2: Generate custom manifests (development)
task manifests-for-namespace NAMESPACE=my-custom-namespace
kubectl apply -f manifests/my-custom-namespace/install.yaml
```

### Air-Gapped Installation

```bash
# Download manifests
curl -L -O https://github.com/inference-gateway/operator/releases/latest/download/install.yaml
curl -L -O https://github.com/inference-gateway/operator/releases/latest/download/crds.yaml
curl -L -O https://github.com/inference-gateway/operator/releases/latest/download/namespace-install.yaml

# Apply in air-gapped environment
kubectl apply -f install.yaml
```

## Upgrade Considerations

1. **CRDs**: Always upgrade CRDs before upgrading the operator
2. **Backward Compatibility**: The operator supports rolling upgrades
3. **Resource Validation**: New CRD versions may have additional validation rules

```bash
# Safe upgrade process
kubectl apply -f https://github.com/inference-gateway/operator/releases/download/v0.3.0/crds.yaml
kubectl apply -f https://github.com/inference-gateway/operator/releases/download/v0.3.0/install.yaml
```

## Troubleshooting

### Common Issues

1. **CRD Installation Failures**: Ensure you have cluster-admin permissions
2. **Operator Pod CrashLoopBackOff**: Check if CRDs are properly installed
3. **Webhook Failures**: Verify network policies allow webhook communication

### Verification Commands

```bash
# Check CRDs
kubectl get crd | grep inference-gateway

# Check operator status
kubectl get pods -n inference-gateway-system

# Check Gateway resources
kubectl get gateways --all-namespaces
```

## Architecture

```
┌─────────────────┐    ┌────────────────────┐    ┌──────────────────────┐
│   install.yaml  │    │     crds.yaml.     │    │namespace-install.yaml│
│                 │    │                    │    │                      │
│ ┌─────────────┐ │    │  ┌──────────────┐  │    │    ┌─────────────┐   │
│ │ Namespace   │ │    │  │ Gateway CRD  │  │    │    │ Deployment  │   │
│ │ CRDs        │ │◄───┤  │              │  │    │    │ RBAC        │   │
│ │ Deployment  │ │    │  │              │  │    │    │ ConfigMaps  │   │
│ │ RBAC        │ │    │  └──────────────┘  │    │    └─────────────┘   │
│ │ Monitoring  │ │    └────────────────────┘    └──────────────────────┘
│ └─────────────┘ │
└─────────────────┘

    Complete               CRDs Only                Operator Only
   Installation                                     (No CRDs)
```

## License

This project is licensed under the MIT License - see the [LICENSE](../LICENSE) file for details.
