/*
Copyright (c) 2025 Inference Gateway

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
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
