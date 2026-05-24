# MCP Server Example

Deploy a standalone **MCP memory server** - a minimal Go MCP that exposes a small in-process key/value store over Streamable HTTP. Once running, register its in-cluster URL under a Gateway's `spec.mcp.servers` so the Inference Gateway can proxy tool calls to it.

The MCP server itself lives in [`mcp-memory-server/`](mcp-memory-server/) and is built with [`metoro-io/mcp-golang`](https://github.com/metoro-io/mcp-golang) - the same library the Inference Gateway uses on its client side, eliminating any cross-implementation capability mismatches.

> **Why a local image instead of a public registry?** This project does **not** maintain a registry of "trusted" MCP servers - that's not a sustainable long-term solution. The recommended path is to build your own MCP image, push it to your cluster (or to a registry you trust), and reference it from an `MCP` resource. This example shows exactly that flow against a local k3d cluster.

## Prerequisites

- Local k3d cluster with the Inference Gateway operator installed:
  ```bash
  task cluster:create && task install && task deploy
  ```
- `docker` (to build the MCP image)
- `k3d` (to import the built image into the cluster)

## Run

1. Build the MCP server image and import it into the k3d cluster:

   ```bash
   cd mcp-memory-server
   docker build -t mcp-memory-server:dev .
   k3d image import mcp-memory-server:dev --cluster dev
   cd ..
   ```

   See [`mcp-memory-server/README.md`](mcp-memory-server/README.md) for the exposed tools and how to run the server locally without Kubernetes.

2. Apply the manifest:

   ```bash
   kubectl apply -f mcp.yaml
   ```

3. Wait for the MCP server to become ready:

   ```bash
   kubectl get mcp -n mcp -w
   kubectl get pods -n mcp
   ```

4. (Optional) Sanity-check the Streamable HTTP endpoint from inside the cluster - a `tools/list` JSON-RPC call should return all four tools (`memory_set`, `memory_get`, `memory_delete`, `memory_list`):

   ```bash
   kubectl run -it --rm curl --image=curlimages/curl --restart=Never -- \
     curl -s -X POST http://mcp-memory-server-service.mcp.svc.cluster.local:8080/mcp \
       -H 'Content-Type: application/json' \
       -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}'
   ```

5. Register the MCP server with a Gateway by adding it to `spec.mcp.servers`:

   ```yaml
   mcp:
     enabled: true
     servers:
       - name: memory-server
         url: "http://mcp-memory-server-service.mcp.svc.cluster.local:8080/mcp"
         healthCheck:
           enabled: true
           path: "/health"
           interval: "30s"
   ```

   See the [`gateway-complete`](../gateway-complete/) example for a full Gateway configuration that includes MCP server references, and the [`orchestrator`](../orchestrator/) example for a deployment that uses `mcp.serviceDiscovery` to pick up MCP CRs by label automatically.

## Cleanup

```bash
kubectl delete -f mcp.yaml
```

## Building your own MCP server

The flow shown here generalises to any MCP server you want to deploy:

1. Write your MCP server (Go, Python, TypeScript, …). For Go, the [`mcp-memory-server/main.go`](mcp-memory-server/main.go) source is a ~100-line template - copy it, swap the tools, and you're done.
2. Build a container image: `docker build -t <name>:<tag> .`
3. Make it reachable from your cluster - for local k3d: `k3d image import <name>:<tag> --cluster dev`; otherwise push to a registry your cluster can pull from.
4. Reference the image from an `MCP` resource (see [`mcp.yaml`](mcp.yaml)).
5. Register it with a Gateway via `spec.mcp.servers` (or via `spec.mcp.serviceDiscovery` for label-based auto-discovery).
