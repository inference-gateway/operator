# Agent Server Example

Deploy a **Google Calendar Agent** - an A2A (Agent-to-Agent) worker that can create, read, update, and delete Google Calendar events. In this example it runs in **mock mode** (`GOOGLE_CALENDAR_MOCK_MODE=true`), so no real Google credentials are needed; all calendar operations return synthetic mock data.

The agent exposes an A2A-compatible HTTP endpoint and can be registered with an Inference Gateway Orchestrator for LLM-driven task delegation.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- An Inference Gateway already running in the cluster (the agent routes its internal LLM calls through it). Deploy one first if you haven't:
  ```bash
  kubectl apply -f ../gateway-minimal/
  ```
  > That example creates a `Gateway` named `simple-gateway` in the `inference-gateway` namespace, so its Service is `simple-gateway.inference-gateway.svc.cluster.local`. `agent.yaml` ships with `llm.baseURL` pointing there - adjust it if your gateway has a different name or namespace.

## Run

1. *(Optional)* To use a real Google account instead of mock mode, set `GOOGLE_CALENDAR_MOCK_MODE: "false"` in `agent.yaml` and supply Google OAuth credentials via a Kubernetes Secret. Refer to the [google-calendar-agent documentation](https://github.com/inference-gateway/google-calendar-agent) for the required environment variables.

2. Supply the API key the agent uses for its own LLM calls. `agent.yaml` reads it from `spec.agent.llm.apiKeySecretRef` (Secret `agent-llm-secret`, key `OPENAI_API_KEY`), which the operator emits as `A2A_AGENT_CLIENT_API_KEY`. Either fill in the stub Secret shipped in `agent.yaml`, or create it yourself:

   ```bash
   kubectl create secret generic agent-llm-secret \
     --from-literal=OPENAI_API_KEY=<YOUR_API_KEY> \
     -n agents
   ```

   > The Secret must be in the `agents` namespace - the Agent's secret reference is namespace-local. `spec.agent.apiKey` is deprecated and ignored by the controller; only `spec.agent.llm.apiKeySecretRef` is read.

3. Apply the manifest:

   ```bash
   kubectl apply -f .
   ```

4. Wait for the Agent to become ready:

   ```bash
   kubectl get agent -n agents -w
   kubectl get pods -n agents
   ```

5. Verify the agent's A2A endpoint is reachable from within the cluster:

   ```bash
   kubectl run -it --rm curl --image=curlimages/curl --restart=Never -- \
     curl http://google-calendar-agent.agents.svc.cluster.local:8080/health
   ```

## Cleanup

```bash
kubectl delete -f .
```
