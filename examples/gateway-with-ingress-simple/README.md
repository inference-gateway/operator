# Gateway with Simple Ingress Example

An Inference Gateway deployment exposed via a Kubernetes Ingress. The operator automatically configures the nginx ingress class, TLS via cert-manager (using the self-signed cluster issuer in development mode), and a single hostname.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
  > `task cluster:create` provisions a k3d cluster with **cert-manager** and **nginx-ingress** pre-installed.
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

3. Wait for the Gateway and Ingress to become ready:

   ```bash
   kubectl get gateway -n inference-gateway -w
   kubectl get ingress -n inference-gateway
   ```

4. Add a local DNS entry for the ingress hostname (if using a local cluster):

   ```bash
   echo "127.0.0.1 api.inference-gateway.local" | sudo tee -a /etc/hosts
   ```

5. Send a test request through the ingress:

   ```bash
   curl -k https://api.inference-gateway.local/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "gpt-4o-mini",
       "messages": [{"role": "user", "content": "Hello!"}]
     }'
   ```

   > The `-k` flag skips TLS verification for the self-signed certificate used in development.

## Cleanup

```bash
kubectl delete -f .
```
