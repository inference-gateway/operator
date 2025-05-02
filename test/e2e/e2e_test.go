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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/inference-gateway/operator/test/utils"
)

// namespace where the project is deployed in
const namespace = "operator-system"

// serviceAccountName created for the project
const serviceAccountName = "operator-controller-manager"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "operator-controller-manager-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "operator-metrics-binding"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the restricted security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=restricted")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

		By("installing CRDs")
		cmd = exec.Command("task", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("task", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("cleaning up metrics ClusterRoleBinding")
		cmd := exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)

		By("cleaning up the curl pod for metrics")
		cmd = exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace, "--ignore-not-found=true")
		_, _ = utils.Run(cmd)

		By("undeploying the controller-manager")
		cmd = exec.Command("task", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("task", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching curl-metrics logs")
			cmd = exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
			metricsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Metrics logs:\n %s", metricsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get curl-metrics logs: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the controller-manager pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=controller-manager",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve controller-manager pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 controller pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("controller-manager"))

				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect controller-manager pod status")
			}
			Eventually(verifyControllerUp).Should(Succeed())
		})

		It("should ensure the metrics endpoint is serving metrics", func() {
			By("cleaning up any existing metrics ClusterRoleBinding")
			cmd := exec.Command("kubectl", "delete", "clusterrolebinding", metricsRoleBindingName, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)

			By("creating a ClusterRoleBinding for the service account to allow access to metrics")
			cmd = exec.Command("kubectl", "create", "clusterrolebinding", metricsRoleBindingName,
				"--clusterrole=operator-metrics-reader",
				fmt.Sprintf("--serviceaccount=%s:%s", namespace, serviceAccountName),
			)
			_, err := utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create ClusterRoleBinding")

			By("validating that the metrics service is available")
			cmd = exec.Command("kubectl", "get", "service", metricsServiceName, "-n", namespace)
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Metrics service should exist")

			By("getting the service account token")
			token, err := serviceAccountToken()
			Expect(err).NotTo(HaveOccurred())
			Expect(token).NotTo(BeEmpty())

			By("waiting for the metrics endpoint to be ready")
			verifyMetricsEndpointReady := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "endpoints", metricsServiceName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("8443"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the controller manager is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\tServing metrics server"),
					"Metrics server not yet started")
			}
			Eventually(verifyMetricsServerStarted).Should(Succeed())

			By("cleaning up any existing curl-metrics pod")
			cmd = exec.Command("kubectl", "delete", "pod", "curl-metrics", "-n", namespace, "--ignore-not-found=true")
			_, _ = utils.Run(cmd)
			time.Sleep(5 * time.Second)

			By("creating the curl-metrics pod to access the metrics endpoint")
			cmd = exec.Command("kubectl", "run", "curl-metrics", "--restart=Never",
				"--namespace", namespace,
				"--image=curlimages/curl:latest",
				"--overrides",
				fmt.Sprintf(`{
					"spec": {
						"containers": [{
							"name": "curl",
							"image": "curlimages/curl:latest",
							"command": ["/bin/sh", "-c"],
							"args": ["curl -v -k -H 'Authorization: Bearer %s' https://%s.%s.svc.cluster.local:8443/metrics"],
							"securityContext": {
								"allowPrivilegeEscalation": false,
								"capabilities": {
									"drop": ["ALL"]
								},
								"runAsNonRoot": true,
								"runAsUser": 1000,
								"seccompProfile": {
									"type": "RuntimeDefault"
								}
							}
						}],
						"serviceAccount": "%s"
					}
				}`, token, metricsServiceName, namespace, serviceAccountName))
			_, err = utils.Run(cmd)
			Expect(err).NotTo(HaveOccurred(), "Failed to create curl-metrics pod")

			By("waiting for the curl-metrics pod to complete.")
			verifyCurlUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get", "pods", "curl-metrics",
					"-o", "jsonpath={.status.phase}",
					"-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Succeeded"), "curl pod in wrong status")
			}
			Eventually(verifyCurlUp, 5*time.Minute).Should(Succeed())

			By("getting the metrics by checking curl-metrics logs")
			metricsOutput := getMetricsOutput()
			Expect(metricsOutput).To(ContainSubstring(
				"controller_runtime_reconcile_total",
			))
		})

		// +kubebuilder:scaffold:e2e-webhooks-checks

		Context("Gateway Controller", func() {
			const (
				gatewayName      = "e2e-test-gateway"
				gatewayNamespace = "default"
				configMapName    = "e2e-test-gateway-config"
				deploymentName   = "e2e-test-gateway"
				serviceName      = "e2e-test-gateway"
				timeout          = 2 * time.Minute
				interval         = time.Second
			)

			BeforeEach(func() {
				By("cleaning up any existing test gateway resources")
				cmd := exec.Command("kubectl", "delete", "gateway", gatewayName, "-n", gatewayNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(5 * time.Second)
			})

			AfterEach(func() {
				By("cleaning up test gateway resources")
				cmd := exec.Command("kubectl", "delete", "gateway", gatewayName, "-n", gatewayNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)
			})

			It("should create ConfigMap, Deployment, and Service when Gateway is created", func() {
				By("creating a test Gateway CR")
				gatewayYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: %s
  namespace: %s
spec:
  environment: production
  enableTelemetry: true
  enableAuth: true
  oidc:
    issuerUrl: "https://auth.example.com"
    clientId: "test-client"
    clientSecretRef:
      name: oidc-secret
      key: client-secret
  server:
    host: "0.0.0.0"
    port: "8080"
  providers:
    openai:
      url: "https://api.openai.com/v1"
      tokenRef:
        name: provider-keys
        key: openai-key
`, gatewayName, gatewayNamespace)

				gatewayFile := filepath.Join(os.TempDir(), "test-gateway.yaml")
				err := os.WriteFile(gatewayFile, []byte(gatewayYAML), 0644)
				Expect(err).NotTo(HaveOccurred(), "Failed to write Gateway YAML file")

				cmd := exec.Command("kubectl", "apply", "-f", gatewayFile)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to create Gateway CR")

				By("creating required secrets")

				oidcSecretYAML := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: oidc-secret
  namespace: %s
type: Opaque
stringData:
  client-secret: test-client-secret
`, gatewayNamespace)

				oidcSecretFile := filepath.Join(os.TempDir(), "oidc-secret.yaml")
				err = os.WriteFile(oidcSecretFile, []byte(oidcSecretYAML), 0644)
				Expect(err).NotTo(HaveOccurred(), "Failed to write OIDC secret YAML file")

				cmd = exec.Command("kubectl", "apply", "-f", oidcSecretFile)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to create OIDC secret")

				providerKeysYAML := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: provider-keys
  namespace: %s
type: Opaque
stringData:
  openai-key: test-openai-key
`, gatewayNamespace)

				providerKeysFile := filepath.Join(os.TempDir(), "provider-keys.yaml")
				err = os.WriteFile(providerKeysFile, []byte(providerKeysYAML), 0644)
				Expect(err).NotTo(HaveOccurred(), "Failed to write provider keys secret YAML file")

				cmd = exec.Command("kubectl", "apply", "-f", providerKeysFile)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to create provider keys secret")

				By("verifying the Gateway CR was created successfully")
				verifyGatewayCreated := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "gateway", gatewayName, "-n", gatewayNamespace, "-o", "jsonpath={.metadata.name}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get Gateway CR")
					g.Expect(output).To(Equal(gatewayName), "Gateway CR not found")
				}
				Eventually(verifyGatewayCreated, timeout, interval).Should(Succeed())

				By("verifying the ConfigMap was created with correct configuration")
				verifyConfigMap := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", gatewayNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "ConfigMap not found")

					cmd = exec.Command("kubectl", "get", "configmap", configMapName, "-n", gatewayNamespace, "-o", "jsonpath={.data['config\\.yaml']}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get ConfigMap data")

					g.Expect(output).To(ContainSubstring("environment: production"))
					g.Expect(output).To(ContainSubstring("enableTelemetry: true"))
					g.Expect(output).To(ContainSubstring("enableAuth: true"))
					g.Expect(output).To(ContainSubstring("issuerUrl: https://auth.example.com"))
					g.Expect(output).To(ContainSubstring("clientId: test-client"))
					g.Expect(output).To(ContainSubstring("url: https://api.openai.com/v1"))
				}
				Eventually(verifyConfigMap, timeout, interval).Should(Succeed())

				By("verifying the Deployment was created with the correct configuration")
				verifyDeployment := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Deployment not found")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace, "-o", "jsonpath={.spec.template.spec.volumes[?(@.name=='"+gatewayName+"-config-volume')].configMap.name}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get Deployment volumes")
					g.Expect(output).To(Equal(configMapName), "ConfigMap not mounted in Deployment")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace, "-o", "jsonpath={.spec.template.spec.containers[0].env}")
					output, err = utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get container env vars")
					g.Expect(output).To(ContainSubstring("OIDC_CLIENT_SECRET"))
					g.Expect(output).To(ContainSubstring("OPENAI_API_KEY"))
				}
				Eventually(verifyDeployment, timeout, interval).Should(Succeed())

				By("verifying the Service was created with the correct configuration")
				verifyService := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "service", serviceName, "-n", gatewayNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Service not found")

					cmd = exec.Command("kubectl", "get", "service", serviceName, "-n", gatewayNamespace, "-o", "jsonpath={.spec.selector.app}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get Service selector")
					g.Expect(output).To(Equal(gatewayName), "Service has incorrect selector")

					cmd = exec.Command("kubectl", "get", "service", serviceName, "-n", gatewayNamespace, "-o", "jsonpath={.spec.ports[0].port}")
					output, err = utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get Service port")
					g.Expect(output).To(Equal("8080"), "Service has incorrect port")
				}
				Eventually(verifyService, timeout, interval).Should(Succeed())

				By("verifying the controller metrics report reconciliation success")
				verifyMetrics := func(g Gomega) {
					metricsOutput := getMetricsOutput()
					g.Expect(metricsOutput).To(ContainSubstring(
						fmt.Sprintf(`controller_runtime_reconcile_total{controller="%s",result="success"}`,
							"gateway"),
					), "Metrics do not show successful Gateway reconciliation")
				}
				Eventually(verifyMetrics, timeout, interval).Should(Succeed())
			})

			It("should create a valid deployment with the inference-gateway image", func() {
				By("creating a test Gateway CR with specific requirements")
				gatewayYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: %s
  namespace: %s
spec:
  environment: production
  enableTelemetry: false
  enableAuth: false
  server:
    host: "0.0.0.0"
    port: "8080"
    readTimeout: "45s"
    writeTimeout: "45s"
    idleTimeout: "180s"
`, gatewayName, gatewayNamespace)

				gatewayFile := filepath.Join(os.TempDir(), "test-gateway-deployment.yaml")
				err := os.WriteFile(gatewayFile, []byte(gatewayYAML), 0644)
				Expect(err).NotTo(HaveOccurred(), "Failed to write Gateway YAML file")

				cmd := exec.Command("kubectl", "apply", "-f", gatewayFile)
				_, err = utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to create Gateway CR")

				By("verifying the Gateway CR was created successfully")
				verifyGatewayCreated := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "gateway", gatewayName, "-n", gatewayNamespace, "-o", "jsonpath={.metadata.name}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get Gateway CR")
					g.Expect(output).To(Equal(gatewayName), "Gateway CR not found")
				}
				Eventually(verifyGatewayCreated, timeout, interval).Should(Succeed())

				By("verifying the inference-gateway Deployment was created correctly")
				verifyDeploymentDetails := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace)
					_, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Deployment not found")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].image}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get container image")
					g.Expect(output).To(Equal("ghcr.io/inference-gateway/inference-gateway:latest"), "Incorrect image in deployment")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].ports[0].containerPort}")
					output, err = utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get container port")
					g.Expect(output).To(Equal("8080"), "Incorrect port in deployment")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].livenessProbe.httpGet.path}")
					output, err = utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get liveness probe")
					g.Expect(output).To(Equal("/health"), "Incorrect liveness probe path")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].readinessProbe.httpGet.path}")
					output, err = utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get readiness probe")
					g.Expect(output).To(Equal("/health"), "Incorrect readiness probe path")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].resources.requests.cpu}")
					output, err = utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get CPU request")
					g.Expect(output).NotTo(BeEmpty(), "CPU request not set")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace,
						"-o", "jsonpath={.spec.template.spec.containers[0].resources.limits.memory}")
					output, err = utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get memory limit")
					g.Expect(output).NotTo(BeEmpty(), "Memory limit not set")
				}
				Eventually(verifyDeploymentDetails, timeout, interval).Should(Succeed())

				By("verifying the ConfigMap exists and matches server timeout configuration")
				verifyConfigMapTimeouts := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "configmap", configMapName, "-n", gatewayNamespace,
						"-o", "jsonpath={.data['config\\.yaml']}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get ConfigMap data")

					g.Expect(output).To(ContainSubstring("readTimeout: 45s"))
					g.Expect(output).To(ContainSubstring("writeTimeout: 45s"))
					g.Expect(output).To(ContainSubstring("idleTimeout: 180s"))
				}
				Eventually(verifyConfigMapTimeouts, timeout, interval).Should(Succeed())

				By("verifying the deployment can start pods successfully")
				verifyDeploymentStatus := func(g Gomega) {
					cmd := exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace,
						"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")
					output, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get deployment status")
					g.Expect(output).To(Equal("True"), "Deployment is not available")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace,
						"-o", "jsonpath={.metadata.generation}")
					currentGen, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get current generation")

					cmd = exec.Command("kubectl", "get", "deployment", deploymentName, "-n", gatewayNamespace,
						"-o", "jsonpath={.status.observedGeneration}")
					observedGen, err := utils.Run(cmd)
					g.Expect(err).NotTo(HaveOccurred(), "Failed to get observed generation")

					g.Expect(currentGen).To(Equal(observedGen), "Deployment not fully reconciled")
				}
				Eventually(verifyDeploymentStatus, timeout, interval).Should(Succeed())
			})
		})
	})
})

