# Gateway with Advanced Ingress Example

An Inference Gateway deployment in **production** mode with an advanced Ingress configuration:

- Multiple hostnames and path rules
- Per-path routing (`/v1` and `/health` on a second hostname)
- Rate limiting (100 requests/minute via nginx annotations)
- CORS allow-list
- Let's Encrypt TLS with separate secrets per hostname

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- **nginx Ingress controller** deployed in the cluster
- **cert-manager** with a `letsencrypt-prod` `ClusterIssuer` configured:
  ```bash
  # Example ClusterIssuer (adjust email and server as needed)
  kubectl apply -f - <<EOF
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: letsencrypt-prod
  spec:
    acme:
      server: https://acme-v02.api.letsencrypt.org/directory
      email: you@example.com
      privateKeySecretRef:
        name: letsencrypt-prod
      solvers:
        - http01:
            ingress:
              class: nginx
  EOF
  ```
- DNS records pointing `api.inference-gateway.local` and `gateway.example.com` to your ingress load-balancer IP
- An [OpenAI API key](https://platform.openai.com/api-keys)

## Run

1. Set your OpenAI API key in `gateway.yaml`:

   Open `gateway.yaml` and replace the empty string in `inference-gateway-providers-secret`:
   ```yaml
   stringData:
     OPENAI_API_KEY: "<YOUR_OPENAI_API_KEY>"
   ```

2. Update the hostnames in `gateway.yaml` to match your real DNS names (replace `api.inference-gateway.local` and `gateway.example.com`).

3. Apply the manifest:

   ```bash
   kubectl apply -f .
   ```

4. Wait for the Gateway, Ingress, and TLS certificates to become ready:

   ```bash
   kubectl get gateway -n inference-gateway -w
   kubectl get ingress -n inference-gateway
   kubectl get certificate -n inference-gateway
   ```

   cert-manager will automatically request and provision the TLS certificates. This may take a minute or two.

5. Send a test request:

   ```bash
   curl https://api.inference-gateway.local/v1/chat/completions \
     -H "Content-Type: application/json" \
     -d '{
       "model": "gpt-4o-mini",
       "messages": [{"role": "user", "content": "Hello!"}]
     }'
   ```

## Rate Limiting

The ingress is configured to allow **100 requests per minute** per client IP. Requests beyond this limit will receive an HTTP 429 response. Adjust the annotation values in `gateway.yaml` to change the limit:

```yaml
annotations:
  nginx.ingress.kubernetes.io/rate-limit: "100"
  nginx.ingress.kubernetes.io/rate-limit-window: "1m"
```

## Cleanup

```bash
kubectl delete -f .
```
