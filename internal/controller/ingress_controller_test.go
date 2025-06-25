/*
Copyright 2025.

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

package controller

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("Ingress Controller", Ordered, func() {
	var (
		baseExpectedSnippet = `# kube-botblocker.github.io operator: Configuration start
# Configuration added by kube-botblocker operator. Do not edit any of this manually
if ($http_user_agent ~* "(GoogleBot|AI2Bot|Ai2Bot-Dolma|Amazonbot|omgili|omgilibot)") {
  return 403;
}
# kube-botblocker.github.io operator: Configuration end`

		existingSnippet = `if ($http_user_agent ~* "testingExistentConfig") {
  return 403
}`
	)

	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: defaultOperatorNamespace,
		},
	}

	BeforeAll(func() {
		k8sClient.Delete(ctx, &namespace)
		By("Creating operator namespace")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Create(ctx, &namespace)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})

	Describe(fmt.Sprintf("When %s='false'", currentNsOnlyEnv), Label(fmt.Sprintf("%s=false", currentNsOnlyEnv)), func() {
		Context(fmt.Sprintf("Creating Ingress with %s annotation", ingConfNameAnn), func() {
			It("Should have SpecHash annotation equal to the IngressConfig SpecHash", func() {
				By("Setting up test context")
				tc := setupDefaultTestContext("ing-spechash-equals-config", nil)

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&tc.ingressConfig), &tc.ingressConfig)).To(Succeed())

				By("Verifying Ingress SpecHash is the same as IngressConfig SpechHash")
				verifySpecHashMatch(&tc.ingress, &tc.ingressConfig)
			})

			It("Should add NGINX configuration to server-snippet", func() {
				By(fmt.Sprintf("Setting up test context with %s annotation", ingConfNameAnn))
				tc := setupDefaultTestContext("ing-creation-default", nil)

				By("Verifying server snippet is added")
				verifyServerSnippet(&tc.ingress, baseExpectedSnippet)
			})

			It("Should append to existing server-snippet", func() {
				By(fmt.Sprintf("Setting up test context with %s annotation and existing server-snippet", ingConfNameAnn))
				annotations := map[string]string{
					serverSnippetAnn: existingSnippet,
				}
				tc := setupDefaultTestContext("ing-creation-append", annotations)

				By("Verifying configuration is appended")
				expectedCombined := existingSnippet + "\n\n" + baseExpectedSnippet
				verifyServerSnippet(&tc.ingress, expectedCombined)
			})

			It("Should skip if the referenced IngressConfig object is not found", func() {
				By("Setting up Ingress referencing unexistent IngressConfig")
				ann := map[string]string{ingConfNameAnn: "unexistent-ingressconfig"}
				tc := testContext{
					ingress: createIngress("ing-unexisting-ingressconfig", "", ann),
				}

				By("Verifying Ingress SpecHash is absent")
				verifySpecHashAbsent(&tc.ingress)

				By("Verifying server-snippet is absent")
				verifyServerSnippetAbsent(&tc.ingress)
			})
		})

		Context("When removing SpecHash annotation from Ingress", func() {
			It("Should restore the SpecHash annotation on Reconcile", func() {
				By("Setting up test context")
				tc := setupDefaultTestContext("ing-spechash-restore", nil)

				By("Deleting the Ingress SpecHash annotation")
				updateIngressAnnotations(&tc.ingress, func(ann map[string]string) {
					delete(ann, ingSpecHashAnn)
				})

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&tc.ingressConfig), &tc.ingressConfig)).To(Succeed())

				By("Verifying Ingress SpecHash has been restored and is the same as IngressConfig SpechHash")
				verifySpecHashMatch(&tc.ingress, &tc.ingressConfig)

			})
		})

		Context(fmt.Sprintf("Adding %s annotation to existing Ingress", ingConfNameAnn), func() {
			It("Should add NGINX configuration to server-snippet", func() {
				By("Setting up test context without annotations")
				tc := testContext{
					ingressConfig: createIngressConfig("ing-existing-default", nil),
					ingress:       createIngress("ing-existing-default", "", nil),
				}

				By("Adding IngressConfig annotation")
				updateIngressAnnotations(&tc.ingress, func(ann map[string]string) {
					ann[ingConfNameAnn] = tc.ingressConfig.Name
				})

				By("Verifying server snippet is added")
				verifyServerSnippet(&tc.ingress, baseExpectedSnippet)
			})

			It("Should append to existing server-snippet", func() {
				By("Setting up test context with only pre-existing server-snippet")
				annotations := map[string]string{
					serverSnippetAnn: existingSnippet,
				}
				tc := testContext{
					ingressConfig: createIngressConfig("ing-existing-append", nil),
					ingress:       createIngress("ing-existing-append", "", annotations),
				}

				By("Adding IngressConfig annotation")
				updateIngressAnnotations(&tc.ingress, func(ann map[string]string) {
					ann[ingConfNameAnn] = tc.ingressConfig.Name
				})

				By("Verifying configuration is appended")
				expectedCombined := existingSnippet + "\n\n" + baseExpectedSnippet
				verifyServerSnippet(&tc.ingress, expectedCombined)
			})
		})

		Context(fmt.Sprintf("Removing %s annotation from existing Ingress", ingConfNameAnn), func() {
			It("Should remove server-snippet when only operator config present", func() {
				By(fmt.Sprintf("Setting up test context with %s annotation", ingConfNameAnn))
				tc := setupDefaultTestContext("ing-removal-default", nil)

				By("Verifying initial configuration")
				verifyServerSnippet(&tc.ingress, baseExpectedSnippet)

				By("Removing IngressConfig annotation")
				updateIngressAnnotations(&tc.ingress, func(ann map[string]string) {
					delete(ann, ingConfNameAnn)
				})

				By("Verifying server snippet is removed")
				verifyServerSnippetAbsent(&tc.ingress)
			})

			It("Should remove only operator related config when other config is present", func() {
				By(fmt.Sprintf("Setting up test context with %s annotation and existing server-snippet", ingConfNameAnn))
				annotations := map[string]string{
					serverSnippetAnn: existingSnippet,
				}
				tc := setupDefaultTestContext("ing-removal-existing", annotations)

				By("Verifying initial combined configuration")
				expectedCombined := existingSnippet + "\n\n" + baseExpectedSnippet
				verifyServerSnippet(&tc.ingress, expectedCombined)

				By("Removing IngressConfig annotation")
				updateIngressAnnotations(&tc.ingress, func(ann map[string]string) {
					delete(ann, ingConfNameAnn)
				})

				By("Verifying only existing snippet remains")
				verifyServerSnippet(&tc.ingress, existingSnippet)
			})
		})
	})

	Describe(fmt.Sprintf("When %s='true'", currentNsOnlyEnv), Label(fmt.Sprintf("%s=true", currentNsOnlyEnv)), func() {
		Context(fmt.Sprintf("Creating Ingress with %s annotation", ingConfNameAnn), func() {
			It("Should only process IngressConfigs in the same namespace", func() {
				By("Creating Ingress in the same namespace as the operator")
				tc := testContext{
					ingressConfig: createIngressConfig("ing-current-ns-only-same-ns", nil),
					ingress:       createIngress("ing-current-ns-only-same-ns", "", nil),
				}

				By("Adding IngressConfig annotation")
				updateIngressAnnotations(&tc.ingress, func(ann map[string]string) {
					ann[ingConfNameAnn] = tc.ingressConfig.Name
				})

				By("Verifying operator configuration is present")
				verifyServerSnippet(&tc.ingress, baseExpectedSnippet)
			})

			It("Should ignore IngressConfigs in other namespaces", func() {
				By("Creating ingress in a namespace different from the operator")
				tc := setupDefaultTestContext("ing-current-ns-only-diff-ns", nil)

				By("Verifying server-snippet annotation wasn't added")
				verifyServerSnippetAbsent(&tc.ingress)

				By("Verifying specHash annotation is absent")
				verifySpecHashAbsent(&tc.ingress)
			})
		})
	})
})
