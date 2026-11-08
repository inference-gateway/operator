package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// store is a minimal thread-safe in-memory key/value store. Process-local on
// purpose - this example deliberately avoids a registry of "trusted" MCP
// servers. If you want persistence, back it with Redis or a PVC yourself.
type store struct {
	mu   sync.RWMutex
	data map[string]string
}

func newStore() *store {
	return &store{data: make(map[string]string)}
}

func (s *store) set(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

func (s *store) get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

func (s *store) delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[key]; !ok {
		return false
	}
	delete(s.data, key)
	return true
}

func (s *store) keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0, len(s.data))
	for k := range s.data {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type MemorySetArgs struct {
	Key   string `json:"key" jsonschema:"Key to store the value under."`
	Value string `json:"value" jsonschema:"String value to associate with the key."`
}

type MemoryKeyArgs struct {
	Key string `json:"key" jsonschema:"Key to look up or remove."`
}

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	path := flag.String("path", "/mcp", "HTTP endpoint path")
	flag.Parse()

	kv := newStore()
	server := mcp.NewServer(&mcp.Implementation{Name: "mcp-memory-server", Version: "1.0.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{Name: "memory_set", Description: "Store a string value under the given key."},
		func(_ context.Context, _ *mcp.CallToolRequest, args MemorySetArgs) (*mcp.CallToolResult, any, error) {
			kv.set(args.Key, args.Value)
			return text("stored %q", args.Key)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "memory_get", Description: "Retrieve the value previously stored under a key."},
		func(_ context.Context, _ *mcp.CallToolRequest, args MemoryKeyArgs) (*mcp.CallToolResult, any, error) {
			v, ok := kv.get(args.Key)
			if !ok {
				return text("key %q not found", args.Key)
			}
			return text("%s", v)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "memory_delete", Description: "Remove a key from the store."},
		func(_ context.Context, _ *mcp.CallToolRequest, args MemoryKeyArgs) (*mcp.CallToolResult, any, error) {
			if !kv.delete(args.Key) {
				return text("key %q not found", args.Key)
			}
			return text("deleted %q", args.Key)
		})

	mcp.AddTool(server, &mcp.Tool{Name: "memory_list", Description: "List all keys currently in the store (sorted)."},
		func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
			keys := kv.keys()
			if len(keys) == 0 {
				return text("(empty)")
			}
			return text("%s\n", strings.Join(keys, "\n"))
		})

	mux := http.NewServeMux()
	mux.Handle(*path, mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true}))

	log.Printf("mcp-memory-server listening on :%d%s", *port, *path)
	srv := &http.Server{Addr: fmt.Sprintf(":%d", *port), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func text(format string, args ...any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}}}, nil, nil
}
