package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	mcp "github.com/metoro-io/mcp-golang"
	mcphttp "github.com/metoro-io/mcp-golang/transport/http"
)

type GetTimeArgs struct {
	Timezone string `json:"timezone" jsonschema:"description=IANA timezone name (e.g. America/New_York). Defaults to UTC."`
	Format   string `json:"format" jsonschema:"description=Time format: rfc3339, unix, or a Go time layout. Defaults to rfc3339."`
}

type ConvertTimeArgs struct {
	Time         string `json:"time" jsonschema:"required,description=RFC3339 timestamp to convert (e.g. 2026-05-21T10:00:00Z)."`
	FromTimezone string `json:"from_timezone" jsonschema:"description=Source IANA timezone. Defaults to the timestamp's own offset."`
	ToTimezone   string `json:"to_timezone" jsonschema:"required,description=Target IANA timezone (e.g. Europe/Berlin)."`
}

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	path := flag.String("path", "/mcp", "HTTP endpoint path")
	flag.Parse()

	transport := mcphttp.NewHTTPTransport(*path)
	transport.WithAddr(fmt.Sprintf(":%d", *port))

	server := mcp.NewServer(transport)

	if err := server.RegisterTool(
		"get_time",
		"Returns the current time in a given IANA timezone (default UTC).",
		handleGetTime,
	); err != nil {
		log.Fatalf("register get_time: %v", err)
	}

	if err := server.RegisterTool(
		"convert_time",
		"Converts an RFC3339 timestamp between IANA timezones.",
		handleConvertTime,
	); err != nil {
		log.Fatalf("register convert_time: %v", err)
	}

	log.Printf("mcp-server-time listening on :%d%s", *port, *path)
	if err := server.Serve(); err != nil {
		log.Fatalf("serve: %v", err)
	}
}

func handleGetTime(args GetTimeArgs) (*mcp.ToolResponse, error) {
	tz := args.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("invalid timezone %q: %v", tz, err))), nil
	}

	now := time.Now().In(loc)

	format := args.Format
	if format == "" {
		format = "rfc3339"
	}
	var out string
	switch format {
	case "rfc3339":
		out = now.Format(time.RFC3339)
	case "unix":
		out = fmt.Sprintf("%d", now.Unix())
	default:
		out = now.Format(format)
	}

	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("%s (%s)", out, tz))), nil
}

func handleConvertTime(args ConvertTimeArgs) (*mcp.ToolResponse, error) {
	if args.Time == "" || args.ToTimezone == "" {
		return mcp.NewToolResponse(mcp.NewTextContent("time and to_timezone are required")), nil
	}

	t, err := time.Parse(time.RFC3339, args.Time)
	if err != nil {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("invalid time %q: %v", args.Time, err))), nil
	}

	if args.FromTimezone != "" {
		fromLoc, err := time.LoadLocation(args.FromTimezone)
		if err != nil {
			return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("invalid from_timezone %q: %v", args.FromTimezone, err))), nil
		}
		t = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), fromLoc)
	}

	toLoc, err := time.LoadLocation(args.ToTimezone)
	if err != nil {
		return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("invalid to_timezone %q: %v", args.ToTimezone, err))), nil
	}

	converted := t.In(toLoc).Format(time.RFC3339)
	return mcp.NewToolResponse(mcp.NewTextContent(fmt.Sprintf("%s -> %s (%s)", args.Time, converted, args.ToTimezone))), nil
}
