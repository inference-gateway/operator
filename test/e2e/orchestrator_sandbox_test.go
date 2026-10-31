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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	utils "github.com/inference-gateway/operator/test/utils"
)

// idleExitCLIImage is the first CLI image whose daemon idles out
// (INFER_DAEMON_IDLE_EXIT). Move to cli:latest once 0.239.0 is released.
const idleExitCLIImage = "ghcr.io/inference-gateway/cli:0.239.0-rc.9"

var _ = Describe("Orchestrator with runtime: Sandbox", Ordered, func() {
	const (
		orchestratorName = "e2e-sandboxed-orchestrator"
		sandboxTimeout   = 5 * time.Minute
		sandboxInterval  = 3 * time.Second
	)

	kubectl := func(args ...string) (string, error) {
		return utils.Run(exec.Command("kubectl", args...))
	}

	orchestratorField := func(jsonPath string) func(g Gomega) string {
		return func(g Gomega) string {
			out, err := kubectl("get", "orchestrator", orchestratorName, "-n", testNamespace, "-o", "jsonpath="+jsonPath)
			g.Expect(err).NotTo(HaveOccurred())
			return strings.TrimSpace(out)
		}
	}

	patchSandbox := func(sandbox string) {
		_, err := kubectl("patch", "orchestrator", orchestratorName, "-n", testNamespace, "--type", "merge",
			"-p", fmt.Sprintf(`{"spec":{"sandbox":%s}}`, sandbox))
		Expect(err).NotTo(HaveOccurred())
	}

	BeforeAll(func() {
		By("deploying the operator")
		if _, err := kubectl("create", "namespace", namespace); err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
			Expect(err).NotTo(HaveOccurred())
		}
		_, err := utils.Run(exec.Command("task", "install"))
		Expect(err).NotTo(HaveOccurred())
		_, err = utils.Run(exec.Command("task", "deploy", fmt.Sprintf("IMG=%s", projectImage)))
		Expect(err).NotTo(HaveOccurred())

		By("installing agent-sandbox and restarting the operator so it watches Sandboxes")
		_, err = utils.Run(exec.Command("task", "agent-sandbox:install"))
		Expect(err).NotTo(HaveOccurred())
		_, err = kubectl("rollout", "restart", "deployment/operator-inference-gateway", "-n", namespace)
		Expect(err).NotTo(HaveOccurred())
		_, err = kubectl("rollout", "status", "deployment/operator-inference-gateway", "-n", namespace, "--timeout=180s")
		Expect(err).NotTo(HaveOccurred())

		By("preparing the managed test namespace")
		if _, err := kubectl("create", "namespace", testNamespace); err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
			Expect(err).NotTo(HaveOccurred())
		}
		_, err = kubectl("label", "namespace", testNamespace, "inference-gateway.com/managed=true", "--overwrite")
		Expect(err).NotTo(HaveOccurred())
	})

	AfterAll(func() {
		_, _ = kubectl("delete", "orchestrator", orchestratorName, "-n", testNamespace, "--ignore-not-found=true")
		_, _ = kubectl("delete", "secret", orchestratorName+"-tls", orchestratorName+"-token", "-n", testNamespace, "--ignore-not-found=true")
	})

	It("runs the daemon in a Sandbox that suspends, resumes and expires", func() {
		By("creating an Orchestrator with runtime: Sandbox")
		_, err := utils.Run(exec.Command("sh", "-c", fmt.Sprintf(`cat <<'EOF' | kubectl apply -f -
apiVersion: core.inference-gateway.com/v1alpha1
kind: Orchestrator
metadata:
  name: %s
  namespace: %s
spec:
  image: ghcr.io/inference-gateway/cli:latest
  runtime: Sandbox
  sandbox:
    storage: 1Gi
  tools:
    enabled: true
    schedule: true
EOF`, orchestratorName, testNamespace)))
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the daemon's Sandbox to become ready")
		Eventually(orchestratorField("{.status.ready}"), sandboxTimeout, sandboxInterval).Should(Equal("true"))
		podName := orchestratorField("{.status.podName}")(Default)
		Expect(podName).NotTo(BeEmpty())
		_, err = kubectl("get", "sandbox", orchestratorName, "-n", testNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("writing to the workspace volume")
		_, err = kubectl("exec", podName, "-n", testNamespace, "--", "sh", "-c", "echo kept > /workspace/marker")
		Expect(err).NotTo(HaveOccurred())

		By("suspending the Sandbox, which removes its pod")
		patchSandbox(`{"operatingMode":"Suspended"}`)
		Eventually(orchestratorField(`{.status.conditions[?(@.type=="Ready")].reason}`), sandboxTimeout, sandboxInterval).
			Should(Equal("SandboxSuspended"))
		Eventually(func() error {
			_, err := kubectl("get", "pod", podName, "-n", testNamespace)
			return err
		}, sandboxTimeout, sandboxInterval).Should(MatchError(ContainSubstring("NotFound")))

		By("resuming the Sandbox on a fresh pod that still has the workspace")
		patchSandbox(`{"operatingMode":"Running"}`)
		Eventually(orchestratorField("{.status.ready}"), sandboxTimeout, sandboxInterval).Should(Equal("true"))
		marker, err := kubectl("exec", podName, "-n", testNamespace, "--", "cat", "/workspace/marker")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(marker)).To(Equal("kept"))

		By("deleting the Orchestrator once its Sandbox expires")
		patchSandbox(fmt.Sprintf(`{"shutdownTime":%q}`, time.Now().Add(10*time.Second).UTC().Format(time.RFC3339)))
		Eventually(func() error {
			_, err := kubectl("get", "orchestrator", orchestratorName, "-n", testNamespace)
			return err
		}, sandboxTimeout, sandboxInterval).Should(MatchError(ContainSubstring("NotFound")))
		Eventually(func() error {
			_, err := kubectl("get", "sandbox", orchestratorName, "-n", testNamespace)
			return err
		}, sandboxTimeout, sandboxInterval).Should(MatchError(ContainSubstring("NotFound")))
	})

	It("parks the Orchestrator when its daemon idles out, and resumes it on request", func() {
		By("creating the binding's certificate and token, so the daemon serves something it can idle out of")
		_, err := utils.Run(exec.Command("sh", "-c", fmt.Sprintf(`set -e
dir=$(mktemp -d)
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj /CN=%[1]s -keyout "$dir/tls.key" -out "$dir/tls.crt" 2>/dev/null
kubectl create secret tls %[1]s-tls -n %[2]s --cert="$dir/tls.crt" --key="$dir/tls.key" --dry-run=client -o yaml | kubectl apply -f -
kubectl create secret generic %[1]s-token -n %[2]s --from-literal=token=e2e --dry-run=client -o yaml | kubectl apply -f -
rm -rf "$dir"`, orchestratorName, testNamespace)))
		Expect(err).NotTo(HaveOccurred())

		By("creating a Sandbox Orchestrator whose daemon idles out after 20 seconds")
		_, err = utils.Run(exec.Command("sh", "-c", fmt.Sprintf(`cat <<'EOF' | kubectl apply -f -
apiVersion: core.inference-gateway.com/v1alpha1
kind: Orchestrator
metadata:
  name: %[1]s
  namespace: %[2]s
spec:
  image: %[3]s
  runtime: Sandbox
  sandbox:
    storage: 1Gi
  daemon:
    binding:
      tokenSecretRef:
        name: %[1]s-token
        key: token
      tls:
        secretName: %[1]s-tls
  env:
    - name: INFER_DAEMON_IDLE_EXIT
      value: 20s
EOF`, orchestratorName, testNamespace, idleExitCLIImage)))
		Expect(err).NotTo(HaveOccurred())
		Eventually(orchestratorField("{.status.ready}"), sandboxTimeout, sandboxInterval).Should(Equal("true"))

		By("waiting for the operator to park it once the daemon exited")
		Eventually(orchestratorField("{.spec.sandbox.operatingMode}"), sandboxTimeout, sandboxInterval).Should(Equal("Suspended"))
		Eventually(orchestratorField(`{.status.conditions[?(@.type=="Ready")].reason}`), sandboxTimeout, sandboxInterval).
			Should(Equal("SandboxSuspended"))
		annotations, err := kubectl("get", "sandbox", orchestratorName, "-n", testNamespace, "-o", "jsonpath={.metadata.annotations}")
		Expect(err).NotTo(HaveOccurred())
		Expect(annotations).To(ContainSubstring("inference-gateway.com/idle-since"))

		By("resuming it the way a client does")
		patchSandbox(`{"operatingMode":"Running"}`)
		Eventually(orchestratorField("{.status.ready}"), sandboxTimeout, sandboxInterval).Should(Equal("true"))
	})
})
