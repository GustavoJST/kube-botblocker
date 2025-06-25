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
	"time"

	"github.com/GustavoJST/kube-botblocker/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	expectedFinalizer = "batch.tutorial.kubebuilder.io/finalizer"
)

var _ = Describe("IngressConfig Controller", Ordered, func() {
	namespace := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-botblocker",
		},
	}

	BeforeAll(func() {
		_ = k8sClient.Delete(ctx, &namespace)
		By("Creating operator namespace")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Create(ctx, &namespace)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	})

	fetchUpdate := func(ingressConfig *v1alpha1.IngressConfig) {
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ingressConfig), ingressConfig)).To(Succeed())
		}, timeout, interval).Should(Succeed())
	}

	Context("When creating a IngressConfig", func() {
		It("Should reconcile successfully", func() {
			By("Creating the IngressConfig")
			ingressConfig := createIngressConfig("ingressconfig-creation", nil)

			By("Waiting for initial reconciliation to complete")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.LastConditionMessage).To(Equal("Ready for usage"))
				g.Expect(meta.IsStatusConditionTrue(ingressConfig.Status.Conditions, "UpdateSucceeded")).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			By("Having the finalizer")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.GetFinalizers()).To((HaveLen(1)))
				g.Expect(ingressConfig.GetFinalizers()[0]).To(Equal(expectedFinalizer))
			}, timeout, interval).Should(Succeed())

			By("Having .spec.blockedUserAgents not be empty")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Spec.BlockedUserAgents).To(Not(BeEmpty()))
			}, timeout, interval).Should(Succeed())

			By("Having .status.specHash not be empty")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.SpecHash).To(Not(BeEmpty()))
			}, timeout, interval).Should(Succeed())

			By("Having the .status.observedGeneration field match .metadata.generation")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.ObservedGeneration).To(Equal(ingressConfig.ObjectMeta.Generation))
			}, timeout, interval).Should(Succeed())

			condition := meta.FindStatusCondition(ingressConfig.Status.Conditions, "UpdateSucceeded")

			By("Having .status.lastConditionMessage be equal to the correct condition message")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.LastConditionMessage).To(Equal(condition.Message))
			}, timeout, interval).Should(Succeed())

			By("Having .status.lastConditionStatus be equal to the correct condition status")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.LastConditionStatus).To(Equal(condition.Status))
			}, timeout, interval).Should(Succeed())

			By("Having .status.lastUpdated be equal to the correct condition lastTransitionTime")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.LastUpdated.Time).To(Equal(condition.LastTransitionTime.Time))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("When updating the Spec of a IngressConfig without associated Ingresses", func() {
		It("Should reconcile successfully", func() {
			By("Creating the IngressConfig")
			ingressConfig := createIngressConfig("ingressconfig-update-no-ingress", nil)

			By("Waiting for initial reconciliation to complete")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.LastConditionMessage).To(Equal("Ready for usage"))
				g.Expect(meta.IsStatusConditionTrue(ingressConfig.Status.Conditions, "UpdateSucceeded")).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			lastUpdatedBefore := *ingressConfig.Status.LastUpdated.DeepCopy()
			specHashBefore := ingressConfig.Status.SpecHash

			// Sleep to have a meaninful difference in lastUpdated
			time.Sleep(5 * time.Second)

			By("Updating the Ingressconfig Spec")
			blockedAgents := ingressConfig.Spec.BlockedUserAgents
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				ingressConfig.Spec.BlockedUserAgents = blockedAgents[:len(blockedAgents)-1]
				g.Expect(k8sClient.Update(ctx, &ingressConfig)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Having the finalizer")
			Expect(ingressConfig.GetFinalizers()).To((HaveLen(1)))
			Expect(ingressConfig.GetFinalizers()[0]).To(Equal(expectedFinalizer))

			By("Having the .status.specHash be updated")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.SpecHash).To(Not(Equal(specHashBefore)))
			}, timeout, interval).Should(Succeed())

			By("Having the blocked user agent list be updated")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Spec.BlockedUserAgents).To(Equal(
					[]string{"GoogleBot", "AI2Bot", "Ai2Bot-Dolma", "Amazonbot", "omgili"},
				))
			}, timeout, interval).Should(Succeed())

			By("Having the .status.observedGeneration field match .metadata.generation")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.ObservedGeneration).To(Equal(ingressConfig.ObjectMeta.Generation))
			}, timeout, interval).Should(Succeed())

			By("Having the .status.lastConditionMessage remain the same")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.LastConditionMessage).To(Equal("Ready for usage"))
			}, timeout, interval).Should(Succeed())

			By("Having the .status.lastConditionStatus be True")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.LastConditionStatus).To(Equal(metav1.ConditionTrue))
			}, timeout, interval).Should(Succeed())

			By("Having the .status.lastUpdated be updated")
			Eventually(func(g Gomega) {
				fetchUpdate(&ingressConfig)
				g.Expect(ingressConfig.Status.LastUpdated.Time.After(lastUpdatedBefore.Time)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

	})

	Context("When updating the Spec of a IngressConfig with associated Ingresses", func() {
		It("Should show the correct status Condition", func() {
			By("Creating Ingress and IngressConfig")
			tc := testContext{
				ingressConfig: createIngressConfig("ingressconfig-update-with-ingress", nil),
				ingress:       createIngress("ingressconfig-update-with-ingress", "", nil),
			}

			By("Associating Ingress with IngressConfig")
			updateIngressAnnotations(&tc.ingress, func(ann map[string]string) {
				ann[ingConfNameAnn] = tc.ingressConfig.Name
			})

			By("Updating IngressConfig Spec")
			blockedAgents := tc.ingressConfig.Spec.BlockedUserAgents
			cutBlockedAgents := blockedAgents[:len(blockedAgents)-1]

			Eventually(func(g Gomega) {
				fetchUpdate(&tc.ingressConfig)
				tc.ingressConfig.Spec.BlockedUserAgents = cutBlockedAgents
				g.Expect(k8sClient.Update(ctx, &tc.ingressConfig)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("Checking if status condition is correct")
			Eventually(func(g Gomega) {
				fetchUpdate(&tc.ingressConfig)
				condition := meta.FindStatusCondition(tc.ingressConfig.Status.Conditions, "UpdateSucceeded")
				g.Expect(condition).To(Not(BeNil()))
				g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(condition.Reason).To(Equal("ReconciliationSuccessful"))
			}, timeout, interval).Should(Succeed())
		})
	})
})
