package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	mcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

type GetTimeArgs struct {
	Timezone string `json:"timezone,omitempty" jsonschema:"IANA timezone name (e.g. America/New_York). Defaults to UTC."`
	Format   string `json:"format,omitempty" jsonschema:"Time format: rfc3339, unix, or a Go time layout. Defaults to rfc3339."`
}

type ConvertTimeArgs struct {
	Time         string `json:"time" jsonschema:"RFC3339 timestamp to convert (e.g. 2026-05-21T10:00:00Z)."`
	FromTimezone string `json:"from_timezone,omitempty" jsonschema:"Source IANA timezone. Defaults to the timestamp's own offset."`
	ToTimezone   string `json:"to_timezone" jsonschema:"Target IANA timezone (e.g. Europe/Berlin)."`
}

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	path := flag.String("path", "/mcp", "HTTP endpoint path")
	flag.Parse()

	server := mcp.NewServer(&mcp.Implementation{Name: "mcp-server-time", Version: "1.0.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "get_time", Description: "Returns the current time in a given IANA timezone (default UTC)."}, handleGetTime)
	mcp.AddTool(server, &mcp.Tool{Name: "convert_time", Description: "Converts an RFC3339 timestamp between IANA timezones."}, handleConvertTime)

	mux := http.NewServeMux()
	mux.Handle(*path, mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true}))

	log.Printf("mcp-server-time listening on :%d%s", *port, *path)
	srv := &http.Server{Addr: fmt.Sprintf(":%d", *port), Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func text(format string, args ...any) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}}}, nil, nil
}

func handleGetTime(_ context.Context, _ *mcp.CallToolRequest, args GetTimeArgs) (*mcp.CallToolResult, any, error) {
	tz := args.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return text("invalid timezone %q: %v", tz, err)
	}

	now := time.Now().In(loc)
	switch args.Format {
	case "", "rfc3339":
		return text("%s (%s)", now.Format(time.RFC3339), tz)
	case "unix":
		return text("%d (%s)", now.Unix(), tz)
	default:
		return text("%s (%s)", now.Format(args.Format), tz)
	}
}

func handleConvertTime(_ context.Context, _ *mcp.CallToolRequest, args ConvertTimeArgs) (*mcp.CallToolResult, any, error) {
	t, err := time.Parse(time.RFC3339, args.Time)
	if err != nil {
		return text("invalid time %q: %v", args.Time, err)
	}

	if args.FromTimezone != "" {
		fromLoc, err := time.LoadLocation(args.FromTimezone)
		if err != nil {
			return text("invalid from_timezone %q: %v", args.FromTimezone, err)
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), fromLoc)
	}

	toLoc, err := time.LoadLocation(args.ToTimezone)
	if err != nil {
		return text("invalid to_timezone %q: %v", args.ToTimezone, err)
	}
	return text("%s -> %s (%s)", args.Time, t.In(toLoc).Format(time.RFC3339), args.ToTimezone)
}
