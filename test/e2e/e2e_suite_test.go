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

package e2e

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/inference-gateway/operator/test/utils"
)

var (
	// projectImage is the name of the image which will be build and loaded
	// with the code source changes to be tested.
	projectImage = "ghcr.io/inference-gateway/operator:latest"

	// Flag to track if we created the cluster as part of the test
	createdCluster = false
)

// TestE2E runs the end-to-end (e2e) test suite for the project. These tests execute in an isolated,
// temporary environment to validate project changes with the purposed to be used in CI jobs.
// The setup requires k3d, builds the Manager Docker image locally, and installs CertManager.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting operator integration test suite\n")
	RunSpecs(t, "e2e suite")
}

var _ = BeforeSuite(func() {
	By("ensuring k3d cluster exists")
	cmd := exec.Command("k3d", "cluster", "list")
	output, err := utils.Run(cmd)
	clusterExists := err == nil && strings.Contains(output, "k3d-dev")

	if !clusterExists {
		By("creating k3d cluster")
		cmd := exec.Command("task", "cluster:create")
		_, err := utils.Run(cmd)
		ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to create k3d cluster")
		createdCluster = true

		time.Sleep(5 * time.Second)
	}

	By("deploying the manager(Operator) to the cluster")
	cmd = exec.Command("task", "deploy", fmt.Sprintf("IMG=%s", projectImage))
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to deploy the manager(Operator)")

	By("loading the manager(Operator) image to k3d")
	err = utils.LoadImageToK3dClusterWithName(projectImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager(Operator) image into k3d")

	_, _ = fmt.Fprintf(GinkgoWriter, "Using image: %s\n", projectImage)
})

var _ = AfterSuite(func() {
	if createdCluster {
		_, _ = fmt.Fprintf(GinkgoWriter, "Cleaning up k3d cluster created for testing...\n")
		cmd := exec.Command("k3d", "cluster", "delete", "k3d-dev")
		if _, err := utils.Run(cmd); err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "WARNING: Failed to delete k3d cluster: %v\n", err)
		}
	}
})
