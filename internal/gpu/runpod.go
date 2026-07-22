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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// defaultRunPodBaseURL is the RunPod REST API v1 base URL.
const defaultRunPodBaseURL = "https://rest.runpod.io/v1"

// runPodProvider implements Provider against the RunPod REST API using only the
// standard library. The external pod is named "igw-gpu-<uid>" so reconciliation
// can recover it by the owning object's UID without a duplicate create.
type runPodProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

// NewRunPodProvider builds a RunPod driver. baseURL is overridable for testing;
// empty uses the public API.
func NewRunPodProvider(apiKey, baseURL string) Provider {
	if baseURL == "" {
		baseURL = defaultRunPodBaseURL
	}
	return &runPodProvider{
		apiKey:  apiKey,
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// runpodPod is the subset of the RunPod pod object we consume.
type runpodPod struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	DesiredStatus string   `json:"desiredStatus"`
	Ports         []string `json:"ports"`
}

func podName(uid string) string { return "igw-gpu-" + uid }

func (p *runPodProvider) Provision(ctx context.Context, req ProvisionRequest) (Allocation, error) {
	if existing, err := p.findByName(ctx, podName(req.UID)); err != nil {
		return Allocation{}, err
	} else if existing != nil {
		return p.toAllocation(*existing, req.Port), nil
	}

	ports := ""
	if req.Port > 0 {
		ports = fmt.Sprintf("%d/http", req.Port)
	}
	body := map[string]any{
		"name":              podName(req.UID),
		"imageName":         req.Image,
		"gpuTypeIds":        req.GPUTypes,
		"gpuCount":          1,
		"ports":             []string{ports},
		"containerDiskInGb": 20,
	}
	if len(req.Command) > 0 {
		body["dockerStartCmd"] = req.Command
	}
	if req.EndpointToken != "" {
		body["env"] = map[string]string{"API_KEY": req.EndpointToken}
	}

	var pod runpodPod
	status, err := p.do(ctx, http.MethodPost, "/pods", body, &pod)
	if err != nil {
		return Allocation{}, err
	}
	if status < 200 || status >= 300 {
		return Allocation{}, fmt.Errorf("runpod: create pod returned status %d", status)
	}
	return p.toAllocation(pod, req.Port), nil
}

func (p *runPodProvider) Get(ctx context.Context, allocationID string) (Allocation, error) {
	var pod runpodPod
	status, err := p.do(ctx, http.MethodGet, "/pods/"+allocationID, nil, &pod)
	if err != nil {
		return Allocation{}, err
	}
	if status == http.StatusNotFound {
		return Allocation{}, ErrNotFound
	}
	if status < 200 || status >= 300 {
		return Allocation{}, fmt.Errorf("runpod: get pod returned status %d", status)
	}
	return p.toAllocation(pod, 0), nil
}

func (p *runPodProvider) Destroy(ctx context.Context, allocationID string) error {
	status, err := p.do(ctx, http.MethodDelete, "/pods/"+allocationID, nil, nil)
	if err != nil {
		return err
	}
	if status == http.StatusNotFound {
		return nil
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("runpod: delete pod returned status %d", status)
	}
	return nil
}

// findByName returns the first pod matching name, or nil if none exist.
func (p *runPodProvider) findByName(ctx context.Context, name string) (*runpodPod, error) {
	var pods []runpodPod
	status, err := p.do(ctx, http.MethodGet, "/pods", nil, &pods)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("runpod: list pods returned status %d", status)
	}
	for i := range pods {
		if pods[i].Name == name {
			return &pods[i], nil
		}
	}
	return nil, nil
}

// toAllocation maps a RunPod pod to a provider Allocation. fallbackPort is used
// for the proxy URL when the pod does not echo an http port (e.g. right after
// create); it is ignored when the pod reports its ports.
func (p *runPodProvider) toAllocation(pod runpodPod, fallbackPort int32) Allocation {
	port := httpPort(pod.Ports)
	if port == 0 {
		port = fallbackPort
	}
	url := ""
	if pod.ID != "" && port > 0 {
		url = fmt.Sprintf("https://%s-%d.proxy.runpod.net", pod.ID, port)
	}
	return Allocation{ID: pod.ID, URL: url, State: mapState(pod.DesiredStatus)}
}

// httpPort extracts the container port exposed as http from RunPod's
// ["8080/http", "22/tcp"] port list, or 0 if none.
func httpPort(ports []string) int32 {
	for _, entry := range ports {
		p, proto, ok := strings.Cut(entry, "/")
		if !ok || proto != "http" {
			continue
		}
		if n, err := strconv.ParseUint(p, 10, 16); err == nil {
			return int32(n)
		}
	}
	return 0
}

func mapState(desiredStatus string) AllocationState {
	switch strings.ToUpper(desiredStatus) {
	case "RUNNING":
		return StateRunning
	case "EXITED", "TERMINATED", "DEAD":
		return StateTerminated
	case "FAILED":
		return StateFailed
	default:
		return StatePending
	}
}

// do issues a JSON request and decodes the response into out (when non-nil and
// the body is non-empty). It returns the HTTP status code.
func (p *runPodProvider) do(ctx context.Context, method, path string, body, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, p.baseURL+path, reader)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	if out == nil || resp.StatusCode == http.StatusNoContent {
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.StatusCode, nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return resp.StatusCode, nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return resp.StatusCode, fmt.Errorf("runpod: decode response: %w", err)
	}
	return resp.StatusCode, nil
}
