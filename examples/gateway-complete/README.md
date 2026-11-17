# Gateway Complete Example

A full-featured Inference Gateway deployment showcasing all available options:

- **Horizontal Pod Autoscaler (HPA)** - scales between 3 and 10 replicas based on CPU utilization
- **Telemetry** - Prometheus metrics exposed on port 9464
- **OIDC authentication** (disabled by default, toggle with `auth.enabled: true`)
- **Multiple AI providers** - OpenAI, Anthropic, Groq, Cohere, Cloudflare, DeepSeek, Ollama, Google, and Mirelo
- **Audio API** (`spec.audio.enabled: true`) - required by the Mirelo provider for `POST /v1/audio/sfx`
- **MCP servers** (disabled by default, toggle with `mcp.enabled: true`)
- **Kubernetes Gateway API** (`spec.gatewayAPI`) - an operator-managed `Gateway` and `HTTPRoute` with TLS via cert-manager
- **Service** (`spec.service`) - the gateway Service's `type`, `port` and `annotations`
- **ServiceAccount** (`spec.serviceAccount`) - an operator-created `custom-gateway-sa` the pods run as

Focused examples cover the remaining features: [`gateway-with-guardrails`](../gateway-with-guardrails/), [`gateway-with-mtls`](../gateway-with-mtls/) and [`gateway-with-model-routing`](../gateway-with-model-routing/).

Use this as a reference when you need to understand what each field does, or as a starting template to trim down to your specific needs.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
  > `task cluster:create` provisions a k3d cluster with **cert-manager** (plus a `selfsigned-cluster-issuer` ClusterIssuer), the **Kubernetes Gateway API CRDs** and **Envoy Gateway** (which provides the `envoy` GatewayClass) pre-installed. No Ingress controller is installed - the operator creates no `Ingress`.
- API keys for whichever providers you want to enable

## Run

1. Uncomment and populate the `inference-gateway-providers-secret` block in `gateway.yaml`. `gateway.yaml` ships with Groq, Cohere, Cloudflare, DeepSeek, Ollama, Google and Mirelo set to `enabled: true`, and the operator injects every enabled provider's `secretKeyRef` into the gateway container. Keep every key from the commented Secret block - a missing key leaves the pod in `CreateContainerConfigError`:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: inference-gateway-providers-secret
     namespace: inference-gateway
   type: Opaque
   stringData:
     OPENAI_API_KEY: ""
     ANTHROPIC_API_KEY: ""
     GROQ_API_KEY: "<YOUR_GROQ_API_KEY>"
     COHERE_API_KEY: ""
     CLOUDFLARE_API_KEY: ""
     DEEPSEEK_API_KEY: "<YOUR_DEEPSEEK_API_KEY>"
     OLLAMA_API_KEY: ""
     GOOGLE_API_KEY: ""
     MIRELO_API_KEY: ""
   ```

2. In `gateway.yaml`, set `enabled: false` on each provider you skip, or keep its key in the Secret (an empty value is fine). OpenAI and Anthropic ship disabled - set `enabled: true` to use them.

3. *(Optional)* Enable OIDC authentication by setting `auth.enabled: true` and filling in `inference-gateway-auth-secret`.

4. *(Optional)* Enable MCP proxy by setting `mcp.enabled: true` and updating the server URLs.

5. Apply the manifests:

   ```bash
   kubectl apply -f .
   ```

6. Watch all resources come up:

   ```bash
   kubectl get gateway.core.inference-gateway.com,hpa,certificate -n inference-gateway -w
   kubectl get gateway.gateway.networking.k8s.io,httproute -n inference-gateway
   ```

   > `gateway` is ambiguous here: both this operator's CRD and the Kubernetes Gateway API register that resource name, so qualify it with the API group.

7. Add a local DNS entry for the `httpRoute` hostname:

   ```bash
   echo "127.0.0.1 api.inference-gateway.local" | sudo tee -a /etc/hosts
   ```

8. Port-forward Envoy and send a test request:

   ```bash
   # Envoy Gateway names the data-plane Service envoy-<namespace>-<gateway-name>-<hash>, so look it up by label
   ENVOY_SVC=$(kubectl -n envoy-gateway-system get svc -o name \
     -l gateway.envoyproxy.io/owning-gateway-namespace=inference-gateway,gateway.envoyproxy.io/owning-gateway-name=inference-gateway)
   kubectl -n envoy-gateway-system port-forward "$ENVOY_SVC" 8443:443 &

   curl -k https://api.inference-gateway.local:8443/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "groq/llama-3.3-70b-versatile",
       "messages": [{"role": "user", "content": "Hello!"}]
     }'
   ```

   > The `-k` flag skips TLS verification for the self-signed certificate used in development.

## Observability

Once the gateway is running, Prometheus metrics are available on port 9464 of each pod:

```bash
kubectl port-forward -n inference-gateway svc/inference-gateway 9464:9464
curl http://localhost:9464/metrics
```

## Cleanup

```bash
kubectl delete -f .
```
