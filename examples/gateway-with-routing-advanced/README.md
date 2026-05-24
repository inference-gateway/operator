# Gateway with Advanced Routing Example

Two ways of exposing the Inference Gateway via the Kubernetes **Gateway API**:

1. **Default mode** (`inference-gateway-advanced`) - the operator creates and owns both the upstream `Gateway` and the `HTTPRoute`. TLS is terminated at the Gateway listener via cert-manager.
2. **Advanced mode** (`inference-gateway-shared`) - a platform team owns a shared upstream `Gateway` and the operator only creates the `HTTPRoute` attached via `parentRefs`. This is the SIG-Network persona split: platform team owns the Gateway, application team owns the Route.

## Note on data-plane behavior (annotations, rate-limiting, CORS)

The old Ingress-based example used `nginx.ingress.kubernetes.io/*` annotations for rate limiting, body-size, CORS, etc. These have **no portable equivalent in the Gateway API** - data-plane behavior is configured through:

- **Built-in `HTTPRouteFilter`s** (URL rewriting, header modification, redirects) - portable across implementations.
- **Implementation-specific Policies** - for Envoy Gateway, look at `BackendTrafficPolicy` (rate limiting, retries) and `ClientTrafficPolicy` (CORS, body limits). For Istio, it's `VirtualService` / `EnvoyFilter`. These are NOT portable.

For the simple things, prefer filters. For platform-wide quotas/CORS, prefer Policies and keep them in the platform team's repo, not the application CR.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed
- **Envoy Gateway** (or another Gateway API implementation) installed, with the `envoy` GatewayClass available
- **cert-manager** with a `letsencrypt-prod` `ClusterIssuer` configured:
  ```bash
  kubectl apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: letsencrypt-prod
  spec:
    acme:
      server: https://acme-v02.api.letsencrypt.org/directory
      email: you@example.com
      privateKeySecretRef:
        name: letsencrypt-prod
      solvers:
        - http01:
            gatewayHTTPRoute:
              parentRefs:
                - name: inference-gateway-advanced
                  namespace: inference-gateway
                  kind: Gateway
                  group: gateway.networking.k8s.io
  EOF
  ```
  > Note: cert-manager's Gateway API integration uses `gatewayHTTPRoute` solvers, not `ingress` solvers - different shape than the legacy Ingress flow.
- For the **advanced-mode** example: a pre-existing shared Gateway named `shared-platform-gateway` in the `platform-system` namespace, with an `allowedRoutes` policy permitting routes from the `inference-gateway` namespace.
- DNS records pointing your hostnames at the Envoy load-balancer IP
- An [OpenAI API key](https://platform.openai.com/api-keys)

## Run

1. Set your OpenAI API key in `gateway.yaml`.
2. Replace the placeholder hostnames with your real DNS names.
3. Apply:
   ```bash
   kubectl apply -f .
   ```
4. Wait for resources to settle:
   ```bash
   kubectl get gateway.core.inference-gateway.com -n inference-gateway -w
   kubectl get gateway.gateway.networking.k8s.io -n inference-gateway
   kubectl get httproute -n inference-gateway
   kubectl get certificate -n inference-gateway
   ```
5. Send a test request once the upstream Gateway reports `Programmed=True`:
   ```bash
   curl https://api.inference-gateway.local/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "gpt-4o-mini",
       "messages": [{"role": "user", "content": "Hello!"}]
     }'
   ```

## Cleanup

```bash
kubectl delete -f .
```
