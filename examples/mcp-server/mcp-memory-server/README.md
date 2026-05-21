# mcp-memory-server

A minimal MCP server written in Go using [`metoro-io/mcp-golang`](https://github.com/metoro-io/mcp-golang) — the same library the Inference Gateway uses on its client side. It exposes a small in-process key/value store over Streamable HTTP at `POST /mcp` + `GET /mcp`:

| tool            | description                                              |
| --------------- | -------------------------------------------------------- |
| `memory_set`    | Store a string value under a key.                        |
| `memory_get`    | Retrieve the value stored under a key.                   |
| `memory_delete` | Remove a key from the store.                             |
| `memory_list`   | List all keys currently in the store (sorted).           |

> **Note:** the store is process-local and resets on pod restart. This example deliberately avoids shipping a registry of "trusted" MCP servers — building your own image and importing it into your cluster is the recommended path. For persistence, back the store with Redis or a PVC yourself.

## Build & import into the local k3d cluster

```sh
# from this directory
docker build -t mcp-memory-server:dev .

# import into the k3d cluster named "dev" (created by `task cluster:create`)
k3d image import mcp-memory-server:dev --cluster dev
```

Then apply the manifest one directory up:

```sh
kubectl apply -f ../mcp.yaml
```

## Run locally (without Kubernetes)

```sh
go run . --port 8080 --path /mcp
```
