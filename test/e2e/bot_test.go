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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	utils "github.com/inference-gateway/operator/test/utils"
)

var _ = Describe("Bot Controller", Ordered, func() {
	const (
		botName      = "e2e-test-bot"
		botNamespace = testNamespace
		secretName   = "e2e-test-bot-credentials"
		botTimeout   = 60 * time.Second
		botInterval  = 2 * time.Second
	)

	BeforeAll(func() {
		By("ensuring CRDs are installed")
		cmd := exec.Command("task", "install")
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed to install CRDs")

		By("ensuring the test namespace exists")
		cmd = exec.Command("kubectl", "create", "namespace", botNamespace)
		if _, err := utils.Run(cmd); err != nil && !strings.Contains(err.Error(), "AlreadyExists") {
			ExpectWithOffset(1, err).NotTo(HaveOccurred(), "failed to create namespace")
		}

		By("labeling the namespace as managed")
		cmd = exec.Command("kubectl", "label", "namespace", botNamespace,
			"inference-gateway.com/managed=true", "--overwrite")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "failed to label namespace")
	})

	AfterEach(func() {
		By("cleaning up the bot")
		cmd := exec.Command("kubectl", "delete", "bot", botName, "-n", botNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)

		By("cleaning up the bot credentials secret")
		cmd = exec.Command("kubectl", "delete", "secret", secretName, "-n", botNamespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)
	})

	It("should reconcile a singleton Recreate Deployment running channels-manager", func() {
		By("creating the bot credentials secret")
		secretYAML := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
type: Opaque
stringData:
  token: e2e-fake-telegram-token
  allowedUsers: "1,2,3"
`, secretName, botNamespace)
		secretFile := filepath.Join(os.TempDir(), "bot-secret.yaml")
		Expect(os.WriteFile(secretFile, []byte(secretYAML), 0o644)).To(Succeed())
		_, err := utils.Run(exec.Command("kubectl", "apply", "-f", secretFile))
		Expect(err).NotTo(HaveOccurred(), "failed to create bot credentials secret")

		By("creating the Bot CR")
		botYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Bot
metadata:
  name: %s
  namespace: %s
spec:
  image: ghcr.io/inference-gateway/cli:latest
  channels:
    telegram:
      enabled: true
      tokenSecretRef:
        name: %s
        key: token
      allowedUsersSecretRef:
        name: %s
        key: allowedUsers
  gateway:
    url: http://inference-gateway:8080
  agent:
    model: openai/gpt-test
`, botName, botNamespace, secretName, secretName)

		botFile := filepath.Join(os.TempDir(), "test-bot.yaml")
		Expect(os.WriteFile(botFile, []byte(botYAML), 0o644)).To(Succeed())
		_, err = utils.Run(exec.Command("kubectl", "apply", "-f", botFile))
		Expect(err).NotTo(HaveOccurred(), "failed to apply Bot CR")

		By("verifying the Deployment is created with replicas=1 and Recreate strategy")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "deployment", botName,
				"-n", botNamespace, "-o", "jsonpath={.spec.replicas}"))
			g.Expect(err).NotTo(HaveOccurred(), "deployment not found")
			g.Expect(strings.TrimSpace(out)).To(Equal("1"), "expected replicas=1")

			out, err = utils.Run(exec.Command("kubectl", "get", "deployment", botName,
				"-n", botNamespace, "-o", "jsonpath={.spec.strategy.type}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(strings.TrimSpace(out)).To(Equal("Recreate"), "expected Recreate strategy")
		}, botTimeout, botInterval).Should(Succeed())

		By("verifying the container command is `infer channels-manager`")
		Eventually(func(g Gomega) {
			cmdOut, err := utils.Run(exec.Command("kubectl", "get", "deployment", botName,
				"-n", botNamespace, "-o", "jsonpath={.spec.template.spec.containers[0].command}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cmdOut).To(ContainSubstring("infer"))

			argsOut, err := utils.Run(exec.Command("kubectl", "get", "deployment", botName,
				"-n", botNamespace, "-o", "jsonpath={.spec.template.spec.containers[0].args}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(argsOut).To(ContainSubstring("channels-manager"))
		}, botTimeout, botInterval).Should(Succeed())

		By("verifying the env vars include the Telegram token via secretKeyRef")
		Eventually(func(g Gomega) {
			out, err := utils.Run(exec.Command("kubectl", "get", "deployment", botName,
				"-n", botNamespace, "-o", "jsonpath={.spec.template.spec.containers[0].env}"))
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(ContainSubstring("INFER_CHANNELS_TELEGRAM_BOT_TOKEN"))
			g.Expect(out).To(ContainSubstring("INFER_CHANNELS_ENABLED"))
			g.Expect(out).To(ContainSubstring("INFER_GATEWAY_URL"))
		}, botTimeout, botInterval).Should(Succeed())
	})
})
