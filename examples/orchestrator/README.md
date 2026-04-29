# Orchestrator example

End-to-end deployment of an `Orchestrator` that:

- Receives messages from a chat channel (currently **Telegram**; more channels coming)
- Reasons with an LLM via the **Inference Gateway** (DeepSeek in this example)
- Delegates work to A2A worker **Agents** (`google-calendar-agent` in mock mode + `mock-agent`)

```text
              ┌─────────────────────┐
   Telegram ◀▶│    Orchestrator     │──▶ Gateway ──▶ DeepSeek
   (long-poll)│  (channels-manager) │      ▲
              └─────────────────────┘      │
                       │                   │
                       ├──▶ google-calendar-agent (A2A)
                       └──▶ mock-agent (A2A)
```

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed (`task cluster:create && task install && task deploy` from the repo root)
- A **Telegram bot token** - DM [@BotFather](https://t.me/BotFather), use `/newbot`, follow the prompts
- Your **Telegram user ID** - DM [@userinfobot](https://t.me/userinfobot); it replies with your numeric ID
- A **DeepSeek API key** - sign up at [platform.deepseek.com](https://platform.deepseek.com)

## Run

1. Edit [`01-secrets.yaml`](01-secrets.yaml) and replace the four `REPLACE_WITH_*` placeholders.

2. Apply everything:

   ```bash
   kubectl apply -f .
   ```

3. Watch pods come up:

   ```bash
   kubectl get pods -n inference-gateway -w
   kubectl get pods -n agents -w
   ```

4. Tail Orchestrator logs (the bot starts polling Telegram once `READY=true`):

   ```bash
   kubectl get orchestrator -n inference-gateway
   kubectl logs -n inference-gateway deploy/orchestrator -f
   ```

5. Send messages to your Telegram bot. Try:
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

## Adding more channels

When additional channels (Slack, Discord, …) land in the CLI, just add another block under `channels:` in [`04-orchestrator.yaml`](04-orchestrator.yaml) - and add the corresponding credentials Secret to [`01-secrets.yaml`](01-secrets.yaml). No new directory needed.
