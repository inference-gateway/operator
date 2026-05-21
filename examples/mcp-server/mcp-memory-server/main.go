package main

import (
	"flag"
	"fmt"
	"log"
	"sort"
	"sync"

	mcp "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
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
	Key   string `json:"key" jsonschema:"required,description=Key to store the value under."`
	Value string `json:"value" jsonschema:"required,description=String value to associate with the key."`
}

type MemoryGetArgs struct {
	Key string `json:"key" jsonschema:"required,description=Key to look up."`
}

type MemoryDeleteArgs struct {
	Key string `json:"key" jsonschema:"required,description=Key to remove."`
}

type MemoryListArgs struct{}

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	path := flag.String("path", "/mcp", "HTTP endpoint path")
	flag.Parse()

	transport := mcphttp.NewHTTPTransport(*path)
	transport.WithAddr(fmt.Sprintf(":%d", *port))

	server := mcp.NewServer(transport)
	kv := newStore()

	if err := server.RegisterTool(
		"memory_set",
		"Store a string value under the given key.",
		func(args MemorySetArgs) (*mcp.ToolResponse, error) {
			if args.Key == "" {
				return mcp.NewToolResponse(mcp.NewTextContent("key is required")), nil
			}
			kv.set(args.Key, args.Value)
			return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("stored %q", args.Key))), nil
		},
	); err != nil {
		log.Fatalf("register memory_set: %v", err)
	}

	if err := server.RegisterTool(
		"memory_get",
		"Retrieve the value previously stored under a key.",
		func(args MemoryGetArgs) (*mcp.ToolResponse, error) {
			if args.Key == "" {
				return mcp.NewToolResponse(mcp.NewTextContent("key is required")), nil
			}
			v, ok := kv.get(args.Key)
			if !ok {
				return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("key %q not found", args.Key))), nil
			}
			return mcp.NewToolResponse(mcp.NewTextContent(v)), nil
		},
	); err != nil {
		log.Fatalf("register memory_get: %v", err)
	}

	if err := server.RegisterTool(
		"memory_delete",
		"Remove a key from the store.",
		func(args MemoryDeleteArgs) (*mcp.ToolResponse, error) {
			if args.Key == "" {
				return mcp.NewToolResponse(mcp.NewTextContent("key is required")), nil
			}
			if !kv.delete(args.Key) {
				return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("key %q not found", args.Key))), nil
			}
			return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("deleted %q", args.Key))), nil
		},
	); err != nil {
		log.Fatalf("register memory_delete: %v", err)
	}

	if err := server.RegisterTool(
		"memory_list",
		"List all keys currently in the store (sorted).",
		func(_ MemoryListArgs) (*mcp.ToolResponse, error) {
			keys := kv.keys()
			if len(keys) == 0 {
				return mcp.NewToolResponse(mcp.NewTextContent("(empty)")), nil
			}
			out := ""
			for _, k := range keys {
				out += k + "\n"
			}
			return mcp.NewToolResponse(mcp.NewTextContent(out)), nil
		},
	); err != nil {
		log.Fatalf("register memory_list: %v", err)
	}

	log.Printf("mcp-memory-server listening on :%d%s", *port, *path)
	if err := server.Serve(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
