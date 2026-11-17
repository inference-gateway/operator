# Orchestrator example

End-to-end deployment of an `Orchestrator` that:

- Receives messages from a chat channel (currently **Telegram**; more channels coming)
- Reasons with an LLM via the **Inference Gateway** (DeepSeek in this example)
- Delegates work to A2A worker **Agents** (`google-calendar-agent` in mock mode + `mock-agent`)
- Persists conversation context in **Redis** so sessions survive pod restarts
- Uses tools from three **MCP** servers running [`mcp-server-time`](mcp-server-time) (see [`06-mcps.yaml`](06-mcps.yaml)):
  - `mcp-time-a` and `mcp-time-b` are labeled `mcp-group: group1` and found by `spec.mcp.serviceDiscovery`
  - `mcp-time-direct` has no group label - the Orchestrator reaches it by URL through `spec.mcp.servers`, sending an `x-api-key` header read from the `mcp-time-credentials` Secret, limited to `get_time` by `includeTools`, with a 10-second `timeoutSeconds`

```text
              ┌─────────────────────┐
   Telegram ◀▶│    Orchestrator     │──▶ Gateway ──▶ DeepSeek
   (long-poll)│   (infer daemon)    │
              └─────────────────────┘
                       │
                       ├──▶ Redis (conversation storage)
                       ├──▶ google-calendar-agent (A2A)
                       ├──▶ mock-agent (A2A)
                       ├──▶ mcp-time-a, mcp-time-b (MCP, discovered by label)
                       └──▶ mcp-time-direct (MCP, by URL + x-api-key header)
```

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed (`task cluster:create && task install && task deploy` from the repo root)
- A **Telegram bot token** - DM [@BotFather](https://t.me/BotFather), use `/newbot`, follow the prompts
- Your **Telegram user ID** - DM [@userinfobot](https://t.me/userinfobot); it replies with your numeric ID
- A **DeepSeek API key** - sign up at [platform.deepseek.com](https://platform.deepseek.com)

## Run

1. Edit [`01-secrets.yaml`](01-secrets.yaml) and replace the three `REPLACE_WITH_*` placeholders.

2. Build the local `mcp-server-time:dev` image and import it into the k3d cluster:

   ```bash
   docker build -t mcp-server-time:dev mcp-server-time
   k3d image import mcp-server-time:dev --cluster dev
   ```

3. Apply everything:

   ```bash
   kubectl apply -f .
   ```

4. Watch pods come up:

   ```bash
   kubectl get pods -n inference-gateway -w
   kubectl get pods -n orchestrator -w
   kubectl get pods -n agents -w
   kubectl get pods -n mcp -w
   ```

5. Tail Orchestrator logs (the bot starts polling Telegram once `READY=true`):

   ```bash
   kubectl get orchestrator -n orchestrator
   kubectl logs -n orchestrator deploy/orchestrator -f
   ```

6. Send messages to your Telegram bot. Try:
   - *"What's on my calendar today?"* - delegated to `google-calendar-agent` (returns mock data)
   - *"Echo this back"* - exercises `mock-agent`
   - *"What's the capital of France?"* - answered directly via the Gateway's LLM

## Cleanup

```bash
kubectl delete -f .
```

## Notes

- The Orchestrator runs as a **singleton** (`replicas: 1`, `strategy: Recreate`) because Telegram allows only one active `getUpdates` consumer per bot token. For HA today, run multiple `Orchestrator` resources with different tokens and disjoint `allowedUsers`.
- `google-calendar-agent` runs in **mock mode** (`GOOGLE_CALENDAR_MOCK_MODE=true`) - calendar operations return synthetic data, no real Google credentials needed.
- The Agents route their internal LLM calls through the **Gateway** (`A2A_AGENT_CLIENT_BASE_URL`), so DeepSeek auth is configured in one place (the Gateway's provider Secret).
- `kubectl logs` only emits when the CLI honors `INFER_LOGGING_STDOUT=true` - supported in CLI `0.13.0` and later.
- **Conversation persistence**: conversations are stored in Redis (`04-redis.yaml`) via `INFER_STORAGE_TYPE=redis`. When the Orchestrator pod is restarted (e.g. because a discovered agent was added or removed, which triggers a ConfigMap roll), the full conversation context for every user session is restored from Redis automatically.
  - Redis AOF persistence (`--appendonly yes`) is enabled, so conversations also survive Redis pod restarts.
  - For production use, back the Redis pod with a `PersistentVolumeClaim`.

## Adding more channels

When additional channels (Slack, Discord, …) land in the CLI, just add another block under `channels:` in [`05-orchestrator.yaml`](05-orchestrator.yaml) - and add the corresponding credentials Secret to [`01-secrets.yaml`](01-secrets.yaml). No new directory needed.
