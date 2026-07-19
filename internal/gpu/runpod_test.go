/*
Copyright 2026 Inference Gateway

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gpu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRunPodProvisionCreatesPod(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-key" {
			t.Errorf("missing/incorrect auth header: %q", got)
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/pods":
			_, _ = w.Write([]byte(`[]`)) // no existing pod -> triggers create
		case r.Method == http.MethodPost && r.URL.Path == "/pods":
			_ = json.NewDecoder(r.Body).Decode(&posted)
			_, _ = w.Write([]byte(`{"id":"abc123","desiredStatus":"RUNNING","ports":["8080/http"]}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := NewRunPodProvider("secret-key", srv.URL)
	alloc, err := p.Provision(context.Background(), ProvisionRequest{
		UID: "uid-1", Image: "img", GPUTypes: []string{"NVIDIA RTX A6000"}, Port: 8080,
		Command: []string{"--port", "8080"}, EndpointToken: "tok",
	})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if alloc.ID != "abc123" {
		t.Errorf("id = %q, want abc123", alloc.ID)
	}
	if alloc.URL != "https://abc123-8080.proxy.runpod.net" {
		t.Errorf("url = %q", alloc.URL)
	}
	if alloc.State != StateRunning {
		t.Errorf("state = %q, want Running", alloc.State)
	}
	if posted["name"] != "igw-gpu-uid-1" {
		t.Errorf("pod name = %v, want igw-gpu-uid-1", posted["name"])
	}
	// Per-allocation token is injected, provider key is not in the body.
	env, _ := posted["env"].(map[string]any)
	if env["API_KEY"] != "tok" {
		t.Errorf("env.API_KEY = %v, want tok", env["API_KEY"])
	}
	if strings.Contains(string(mustJSON(t, posted)), "secret-key") {
		t.Error("provider API key leaked into create body")
	}
}

func TestRunPodProvisionIsIdempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/pods":
			// existing pod tagged with our UID -> must be recovered, not recreated
			_, _ = w.Write([]byte(`[{"id":"existing","name":"igw-gpu-uid-1","desiredStatus":"RUNNING","ports":["8080/http"]}]`))
		case r.Method == http.MethodPost:
			t.Error("Provision must not create a second pod when one already exists")
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	p := NewRunPodProvider("k", srv.URL)
	alloc, err := p.Provision(context.Background(), ProvisionRequest{UID: "uid-1", Port: 8080})
	if err != nil {
		t.Fatalf("Provision: %v", err)
	}
	if alloc.ID != "existing" {
		t.Errorf("id = %q, want existing (recovered)", alloc.ID)
	}
}

func TestRunPodGetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := NewRunPodProvider("k", srv.URL)
	_, err := p.Get(context.Background(), "gone")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get = %v, want ErrNotFound", err)
	}
}

func TestRunPodDestroyTreatsNotFoundAsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("unexpected method %s", r.Method)
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	p := NewRunPodProvider("k", srv.URL)
	if err := p.Destroy(context.Background(), "gone"); err != nil {
		t.Fatalf("Destroy on 404 = %v, want nil", err)
	}
}

func TestRunPodGetBuildsURLFromPorts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":"xyz","desiredStatus":"RUNNING","ports":["22/tcp","9000/http"]}`))
	}))
	defer srv.Close()

	p := NewRunPodProvider("k", srv.URL)
	alloc, err := p.Get(context.Background(), "xyz")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if alloc.URL != "https://xyz-9000.proxy.runpod.net" {
		t.Errorf("url = %q, want https://xyz-9000.proxy.runpod.net", alloc.URL)
	}
}

func TestMapState(t *testing.T) {
	cases := map[string]AllocationState{
		"RUNNING":    StateRunning,
		"EXITED":     StateTerminated,
		"TERMINATED": StateTerminated,
		"FAILED":     StateFailed,
		"":           StatePending,
		"CREATED":    StatePending,
	}
	for in, want := range cases {
		if got := mapState(in); got != want {
			t.Errorf("mapState(%q) = %q, want %q", in, got, want)
		}
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return b
}
