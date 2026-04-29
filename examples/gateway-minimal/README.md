# Gateway Minimal Example

A minimal Inference Gateway deployment with a single OpenAI provider. This is the simplest possible configuration — one replica, one provider, no ingress, no TLS.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- An [OpenAI API key](https://platform.openai.com/api-keys)

## Run

1. Set your OpenAI API key in `gateway.yaml`:

   Open `gateway.yaml` and replace the empty string in `openai-secret` with your key:
   ```yaml
   stringData:
     api_key: "<YOUR_OPENAI_API_KEY>"
   ```

2. Apply the manifest:

   ```bash
   kubectl apply -f .
   ```

3. Wait for the Gateway to become ready:

   ```bash
   kubectl get gateway -n inference-gateway -w
   ```

4. Forward the gateway port to your local machine:

   ```bash
   kubectl port-forward -n inference-gateway svc/simple-gateway 8080:8080
   ```

5. Send a test request:

   ```bash
   curl http://localhost:8080/v1/chat/completions \
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
