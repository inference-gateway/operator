# MCP Server Example

Deploy an **MCP Memory Server** — a Model Context Protocol (MCP) server that provides persistent in-cluster memory storage for LLM interactions. The server is exposed over TLS using a self-signed certificate managed by cert-manager and scales automatically via HPA.

Once deployed, register this server's in-cluster URL in a Gateway resource under `spec.mcp.servers` to allow the Inference Gateway to proxy MCP tool calls to it.

## Prerequisites

- Kubernetes cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
  > `task cluster:create` provisions a k3d cluster with **cert-manager** pre-installed, which is required for the TLS certificate.
- A `selfsigned-cluster-issuer` ClusterIssuer available in the cluster (created automatically by `task cluster:create`). Verify:
  ```bash
  kubectl get clusterissuer selfsigned-cluster-issuer
  ```

## Run

1. Apply the manifest:

   ```bash
   kubectl apply -f .
   ```

2. Wait for the certificate and MCP server to become ready:

   ```bash
   kubectl get certificate -n mcp -w
   kubectl get mcp -n mcp -w
   kubectl get pods -n mcp
   ```

3. Verify the MCP server is running by checking its health endpoint from within the cluster:

   ```bash
   kubectl run -it --rm curl --image=curlimages/curl --restart=Never -- \
     curl -k https://mcp-memory-server.mcp.svc.cluster.local:3000/health
   ```

4. Register the MCP server with a Gateway by adding it to the `spec.mcp.servers` list:

   ```yaml
   mcp:
     enabled: true
     servers:
       - name: memory-server
         url: "https://mcp-memory-server.mcp.svc.cluster.local:3000"
         healthCheck:
           enabled: true
           path: "/health"
           interval: "30s"
   ```

   See the [`gateway-complete`](../gateway-complete/) example for a full Gateway configuration that includes MCP server references.

## Cleanup

```bash
kubectl delete -f .
```
