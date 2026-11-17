# Orchestrator Telegram Example

An `Orchestrator` that answers Telegram messages (`spec.channels.telegram`) with an LLM reached through a `Gateway` (`spec.gateway.ref`). The Deployment runs as a singleton because Telegram allows only one `getUpdates` consumer per bot token.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- The [`gateway-minimal`](../gateway-minimal/) Gateway running with its OpenAI key set:
  ```bash
  kubectl apply -f ../gateway-minimal/
  ```
- A Telegram bot token from [@BotFather](https://t.me/BotFather) (`/newbot`) and your numeric user ID from [@userinfobot](https://t.me/userinfobot)

## Run

1. Create the bot credentials:

   ```bash
   kubectl create secret generic telegram-bot-credentials -n inference-gateway \
     --from-literal=token='<TELEGRAM_BOT_TOKEN>' --from-literal=allowedUsers='<YOUR_USER_ID>'
   ```

2. Apply the manifest and wait for it to become ready:

   ```bash
   kubectl apply -f orchestrator.yaml
   kubectl get orchestrator telegram-bot -n inference-gateway -w
   ```

3. Send your bot a message on Telegram and watch the daemon answer it:

   ```bash
   kubectl logs -n inference-gateway deploy/telegram-bot -f
   ```

## Cleanup

```bash
kubectl delete -f orchestrator.yaml
kubectl delete secret telegram-bot-credentials -n inference-gateway
```
