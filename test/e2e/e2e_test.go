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
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"

	utils "github.com/inference-gateway/operator/test/utils"
)

// namespace where the project is deployed in
const namespace = "inference-gateway-system"

// serviceAccountName created for the project
const serviceAccountName = "operator-inference-gateway"

// metricsServiceName is the name of the metrics service of the project
const metricsServiceName = "operator-inference-gateway-metrics-service"

// metricsRoleBindingName is the name of the RBAC that will be created to allow get the metrics data
const metricsRoleBindingName = "operator-inference-gateway-metrics-binding"

var _ = Describe("Operator", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// enforce the restricted security policy to the namespace, installing CRDs,
	// and deploying the controller.
	BeforeAll(func() {
		By("creating operator namespace")
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

		By("deploying the inference-gateway operator")
		cmd = exec.Command("task", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the inference-gateway operator")
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

		By("undeploying the inference-gateway operator")
		cmd = exec.Command("task", "undeploy")
		_, _ = utils.Run(cmd)

		By("uninstalling CRDs")
		cmd = exec.Command("task", "uninstall")
		_, _ = utils.Run(cmd)

		By("removing operator namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching operator pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-c", "operator", "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Operator logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Operator logs: %s", err)
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

	Context("Operator", func() {
		It("should run successfully", func() {
			By("validating that the inference-gateway-operator pod is running as expected")
			verifyControllerUp := func(g Gomega) {
				cmd := exec.Command("kubectl", "get",
					"pods", "-l", "control-plane=inference-gateway-operator",
					"-o", "go-template={{ range .items }}"+
						"{{ if not .metadata.deletionTimestamp }}"+
						"{{ .metadata.name }}"+
						"{{ \"\\n\" }}{{ end }}{{ end }}",
					"-n", namespace,
				)

				podOutput, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve inference-gateway-operator pod information")
				podNames := utils.GetNonEmptyLines(podOutput)
				g.Expect(podNames).To(HaveLen(1), "expected 1 operator pod running")
				controllerPodName = podNames[0]
				g.Expect(controllerPodName).To(ContainSubstring("operator-inference-gateway"))

				cmd = exec.Command("kubectl", "get",
					"pods", controllerPodName, "-o", "jsonpath={.status.phase}",
					"-n", namespace,
				)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(Equal("Running"), "Incorrect operator-inference-gateway pod status")
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
				g.Expect(output).To(ContainSubstring("9090"), "Metrics endpoint is not ready")
			}
			Eventually(verifyMetricsEndpointReady).Should(Succeed())

			By("verifying that the operator is serving the metrics server")
			verifyMetricsServerStarted := func(g Gomega) {
				cmd := exec.Command("kubectl", "logs", controllerPodName, "-c", "operator", "-n", namespace)
				output, err := utils.Run(cmd)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(output).To(ContainSubstring("controller-runtime.metrics\",\"msg\":\"Serving metrics server"),
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
							"args": ["curl -v -H 'Authorization: Bearer %s' http://%s.%s.svc.cluster.local:9090/metrics"],
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
				By("ensuring the test namespace has the required label")
				cmd := exec.Command("kubectl", "label", "namespace", gatewayNamespace, "inference-gateway.com/managed=true", "--overwrite")
				_, err := utils.Run(cmd)
				Expect(err).NotTo(HaveOccurred(), "Failed to label namespace")

				By("cleaning up any existing test gateway resources")
				cmd = exec.Command("kubectl", "delete", "gateway", gatewayName, "-n", gatewayNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				time.Sleep(5 * time.Second)
			})

			AfterEach(func() {
				By("cleaning up test gateway resources")
				cmd := exec.Command("kubectl", "delete", "gateway", gatewayName, "-n", gatewayNamespace, "--ignore-not-found=true")
				_, _ = utils.Run(cmd)

				By("removing the label from the test namespace")
				cmd = exec.Command("kubectl", "label", "namespace", gatewayNamespace, "inference-gateway.com/managed-", "--ignore-not-found=true")
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
  telemetry:
    enabled: true
  auth:
    enabled: true
    oidc:
      issuerUrl: "https://auth.example.com"
      clientId: "test-client"
      clientSecretRef:
        name: oidc-secret
        key: client-secret
  server:
    host: "0.0.0.0"
    port: 8080
  providers:
    - name: openai
      type: openai
      config:
        baseUrl: "https://api.openai.com/v1"
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
					g.Expect(output).To(ContainSubstring("enabled: true"))
					g.Expect(output).To(ContainSubstring("issuerUrl: https://auth.example.com"))
					g.Expect(output).To(ContainSubstring("clientId: test-client"))
					g.Expect(output).To(ContainSubstring("baseUrl: https://api.openai.com/v1"))
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
  telemetry:
    enabled: false
  auth:
    enabled: false
  server:
    host: "0.0.0.0"
    port: 8080
    timeouts:
      read: "45s"
      write: "45s"
      idle: "180s"
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

					g.Expect(output).To(ContainSubstring("read: 45s"))
					g.Expect(output).To(ContainSubstring("write: 45s"))
					g.Expect(output).To(ContainSubstring("idle: 180s"))
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

			Context("Horizontal Pod Autoscaler (HPA) Integration", func() {
				hpaGatewayName := "test-hpa-gateway"

				BeforeEach(func() {
					By("ensuring the test namespace has the required label for HPA tests")
					cmd := exec.Command("kubectl", "label", "namespace", gatewayNamespace, "inference-gateway.com/managed=true", "--overwrite")
					_, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred(), "Failed to label namespace")

					By("cleaning up any existing HPA test gateway resources")
					cmd = exec.Command("kubectl", "delete", "gateway", hpaGatewayName, "-n", gatewayNamespace, "--ignore-not-found=true")
					_, _ = utils.Run(cmd)

					time.Sleep(5 * time.Second)
				})

				AfterEach(func() {
					By("cleaning up HPA test gateway resources")
					cmd := exec.Command("kubectl", "delete", "gateway", hpaGatewayName, "-n", gatewayNamespace, "--ignore-not-found=true")
					_, _ = utils.Run(cmd)

					By("removing the label from the test namespace")
					cmd = exec.Command("kubectl", "label", "namespace", gatewayNamespace, "inference-gateway.com/managed-", "--ignore-not-found=true")
					_, _ = utils.Run(cmd)
				})

				It("should create and manage HPA when enabled in Gateway spec", func() {
					By("creating a Gateway with HPA enabled")
					hpaGatewayYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: %s
  namespace: %s
spec:
  environment: development
  replicas: 2
  hpa:
    enabled: true
    minReplicas: 1
    maxReplicas: 5
    targetCPUUtilizationPercentage: 70
    targetMemoryUtilizationPercentage: 80
    scaleDownStabilizationWindowSeconds: 300
    scaleUpStabilizationWindowSeconds: 60
  telemetry:
    enabled: true
    metrics:
      enabled: true
      port: 9464
  providers:
    - name: openai
      type: openai
      config:
        baseUrl: "https://api.openai.com/v1"
        tokenRef:
          name: provider-keys
          key: openai-key
`, hpaGatewayName, gatewayNamespace)

					hpaGatewayFile := filepath.Join(os.TempDir(), "test-hpa-gateway.yaml")
					err := os.WriteFile(hpaGatewayFile, []byte(hpaGatewayYAML), 0644)
					Expect(err).NotTo(HaveOccurred(), "Failed to write HPA Gateway YAML file")

					cmd := exec.Command("kubectl", "apply", "-f", hpaGatewayFile)
					_, err = utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred(), "Failed to create HPA Gateway")

					By("verifying that HPA is created")
					Eventually(func() error {
						cmd := exec.Command("kubectl", "get", "hpa", fmt.Sprintf("%s-hpa", hpaGatewayName), "-n", gatewayNamespace, "-o", "json")
						output, err := utils.Run(cmd)
						if err != nil {
							return fmt.Errorf("HPA not found: %v", err)
						}

						var hpa autoscalingv2.HorizontalPodAutoscaler
						if err := json.Unmarshal([]byte(output), &hpa); err != nil {
							return fmt.Errorf("failed to parse HPA JSON: %v", err)
						}

						if hpa.Spec.MinReplicas == nil || *hpa.Spec.MinReplicas != 1 {
							return fmt.Errorf("HPA minReplicas incorrect: expected 1, got %v", hpa.Spec.MinReplicas)
						}

						if hpa.Spec.MaxReplicas != 5 {
							return fmt.Errorf("HPA maxReplicas incorrect: expected 5, got %d", hpa.Spec.MaxReplicas)
						}

						if len(hpa.Spec.Metrics) == 0 {
							return fmt.Errorf("HPA metrics not configured")
						}

						foundCPU, foundMemory := false, false
						for _, metric := range hpa.Spec.Metrics {
							if metric.Type == autoscalingv2.ResourceMetricSourceType && metric.Resource != nil {
								switch metric.Resource.Name {
								case "cpu":
									if metric.Resource.Target.Type == autoscalingv2.UtilizationMetricType &&
										metric.Resource.Target.AverageUtilization != nil &&
										*metric.Resource.Target.AverageUtilization == 70 {
										foundCPU = true
									}
								case "memory":
									if metric.Resource.Target.Type == autoscalingv2.UtilizationMetricType &&
										metric.Resource.Target.AverageUtilization != nil &&
										*metric.Resource.Target.AverageUtilization == 80 {
										foundMemory = true
									}
								}
							}
						}

						if !foundCPU {
							return fmt.Errorf("CPU metric with 70%% target not found")
						}
						if !foundMemory {
							return fmt.Errorf("Memory metric with 80%% target not found")
						}

						return nil
					}, timeout, interval).Should(Succeed(), "HPA should be created with correct configuration")

					By("verifying that the Deployment has correct replica configuration for HPA")
					Eventually(func() error {
						cmd := exec.Command("kubectl", "get", "deployment", hpaGatewayName, "-n", gatewayNamespace, "-o", "json")
						output, err := utils.Run(cmd)
						if err != nil {
							return fmt.Errorf("Deployment not found: %v", err)
						}

						var deployment appsv1.Deployment
						if err := json.Unmarshal([]byte(output), &deployment); err != nil {
							return fmt.Errorf("failed to parse Deployment JSON: %v", err)
						}

						if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 1 {
							return fmt.Errorf("Deployment replicas should be set to minReplicas (1), got %v", deployment.Spec.Replicas)
						}

						return nil
					}, timeout, interval).Should(Succeed(), "Deployment should have correct replica configuration")

					By("verifying that HPA target reference is correct")
					cmd = exec.Command("kubectl", "get", "hpa", fmt.Sprintf("%s-hpa", hpaGatewayName), "-n", gatewayNamespace, "-o", "json")
					output, err := utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred(), "Failed to get HPA")

					var hpa autoscalingv2.HorizontalPodAutoscaler
					err = json.Unmarshal([]byte(output), &hpa)
					Expect(err).NotTo(HaveOccurred(), "Failed to parse HPA JSON")

					Expect(hpa.Spec.ScaleTargetRef.APIVersion).To(Equal("apps/v1"))
					Expect(hpa.Spec.ScaleTargetRef.Kind).To(Equal("Deployment"))
					Expect(hpa.Spec.ScaleTargetRef.Name).To(Equal(hpaGatewayName))
				})

				It("should remove HPA when disabled in Gateway spec", func() {
					By("creating a Gateway with HPA enabled initially")
					hpaGatewayYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: %s
  namespace: %s
spec:
  environment: development
  replicas: 3
  hpa:
    enabled: true
    minReplicas: 2
    maxReplicas: 6
    targetCPUUtilizationPercentage: 75
  providers:
    - name: openai
      type: openai
      config:
        baseUrl: "https://api.openai.com/v1"
        tokenRef:
          name: provider-keys
          key: openai-key
`, hpaGatewayName, gatewayNamespace)

					hpaGatewayFile := filepath.Join(os.TempDir(), "test-hpa-gateway-disable.yaml")
					err := os.WriteFile(hpaGatewayFile, []byte(hpaGatewayYAML), 0644)
					Expect(err).NotTo(HaveOccurred(), "Failed to write HPA Gateway YAML file")

					cmd := exec.Command("kubectl", "apply", "-f", hpaGatewayFile)
					_, err = utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred(), "Failed to create HPA Gateway")

					By("waiting for HPA to be created")
					Eventually(func() error {
						cmd := exec.Command("kubectl", "get", "hpa", fmt.Sprintf("%s-hpa", hpaGatewayName), "-n", gatewayNamespace)
						_, err := utils.Run(cmd)
						return err
					}, timeout, interval).Should(Succeed(), "HPA should be created")

					By("updating Gateway to disable HPA")
					disabledHpaGatewayYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: %s
  namespace: %s
spec:
  environment: development
  replicas: 3
  hpa:
    enabled: false
  providers:
    - name: openai
      type: openai
      config:
        baseUrl: "https://api.openai.com/v1"
        tokenRef:
          name: provider-keys
          key: openai-key
`, hpaGatewayName, gatewayNamespace)

					err = os.WriteFile(hpaGatewayFile, []byte(disabledHpaGatewayYAML), 0644)
					Expect(err).NotTo(HaveOccurred(), "Failed to write updated Gateway YAML file")

					cmd = exec.Command("kubectl", "apply", "-f", hpaGatewayFile)
					_, err = utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred(), "Failed to update Gateway")

					By("verifying that HPA is deleted")
					Eventually(func() bool {
						cmd := exec.Command("kubectl", "get", "hpa", fmt.Sprintf("%s-hpa", hpaGatewayName), "-n", gatewayNamespace)
						_, err := utils.Run(cmd)
						return err != nil
					}, timeout, interval).Should(BeTrue(), "HPA should be deleted")

					By("verifying that Deployment replicas are set to the specified value")
					Eventually(func() error {
						cmd := exec.Command("kubectl", "get", "deployment", hpaGatewayName, "-n", gatewayNamespace, "-o", "json")
						output, err := utils.Run(cmd)
						if err != nil {
							return fmt.Errorf("Deployment not found: %v", err)
						}

						var deployment appsv1.Deployment
						if err := json.Unmarshal([]byte(output), &deployment); err != nil {
							return fmt.Errorf("failed to parse Deployment JSON: %v", err)
						}

						if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != 3 {
							return fmt.Errorf("Deployment replicas should be 3 when HPA is disabled, got %v", deployment.Spec.Replicas)
						}

						return nil
					}, timeout, interval).Should(Succeed(), "Deployment should have correct replica count when HPA is disabled")
				})
			})

			Context("OpenTelemetry Integration", func() {
				const (
					otelGatewayName      = "otel-test-gateway"
					otelGatewayNamespace = "default"
					otelConfigMapName    = "otel-test-gateway-config"
					otelDeploymentName   = "otel-test-gateway"
					otelServiceName      = "otel-test-gateway"
					timeout              = 3 * time.Minute
					interval             = 2 * time.Second
				)

				BeforeEach(func() {
					By("cleaning up any existing OpenTelemetry test gateway resources")
					cmd := exec.Command("kubectl", "delete", "gateway", otelGatewayName, "-n", otelGatewayNamespace, "--ignore-not-found=true")
					_, _ = utils.Run(cmd)
					time.Sleep(5 * time.Second)
				})

				AfterEach(func() {
					By("cleaning up OpenTelemetry test gateway resources")
					cmd := exec.Command("kubectl", "delete", "gateway", otelGatewayName, "-n", otelGatewayNamespace, "--ignore-not-found=true")
					_, _ = utils.Run(cmd)
				})

				It("should configure OpenTelemetry metrics and tracing endpoints correctly", func() {
					By("creating a Gateway CR with comprehensive OpenTelemetry configuration")
					gatewayYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: %s
  namespace: %s
spec:
  environment: production
  replicas: 1
  image: "ghcr.io/inference-gateway/inference-gateway:latest"
  
  # OpenTelemetry configuration
  telemetry:
    enabled: true
    metrics:
      enabled: true
      port: 9464
    tracing:
      enabled: true
      endpoint: "http://jaeger-collector:14268/api/traces"
  
  server:
    host: "0.0.0.0"
    port: 8080
  
  # Minimal provider for testing
  providers:
    - name: "openai"
      type: "openai"
      config:
        baseUrl: "https://api.openai.com/v1"
        authType: "bearer"
        tokenRef:
          name: otel-provider-keys
          key: openai-key
`, otelGatewayName, otelGatewayNamespace)

					gatewayFile := filepath.Join(os.TempDir(), "otel-test-gateway.yaml")
					err := os.WriteFile(gatewayFile, []byte(gatewayYAML), 0644)
					Expect(err).NotTo(HaveOccurred(), "Failed to write OpenTelemetry Gateway YAML file")

					By("creating required secrets for the OpenTelemetry test")
					providerKeysYAML := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: otel-provider-keys
  namespace: %s
type: Opaque
stringData:
  openai-key: test-openai-key-for-otel
`, otelGatewayNamespace)

					providerKeysFile := filepath.Join(os.TempDir(), "otel-provider-keys.yaml")
					err = os.WriteFile(providerKeysFile, []byte(providerKeysYAML), 0644)
					Expect(err).NotTo(HaveOccurred(), "Failed to write OpenTelemetry provider keys secret YAML file")

					cmd := exec.Command("kubectl", "apply", "-f", providerKeysFile)
					_, err = utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred(), "Failed to create OpenTelemetry provider keys secret")

					cmd = exec.Command("kubectl", "apply", "-f", gatewayFile)
					_, err = utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred(), "Failed to create OpenTelemetry Gateway CR")

					By("verifying the OpenTelemetry Gateway CR was created successfully")
					verifyOtelGatewayCreated := func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "gateway", otelGatewayName, "-n", otelGatewayNamespace, "-o", "jsonpath={.metadata.name}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "Failed to get OpenTelemetry Gateway CR")
						g.Expect(output).To(Equal(otelGatewayName), "OpenTelemetry Gateway CR not found")
					}
					Eventually(verifyOtelGatewayCreated, timeout, interval).Should(Succeed())

					By("verifying the ConfigMap contains OpenTelemetry configuration")
					verifyOtelConfigMap := func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "configmap", otelConfigMapName, "-n", otelGatewayNamespace)
						_, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "OpenTelemetry ConfigMap not found")

						cmd = exec.Command("kubectl", "get", "configmap", otelConfigMapName, "-n", otelGatewayNamespace, "-o", "jsonpath={.data['config\\.yaml']}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "Failed to get OpenTelemetry ConfigMap data")

						g.Expect(output).To(ContainSubstring("telemetry:"), "Telemetry section missing from config")
						g.Expect(output).To(ContainSubstring("enabled: true"), "Telemetry not enabled in config")

						g.Expect(output).To(ContainSubstring("metrics:"), "Metrics section missing from config")
						g.Expect(output).To(ContainSubstring("port: 9464"), "Metrics port not configured correctly")

						g.Expect(output).To(ContainSubstring("tracing:"), "Tracing section missing from config")
						g.Expect(output).To(ContainSubstring("endpoint: http://jaeger-collector:14268/api/traces"), "Tracing endpoint not configured correctly")
					}
					Eventually(verifyOtelConfigMap, timeout, interval).Should(Succeed())

					By("verifying the Deployment has the metrics port exposed")
					verifyOtelDeployment := func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "deployment", otelDeploymentName, "-n", otelGatewayNamespace)
						_, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "OpenTelemetry Deployment not found")

						cmd = exec.Command("kubectl", "get", "deployment", otelDeploymentName, "-n", otelGatewayNamespace,
							"-o", "jsonpath={.spec.template.spec.containers[0].ports}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "Failed to get container ports")
						g.Expect(output).To(ContainSubstring("9464"), "Metrics port 9464 not exposed in container")

						cmd = exec.Command("kubectl", "get", "deployment", otelDeploymentName, "-n", otelGatewayNamespace,
							"-o", "jsonpath={.spec.template.spec.containers[0].env}")
						output, err = utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "Failed to get container environment variables")
						g.Expect(output).To(ContainSubstring("OPENAI_API_KEY"), "Provider API key environment variable not set")
					}
					Eventually(verifyOtelDeployment, timeout, interval).Should(Succeed())

					By("verifying the Service exposes the metrics port")
					verifyOtelService := func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "service", otelServiceName, "-n", otelGatewayNamespace)
						_, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "OpenTelemetry Service not found")

						cmd = exec.Command("kubectl", "get", "service", otelServiceName, "-n", otelGatewayNamespace,
							"-o", "jsonpath={.spec.ports}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "Failed to get Service ports")
						g.Expect(output).To(ContainSubstring("8080"), "Main service port 8080 not found")
						g.Expect(output).To(ContainSubstring("9464"), "Metrics port 9464 not exposed in service")
					}
					Eventually(verifyOtelService, timeout, interval).Should(Succeed())

					By("verifying Gateway status shows telemetry is enabled")
					verifyOtelGatewayStatus := func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "gateway", otelGatewayName, "-n", otelGatewayNamespace,
							"-o", "jsonpath={.status.phase}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "Failed to get Gateway status")
						g.Expect(output).ToNot(Equal("Failed"), "Gateway should not be in Failed state")
					}
					Eventually(verifyOtelGatewayStatus, timeout, interval).Should(Succeed())
				})

				It("should create a metrics-enabled deployment with proper OpenTelemetry endpoints", func() {
					By("creating a minimal Gateway CR with metrics only")
					gatewayYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: %s-metrics
  namespace: %s
spec:
  environment: development
  replicas: 1
  
  # Metrics-only OpenTelemetry configuration
  telemetry:
    enabled: true
    metrics:
      enabled: true
      port: 9464
    # Tracing explicitly disabled for this test
    tracing:
      enabled: false
  
  server:
    host: "0.0.0.0"
    port: 8080
    
  providers:
    - name: "openai"
      type: "openai" 
      config:
        baseUrl: "https://api.openai.com/v1"
        authType: "bearer"
        tokenRef:
          name: otel-provider-keys
          key: openai-key
`, otelGatewayName, otelGatewayNamespace)

					gatewayFile := filepath.Join(os.TempDir(), "otel-metrics-gateway.yaml")
					err := os.WriteFile(gatewayFile, []byte(gatewayYAML), 0644)
					Expect(err).NotTo(HaveOccurred(), "Failed to write metrics-only Gateway YAML file")

					cmd := exec.Command("kubectl", "apply", "-f", gatewayFile)
					_, err = utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred(), "Failed to create metrics-only Gateway CR")

					By("verifying metrics configuration is applied correctly")
					verifyMetricsConfig := func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "configmap", otelGatewayName+"-metrics-config", "-n", otelGatewayNamespace,
							"-o", "jsonpath={.data['config\\.yaml']}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "Failed to get metrics ConfigMap data")

						g.Expect(output).To(ContainSubstring("enabled: true"), "Metrics should be enabled")
						g.Expect(output).To(ContainSubstring("port: 9464"), "Metrics port should be 9464")

						g.Expect(output).To(ContainSubstring("enabled: false"), "Tracing should be disabled")
					}
					Eventually(verifyMetricsConfig, timeout, interval).Should(Succeed())

					By("cleaning up the metrics-only gateway")
					cmd = exec.Command("kubectl", "delete", "gateway", otelGatewayName+"-metrics", "-n", otelGatewayNamespace, "--ignore-not-found=true")
					_, _ = utils.Run(cmd)
				})

				It("should validate telemetry configuration with different trace endpoints", func() {
					By("testing OTLP gRPC endpoint configuration")
					gatewayYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: %s-grpc
  namespace: %s
spec:
  environment: production
  
  telemetry:
    enabled: true
    metrics:
      enabled: true
      port: 9464
    tracing:
      enabled: true
      endpoint: "http://otel-collector:4317"  # OTLP gRPC endpoint
  
  providers:
    - name: "openai"
      type: "openai"
      config:
        baseUrl: "https://api.openai.com/v1"
        authType: "bearer"
        tokenRef:
          name: otel-provider-keys
          key: openai-key
`, otelGatewayName, otelGatewayNamespace)

					gatewayFile := filepath.Join(os.TempDir(), "otel-grpc-gateway.yaml")
					err := os.WriteFile(gatewayFile, []byte(gatewayYAML), 0644)
					Expect(err).NotTo(HaveOccurred(), "Failed to write OTLP gRPC Gateway YAML file")

					cmd := exec.Command("kubectl", "apply", "-f", gatewayFile)
					_, err = utils.Run(cmd)
					Expect(err).NotTo(HaveOccurred(), "Failed to create OTLP gRPC Gateway CR")

					By("verifying OTLP gRPC tracing endpoint configuration")
					verifyGrpcConfig := func(g Gomega) {
						cmd := exec.Command("kubectl", "get", "configmap", otelGatewayName+"-grpc-config", "-n", otelGatewayNamespace,
							"-o", "jsonpath={.data['config\\.yaml']}")
						output, err := utils.Run(cmd)
						g.Expect(err).NotTo(HaveOccurred(), "Failed to get OTLP gRPC ConfigMap data")

						g.Expect(output).To(ContainSubstring("endpoint: http://otel-collector:4317"), "OTLP gRPC endpoint not configured correctly")
					}
					Eventually(verifyGrpcConfig, timeout, interval).Should(Succeed())

					By("cleaning up the OTLP gRPC gateway")
					cmd = exec.Command("kubectl", "delete", "gateway", otelGatewayName+"-grpc", "-n", otelGatewayNamespace, "--ignore-not-found=true")
					_, _ = utils.Run(cmd)
				})

				It("should handle telemetry configuration validation errors gracefully", func() {
					By("testing invalid metrics port configuration")
					gatewayYAML := fmt.Sprintf(`
apiVersion: core.inference-gateway.com/v1alpha1
kind: Gateway
metadata:
  name: %s-invalid
  namespace: %s
spec:
  environment: development
  
  telemetry:
    enabled: true
    metrics:
      enabled: true
      port: 99999  # Invalid port (too high)
    tracing:
      enabled: false
  
  providers:
    - name: "openai"
      type: "openai"
      config:
        baseUrl: "https://api.openai.com/v1"
        authType: "bearer"
        tokenRef:
          name: otel-provider-keys
          key: openai-key
`, otelGatewayName, otelGatewayNamespace)

					gatewayFile := filepath.Join(os.TempDir(), "otel-invalid-gateway.yaml")
					err := os.WriteFile(gatewayFile, []byte(gatewayYAML), 0644)
					Expect(err).NotTo(HaveOccurred(), "Failed to write invalid Gateway YAML file")

					By("verifying that invalid configuration is rejected")
					cmd := exec.Command("kubectl", "apply", "-f", gatewayFile)
					output, err := utils.Run(cmd)

					if err == nil {
						By("checking that the Gateway shows appropriate status for invalid config")
						verifyInvalidConfig := func(g Gomega) {
							cmd := exec.Command("kubectl", "get", "gateway", otelGatewayName+"-invalid", "-n", otelGatewayNamespace,
								"-o", "jsonpath={.status}")
							output, err := utils.Run(cmd)
							if err == nil && output != "" {
								g.Expect(output).ToNot(ContainSubstring(`"phase":"Running"`), "Gateway should not be running with invalid config")
							}
						}
						Eventually(verifyInvalidConfig, timeout, interval).Should(Succeed())
					} else {
						Expect(output).To(ContainSubstring("Invalid value"), "Expected validation error for invalid port")
					}

					By("cleaning up the invalid gateway if it was created")
					cmd = exec.Command("kubectl", "delete", "gateway", otelGatewayName+"-invalid", "-n", otelGatewayNamespace, "--ignore-not-found=true")
					_, _ = utils.Run(cmd)
				})
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
