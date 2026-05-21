# mcp-server-time

A minimal MCP server written in Go using [`metoro-io/mcp-golang`](https://github.com/metoro-io/mcp-golang) - the same library the inference-gateway uses on its client side. Built for end-to-end testing of MCP service discovery in this operator without cross-implementation capability mismatches.

Exposes two tools over Streamable HTTP at `POST /mcp` + `GET /mcp`:

| tool           | description                                              |
| -------------- | -------------------------------------------------------- |
| `get_time`     | Current time in a given IANA timezone (default UTC).     |
| `convert_time` | Convert an RFC3339 timestamp between IANA timezones.     |

## Build & import into the local k3d cluster

```sh
# from this directory
docker build -t mcp-server-time:dev .

# import into the k3d cluster named "dev" (see task cluster:create)
k3d image import mcp-server-time:dev --cluster dev
```

Then apply the manifests one directory up:

```sh
kubectl apply -f ../06-mcps.yaml
```

The Orchestrator and Gateway in this example are configured with `mcp.serviceDiscovery` selectors that match `mcp-group: group1`, so both will discover this server automatically once it's running.

## Run locally (without Kubernetes)

```sh
go run . --port 8080 --path /mcp
```
