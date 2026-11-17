# Gateway with Guardrails Example

An Inference Gateway that evaluates [OPA/Rego](https://www.openpolicyagent.org/) policies on every request before it reaches a provider. Policies live in a ConfigMap referenced by `spec.guardrails.configMapRef`.

## What the operator does

| `spec.guardrails` field | Effect on the gateway Deployment |
| --- | --- |
| `enabled: true` | Sets `GUARDRAILS_ENABLED=true` and `GUARDRAILS_POLICY_DIR=/etc/inference-gateway/guardrails` |
| `configMapRef.name` | Mounts the ConfigMap read-only at `/etc/inference-gateway/guardrails` - every key becomes a policy file |
| `failMode` | Sets `GUARDRAILS_FAIL_MODE` - `deny` (default) becomes `closed`, `allow` becomes `open` |
| `externalUrl` | Sets `GUARDRAILS_EXTERNAL_URL` - the gateway also calls an external guardrail service |
| `externalTimeout` | Sets `GUARDRAILS_EXTERNAL_TIMEOUT` (default `5s`) - only emitted together with `externalUrl` |

Each `.rego` file must be in `package guardrails` and expose a `main` rule returning one of `allow`, `block` (with a `message`), `redact` or `warn`. See the gateway's [guardrails examples](https://github.com/inference-gateway/inference-gateway/tree/main/examples/guardrails) for redaction and authorization policies.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- An [OpenAI API key](https://platform.openai.com/api-keys)

## Run

1. Set `OPENAI_API_KEY` in the `inference-gateway-providers-secret` Secret in `gateway.yaml`.

2. Apply the manifest:

   ```bash
   kubectl apply -f .
   kubectl get gateway.core.inference-gateway.com -n inference-gateway -w
   ```

3. Check the env vars and the policy mount:

   ```bash
   kubectl get deployment inference-gateway -n inference-gateway \
     -o jsonpath='{range .spec.template.spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' | grep GUARDRAILS
   kubectl exec -n inference-gateway deploy/inference-gateway -- ls /etc/inference-gateway/guardrails
   ```

4. Port-forward and send an allowed request, then one the `block_pii.rego` policy refuses:

   ```bash
   kubectl port-forward -n inference-gateway svc/inference-gateway 8080:8080 &

   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "openai/gpt-4o-mini", "messages": [{"role": "user", "content": "Hello!"}]}'

   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "openai/gpt-4o-mini", "messages": [{"role": "user", "content": "My card is 4111 1111 1111 1111"}]}'
   ```

   The second request is rejected with `request contains sensitive payment information` and never reaches OpenAI.

5. Edit the ConfigMap to change enforcement and re-apply. The ConfigMap is mounted as a volume, so the kubelet syncs the new files into the pod - restart the Deployment (`kubectl rollout restart deployment/inference-gateway -n inference-gateway`) if the gateway only loads policies at startup.

## Cleanup

```bash
kubectl delete -f .
```
