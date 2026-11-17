# Daemon Binding Example

An `Orchestrator` that only serves the daemon's AG-UI binding (`spec.daemon.binding`) - the WebSocket the chat TUI, the browser extension and other daemons sync sessions over. The binding always serves TLS, and clients authenticate with the token from `tokenSecretRef`.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- `openssl`

## Run

1. Create the namespace, a self-signed certificate for `localhost` and the token:

   ```bash
   kubectl create namespace orchestrators
   openssl req -x509 -newkey rsa:2048 -nodes -days 30 -subj /CN=localhost \
     -addext subjectAltName=DNS:localhost -keyout tls.key -out tls.crt
   kubectl create secret tls daemon-binding-tls -n orchestrators --cert=tls.crt --key=tls.key
   kubectl create secret generic daemon-binding -n orchestrators --from-literal=token="$(openssl rand -hex 24)"
   ```

2. Apply the manifest and wait for it to become ready:

   ```bash
   kubectl apply -f orchestrator.yaml
   kubectl get orchestrator daemon -n orchestrators -w
   ```

3. Reach the binding from your machine:

   ```bash
   kubectl port-forward -n orchestrators svc/daemon-binding 52789
   ```

## Cleanup

```bash
kubectl delete namespace orchestrators
rm tls.crt tls.key
```
