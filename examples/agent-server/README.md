# Agent Server Example

Deploy a **Google Calendar Agent** — an A2A (Agent-to-Agent) worker that can create, read, update, and delete Google Calendar events. In this example it runs in **mock mode** (`GOOGLE_CALENDAR_MOCK_MODE=true`), so no real Google credentials are needed; all calendar operations return synthetic mock data.

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

## Run

1. *(Optional)* To use a real Google account instead of mock mode, set `GOOGLE_CALENDAR_MOCK_MODE: "false"` in `agent.yaml` and supply Google OAuth credentials via a Kubernetes Secret. Refer to the [google-calendar-agent documentation](https://github.com/inference-gateway/google-calendar-agent) for the required environment variables.

2. Update the `apiKey.secretRef` in `agent.yaml` to point to a Secret that contains the API key for the LLM provider the agent should use:

   ```bash
   kubectl create secret generic your-api-key \
     --from-literal=api-key=<YOUR_API_KEY> \
     -n agents
   ```

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
