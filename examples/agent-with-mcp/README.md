# Agent with MCP Client Example

An `Agent` whose MCP client connects to the memory server from [`../mcp-server/`](../mcp-server/), discovers its tools (`memory_set`, `memory_get`, `memory_delete`, `memory_list`) and exposes them to the agent's LLM. The agent also advertises a public URL in its agent card via `spec.card.url`.

`Agent.spec.mcp` is the client side. It is unrelated to the `MCP` CRD, which deploys an MCP server workload, and to `Gateway.spec.mcp`, which connects the gateway to MCP servers.

## What the operator does

| `spec` field | Env var on the agent container | Default |
| --- | --- | --- |
| `mcp.enabled` | `A2A_MCP_ENABLED` (always emitted) | `false` |
| `mcp.servers` | `A2A_MCP_SERVERS` (comma-separated, omitted when empty) | - |
| `mcp.endpoint` | `A2A_MCP_ENDPOINT` (appended to each server URL) | `/mcp` |
| `mcp.refreshInterval` | `A2A_MCP_REFRESH_INTERVAL` | `5m` |
| `mcp.dialTimeout` | `A2A_MCP_DIAL_TIMEOUT` | `30s` |
| `mcp.callTimeout` | `A2A_MCP_CALL_TIMEOUT` | `30s` |
| `mcp.maxRetries` | `A2A_MCP_MAX_RETRIES` (`0` retries forever) | `0` |
| `mcp.retryInterval` | `A2A_MCP_RETRY_INTERVAL` | `2s` |
| `mcp.retryMaxInterval` | `A2A_MCP_RETRY_MAX_INTERVAL` | `30s` |
| `card.url` | `A2A_AGENT_URL` | `http://<name>.<namespace>.svc.cluster.local:<port>` |

The connection knobs are only emitted when `mcp.enabled` is `true`. Set `card.url` when the agent sits behind an Ingress or Gateway and callers should reach it by that hostname - it only changes what the agent advertises, the operator does not create the route.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- A Gateway for the agent's LLM calls:
  ```bash
  kubectl apply -f ../gateway-minimal/
  ```
- The MCP memory server - follow [`../mcp-server/README.md`](../mcp-server/README.md) to build the image and apply `mcp.yaml`. It is served at `http://mcp-memory-server-service.mcp.svc.cluster.local:8080/mcp`.

## Run

1. Set `OPENAI_API_KEY` in the `agent-llm-secret` Secret in `agent.yaml`.

2. Apply the manifest:

   ```bash
   kubectl apply -f .
   kubectl get agent -n agents -w
   ```

3. Check the MCP client env vars and the advertised URL:

   ```bash
   kubectl get deployment google-calendar-agent -n agents \
     -o jsonpath='{range .spec.template.spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' | grep -E 'A2A_(MCP|AGENT_URL)'

   kubectl run -it --rm curl --image=curlimages/curl --restart=Never -- \
     curl -s http://google-calendar-agent.agents.svc.cluster.local:8080/.well-known/agent-card.json
   ```

   The card reports `"url": "https://calendar-agent.example.com"`.

4. Follow the agent logs to see it discover the memory server's tools:

   ```bash
   kubectl logs -n agents deploy/google-calendar-agent -f
   ```

## Cleanup

```bash
kubectl delete -f .
```
