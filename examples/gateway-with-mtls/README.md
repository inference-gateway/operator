# Gateway with TLS and Mutual TLS Example

An Inference Gateway that terminates TLS itself, requires a client certificate from inbound callers, and presents a client certificate to providers. cert-manager issues every certificate from a throwaway CA.

## What the operator does

| Field | Effect on the gateway Deployment |
| --- | --- |
| `spec.server.tls.enabled` | Sets `SERVER_TLS_CERT_PATH` / `SERVER_TLS_KEY_PATH`, renames the container and Service port from `http` to `https`, and switches the health probes to HTTPS |
| `spec.server.tls.certificateRef` / `keyRef` | Mounts each referenced key at `/app/tls/cert` and `/app/tls/key` |
| neither ref set | Mounts the whole `inference-gateway-tls` Secret at `/app/tls` and reads `tls.crt` / `tls.key` from it |
| `spec.server.tls.clientCARef` | Mounts the CA at `/app/tls/client-ca`, sets `SERVER_TLS_CLIENT_CA_PATH`, and switches the liveness/readiness probes to `tcpSocket` because the kubelet has no client certificate |
| `spec.client.tls.caRef` / `certificateRef` / `keyRef` | Mounts each key under `/app/tls/client/{ca,cert,key}` and sets `CLIENT_TLS_CA_PATH`, `CLIENT_TLS_CERT_PATH`, `CLIENT_TLS_KEY_PATH` |

The server port number does not change - `spec.server.port` (default `8080`) serves HTTPS once TLS is enabled. The client CA is trusted on top of the system roots, so public providers keep working with `spec.client.tls` set.

### Using the default `inference-gateway-tls` Secret

Drop `certificateRef` and `keyRef` and create a Secret with the conventional name and keys instead:

```yaml
server:
  tls:
    enabled: true
```

```bash
kubectl create secret tls inference-gateway-tls --cert=tls.crt --key=tls.key -n inference-gateway
```

## Prerequisites

- Kubernetes cluster with cert-manager and the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
  > `task cluster:create` installs cert-manager.
- An [OpenAI API key](https://platform.openai.com/api-keys)

## Run

1. Set `OPENAI_API_KEY` in the `inference-gateway-providers-secret` Secret in `gateway.yaml`.

2. Apply the manifests (`certificates.yaml` sorts first and creates the namespace):

   ```bash
   kubectl apply -f .
   kubectl get certificate -n inference-gateway
   kubectl get gateway.core.inference-gateway.com -n inference-gateway -w
   ```

3. Check the port name and the probes:

   ```bash
   kubectl get svc inference-gateway -n inference-gateway -o jsonpath='{.spec.ports[0].name}'
   kubectl get deployment inference-gateway -n inference-gateway \
     -o jsonpath='{.spec.template.spec.containers[0].readinessProbe}'
   ```

   The port is named `https` and the probe is a `tcpSocket` probe.

4. Extract the client certificate and call the gateway:

   ```bash
   kubectl get secret gateway-client-tls -n inference-gateway -o jsonpath='{.data.ca\.crt}' | base64 -d > ca.crt
   kubectl get secret gateway-client-tls -n inference-gateway -o jsonpath='{.data.tls\.crt}' | base64 -d > client.crt
   kubectl get secret gateway-client-tls -n inference-gateway -o jsonpath='{.data.tls\.key}' | base64 -d > client.key

   kubectl port-forward -n inference-gateway svc/inference-gateway 8080:8080 &

   curl --cacert ca.crt --cert client.crt --key client.key https://localhost:8080/v1/models
   ```

   The same call without `--cert` / `--key` fails the TLS handshake.

5. To exercise provider mTLS, set `OPENAI_API_URL` to an OpenAI-compatible server that requires a client certificate signed by the example CA. The gateway presents `gateway-client-tls` to it.

## Cleanup

```bash
kubectl delete -f .
rm -f ca.crt client.crt client.key
```
