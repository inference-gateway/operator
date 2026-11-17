# Gateway with Model Routing Example

Gateway-native model routing: a client requests a logical alias such as `fast-chat`, and the gateway rotates round-robin through the alias's deployments (`groq/llama-3.3-70b-versatile`, `openai/gpt-4o-mini`), forwarding to the resolved provider and model.

`spec.routing` is unrelated to `spec.gatewayAPI`, which configures north-south Kubernetes Gateway API traffic (see [`../gateway-with-routing-simple/`](../gateway-with-routing-simple/)).

## What the operator does

| `spec.routing` field | Effect on the gateway Deployment |
| --- | --- |
| `enabled: true` | Sets `ROUTING_ENABLED=true` |
| `config` (inline YAML) | Renders it into an operator-owned `<name>-routing` ConfigMap under the key `routing.yaml`, mounts it at `/etc/inference-gateway/routing` and sets `ROUTING_CONFIG_PATH=/etc/inference-gateway/routing/routing.yaml` |
| `configMapRef` (`name`, `key`) | Mounts your own ConfigMap at `/etc/inference-gateway/routing` and sets `ROUTING_CONFIG_PATH=/etc/inference-gateway/routing/<key>` - no `<name>-routing` ConfigMap is created |

Inline `config` wins when both are set. Removing `config` deletes the `<name>-routing` ConfigMap. Every deployment's provider must also be configured under `spec.providers`.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- [OpenAI](https://platform.openai.com/api-keys) and [Groq](https://console.groq.com/keys) API keys

## Run

1. Set `OPENAI_API_KEY` and `GROQ_API_KEY` in the `inference-gateway-providers-secret` Secret in `gateway.yaml`.

2. Apply the manifest:

   ```bash
   kubectl apply -f .
   kubectl get gateway.core.inference-gateway.com -n inference-gateway -w
   ```

3. Inspect the rendered routing ConfigMap:

   ```bash
   kubectl get configmap inference-gateway-routing -n inference-gateway -o jsonpath='{.data.routing\.yaml}'
   ```

4. Port-forward and request the `fast-chat` alias a few times:

   ```bash
   kubectl port-forward -n inference-gateway svc/inference-gateway 8080:8080 &

   for i in 1 2; do
     curl -s http://localhost:8080/v1/chat/completions \
       -H "Content-Type: application/json" \
       -d '{"model": "fast-chat", "messages": [{"role": "user", "content": "Hello!"}]}' | jq -r .model
   done
   ```

   The `model` in the responses alternates between the Groq and OpenAI deployments.

## Using an existing ConfigMap

Replace `config` with a reference to a ConfigMap you manage:

```yaml
routing:
  enabled: true
  configMapRef:
    name: my-routing-config
    key: routing.yaml
```

```bash
kubectl create configmap my-routing-config --from-file=routing.yaml -n inference-gateway
```

## Cleanup

```bash
kubectl delete -f .
```
