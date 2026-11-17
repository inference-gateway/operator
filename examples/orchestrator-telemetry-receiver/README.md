# Orchestrator Telemetry Receiver Example

An `Orchestrator` that runs an OTLP/HTTP receiver, and a `Gateway` that pushes its traces to it. Use the receiver when the `infer` daemon should collect spans from the workloads it calls instead of every workload exporting to a separate collector.

## What the operator does

| `spec.telemetry` field | Effect |
| --- | --- |
| `enabled: true` | Required - the receiver address is only passed to the daemon when telemetry is on |
| `receiver.enabled: true` | Sets `INFER_TELEMETRY_RECEIVER_ADDRESS=0.0.0.0:<port>`, adds the container port and creates the `<name>-receiver` Service (port name `otlp-http`) |
| `receiver.port` | The receiver and Service port (default `4318`) |

Disabling the receiver removes the `<name>-receiver` Service again. Enabling the receiver together with `spec.channels` sets the `ReceiverPortContention` condition, because agent subprocesses spawned by the daemon contend for the receiver port.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- A [DeepSeek API key](https://platform.deepseek.com/api_keys) (or swap the provider and `spec.agent.model`)

## Run

1. Set `DEEPSEEK_API_KEY` in the `inference-gateway-providers-secret` Secret in `orchestrator.yaml`.

2. Apply the manifest:

   ```bash
   kubectl apply -f .
   kubectl get orchestrator -n orchestrator -w
   ```

3. Check the receiver Service and env var:

   ```bash
   kubectl get svc orchestrator-receiver -n orchestrator
   kubectl get deployment orchestrator -n orchestrator \
     -o jsonpath='{range .spec.template.spec.containers[0].env[*]}{.name}={.value}{"\n"}{end}' | grep RECEIVER
   ```

4. Send a request through the Gateway so it emits a span:

   ```bash
   kubectl port-forward -n inference-gateway svc/inference-gateway 8080:8080 &

   curl http://localhost:8080/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{"model": "deepseek/deepseek-v4-flash", "messages": [{"role": "user", "content": "Hello!"}]}'
   ```

   The Gateway exports the span to `http://orchestrator-receiver.orchestrator.svc.cluster.local:4318`. An `Agent` reports the same way through its own `spec.telemetry.traces.exporter.otlp.endpoint`.

## Cleanup

```bash
kubectl delete -f .
```
