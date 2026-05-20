# Gateway with Simple Routing Example

An Inference Gateway exposed via the Kubernetes **Gateway API** (`gateway.networking.k8s.io/v1`). The operator creates one upstream `Gateway` (HTTP/80 listener) and one `HTTPRoute` attached to it, both owned by the project `Gateway` CR.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
  > `task cluster:create` provisions a k3d cluster with **cert-manager**, the **Kubernetes Gateway API CRDs**, and **Envoy Gateway** (registered as the `envoy` GatewayClass) pre-installed.
- An [OpenAI API key](https://platform.openai.com/api-keys)

## Run

1. Set your OpenAI API key in `gateway.yaml`:

   Open `gateway.yaml` and replace the empty string in `inference-gateway-providers-secret`:
   ```yaml
   stringData:
     OPENAI_API_KEY: "<YOUR_OPENAI_API_KEY>"
   ```

2. Apply the manifest:

   ```bash
   kubectl apply -f .
   ```

3. Wait for the resources to become ready:

   ```bash
   kubectl get gateway.core.inference-gateway.com -n inference-gateway -w
   kubectl get gateway.gateway.networking.k8s.io -n inference-gateway
   kubectl get httproute -n inference-gateway
   ```

   The upstream `Gateway` will report `Programmed=True` once Envoy Gateway has provisioned its data plane.

4. Add a local DNS entry for the hostname (if using a local cluster):

   ```bash
   echo "127.0.0.1 api.inference-gateway.local" | sudo tee -a /etc/hosts
   ```

5. Port-forward Envoy and send a test request:

   ```bash
   # Find the Envoy service for this Gateway and port-forward it
   kubectl -n envoy-gateway-system port-forward svc/envoy-inference-gateway-inference-gateway 8080:80 &

   curl http://api.inference-gateway.local:8080/v1/chat/completions \
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
