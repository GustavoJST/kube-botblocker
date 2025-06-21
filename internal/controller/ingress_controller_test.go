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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/GustavoJST/kube-botblocker/api/v1alpha1"
	"github.com/GustavoJST/kube-botblocker/pkg/annotations"
)

var _ = Describe("Ingress Controller", Ordered, func() {
	const (
		timeout  = 10 * time.Second
		duration = 10 * time.Second
		interval = 250 * time.Millisecond

		defaultTestNamespace = "default"
		operatorNamespace    = "kube-botblocker"
	)

	var (
		ingConfNameAnn   = annotations.IngressConfigNameAnnotation
		ingSpecHashAnn   = annotations.IngressConfigSpecHash
		serverSnippetAnn = annotations.IngressServerSnippet

		blockedAgents = []string{
			"GoogleBot", "AI2Bot", "Ai2Bot-Dolma",
			"Amazonbot", "omgili", "omgilibot",
		}

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
			Name: operatorNamespace,
		},
	}

	type testContext struct {
		ingressConfig v1alpha1.IngressConfig
		ingress       networkingv1.Ingress
	}

	makeTestName := func(base string, suffix int) string {
		return fmt.Sprintf("%s-%d-%d", base, GinkgoRandomSeed(), suffix)
	}

	createIngressConfig := func(baseName string) v1alpha1.IngressConfig {
		name := makeTestName(baseName, GinkgoParallelProcess())
		key := types.NamespacedName{
			Name:      name,
			Namespace: operatorNamespace,
		}

		ingressConfig := v1alpha1.IngressConfig{
			TypeMeta: metav1.TypeMeta{
				Kind:       "IngressConfig",
				APIVersion: "kube-botblocker.github.io/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      key.Name,
				Namespace: key.Namespace,
			},
			Spec: v1alpha1.IngressConfigSpec{
				BlockedUserAgents: blockedAgents,
			},
		}

		Expect(k8sClient.Create(ctx, &ingressConfig)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &ingressConfig)).To(Succeed())
		})

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &ingressConfig)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		return ingressConfig
	}

	createIngress := func(baseName string, namespace string, annotations map[string]string) networkingv1.Ingress {
		if namespace == "" {
			namespace = defaultTestNamespace
		}
		name := makeTestName(baseName, GinkgoParallelProcess())
		ingress := networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{
				Name:        name,
				Namespace:   namespace,
				Annotations: annotations,
			},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{}},
			},
		}

		Expect(k8sClient.Create(ctx, &ingress)).To(Succeed())
		DeferCleanup(func() {
			Expect(k8sClient.Delete(ctx, &ingress)).To(Succeed())
		})

		return ingress
	}

	setupDefaultTestContext := func(baseName string, ingressAnnotations map[string]string) testContext {
		ingressConfig := createIngressConfig(baseName)

		if ingressAnnotations == nil {
			ingressAnnotations = make(map[string]string)
		}
		if _, exists := ingressAnnotations[ingConfNameAnn]; !exists {
			ingressAnnotations[ingConfNameAnn] = ingressConfig.Name
		}

		ingress := createIngress(baseName, "", ingressAnnotations)

		return testContext{
			ingressConfig: ingressConfig,
			ingress:       ingress,
		}
	}

	verifyServerSnippet := func(ingress *networkingv1.Ingress, expected string) {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(Succeed())
			g.Expect(ingress.GetAnnotations()[serverSnippetAnn]).To(Equal(expected))
		}, timeout, interval).Should(Succeed())
	}

	verifyServerSnippetAbsent := func(ingress *networkingv1.Ingress) {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(Succeed())
			g.Expect(metav1.HasAnnotation(ingress.ObjectMeta, serverSnippetAnn)).To(
				BeFalseBecause("server-snippet annotation should not be present"),
			)
		}, timeout, interval).Should(Succeed())
	}

	verifySpecHash := func(ingress *networkingv1.Ingress, ingressConfig *v1alpha1.IngressConfig) {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ingressConfig), ingressConfig)).To(Succeed())
			g.Expect(ingressConfig.Status.SpecHash).To(Not(BeEmpty()))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(Succeed())
			g.Expect(ingress.GetAnnotations()[ingSpecHashAnn]).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(ingressConfig.Status.SpecHash).To(Equal(ingress.GetAnnotations()[ingSpecHashAnn]))
		}, timeout, interval).Should(Succeed())
	}

	verifySpecHashAbsent := func(ingress *networkingv1.Ingress) {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(Succeed())
			g.Expect(metav1.HasAnnotation(ingress.ObjectMeta, ingSpecHashAnn)).To(
				BeFalseBecause("IngressConfigSpecHash annotation should not be present"),
			)
		}, timeout, interval).Should(Succeed())
	}

	updateIngressAnnotations := func(ingress *networkingv1.Ingress, updateFunc func(map[string]string)) {
		ann := ingress.GetAnnotations()
		if ann == nil {
			ann = make(map[string]string)
		}
		updateFunc(ann)
		ingress.SetAnnotations(ann)
		Expect(k8sClient.Update(ctx, ingress)).To(Succeed())
	}

	BeforeAll(func() {
		By("Creating operator namespace")
		Expect(k8sClient.Create(ctx, &namespace)).To(Succeed())
	})

	Describe(fmt.Sprintf("When %s='false'", currentNsOnlyEnv), Label(fmt.Sprintf("%s=false", currentNsOnlyEnv)), func() {
		Context(fmt.Sprintf("Creating Ingress with %s annotation", ingConfNameAnn), func() {
			It("Should have SpecHash annotation equal to the IngressConfig SpecHash", func() {
				By("Setting up test context")
				tc := setupDefaultTestContext("ing-spechash-equals-config", nil)

				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&tc.ingressConfig), &tc.ingressConfig)).To(Succeed())

				By("Verifying Ingress SpecHash is the same as IngressConfig SpechHash")
				verifySpecHash(&tc.ingress, &tc.ingressConfig)
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
				verifySpecHash(&tc.ingress, &tc.ingressConfig)

			})
		})

		Context(fmt.Sprintf("Adding %s annotation to existing Ingress", ingConfNameAnn), func() {
			It("Should add NGINX configuration to server-snippet", func() {
				By("Setting up test context without annotations")
				tc := testContext{
					ingressConfig: createIngressConfig("ing-existing-default"),
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
					ingressConfig: createIngressConfig("ing-existing-append"),
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
					ingressConfig: createIngressConfig("ing-current-ns-only-same-ns"),
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