// serviceAccountToken returns a token for the specified service account in the given namespace.
// It uses the Kubernetes TokenRequest API to generate a token by directly sending a request
// and parsing the resulting token from the API response.
func serviceAccountToken() (string, error) {
	const tokenRequestRawString = `{
		"apiVersion": "authentication.k8s.io/v1",
		"kind": "TokenRequest"
	}`

	secretName := fmt.Sprintf("%s-token-request", serviceAccountName)
	tokenRequestFile := filepath.Join("/tmp", secretName)
	err := os.WriteFile(tokenRequestFile, []byte(tokenRequestRawString), os.FileMode(0o644))
	if err != nil {
		return "", err
	}

	var out string
	verifyTokenCreation := func(g Gomega) {
		cmd := exec.Command("kubectl", "create", "--raw", fmt.Sprintf(
			"/api/v1/namespaces/%s/serviceaccounts/%s/token",
			namespace,
			serviceAccountName,
		), "-f", tokenRequestFile)

		output, err := cmd.CombinedOutput()
		g.Expect(err).NotTo(HaveOccurred())

		var token tokenRequest
		err = json.Unmarshal(output, &token)
		g.Expect(err).NotTo(HaveOccurred())

		out = token.Status.Token
	}
	Eventually(verifyTokenCreation).Should(Succeed())

	return out, err
}

// getMetricsOutput retrieves and returns the logs from the curl pod used to access the metrics endpoint.
func getMetricsOutput() string {
	By("getting the curl-metrics logs")
	cmd := exec.Command("kubectl", "logs", "curl-metrics", "-n", namespace)
	metricsOutput, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to retrieve logs from curl pod")
	Expect(metricsOutput).To(ContainSubstring("< HTTP/1.1 200 OK"))
	return metricsOutput
}

// tokenRequest is a simplified representation of the Kubernetes TokenRequest API response,
// containing only the token field that we need to extract.
type tokenRequest struct {
	Status struct {
		Token string `json:"token"`
	} `json:"status"`
}
