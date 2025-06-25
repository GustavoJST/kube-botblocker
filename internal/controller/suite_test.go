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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/GustavoJST/kube-botblocker/api/v1alpha1"
	"github.com/GustavoJST/kube-botblocker/pkg/annotations"
	"github.com/GustavoJST/kube-botblocker/pkg/environment"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client

	currentNsOnlyEnv = "CURRENT_NAMESPACE_ONLY"
	OperatorNsEnv    = "OPERATOR_NAMESPACE"

	defaultTestNamespace     = "default"
	defaultOperatorNamespace = "kube-botblocker"

	ingConfNameAnn   = annotations.IngressConfigNameAnnotation
	ingSpecHashAnn   = annotations.IngressConfigSpecHash
	serverSnippetAnn = annotations.IngressServerSnippet

	defaultBlockedAgents = []string{
		"GoogleBot", "AI2Bot", "Ai2Bot-Dolma",
		"Amazonbot", "omgili", "omgilibot",
	}
)

const (
	timeout  = 1 * time.Minute
	interval = 250 * time.Millisecond
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = v1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	env, err := environment.GetOperatorEnv()
	Expect(err).ToNot(HaveOccurred())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	err = (&IngressReconciler{
		Client:      k8sManager.GetClient(),
		Scheme:      k8sManager.GetScheme(),
		Environment: env,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&IngressConfigReconciler{
		Client: k8sManager.GetClient(),
		Scheme: k8sManager.GetScheme(),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// Makefile targets, the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

type testContext struct {
	ingressConfig v1alpha1.IngressConfig
	ingress       networkingv1.Ingress
}

func makeTestName(base string, suffix int) string {
	return fmt.Sprintf("%s-%d-%d", base, GinkgoRandomSeed(), suffix)
}

// If blockedAgents is nil, use default user agents list for IngressConfig creation
func createIngressConfig(baseName string, blockedAgents []string) v1alpha1.IngressConfig {
	if blockedAgents == nil {
		blockedAgents = defaultBlockedAgents
	}

	name := makeTestName(baseName, GinkgoParallelProcess())
	key := types.NamespacedName{
		Name:      name,
		Namespace: defaultOperatorNamespace,
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

func createIngress(baseName string, namespace string, annotations map[string]string) networkingv1.Ingress {
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

func setupDefaultTestContext(baseName string, ingressAnnotations map[string]string) testContext {
	ingressConfig := createIngressConfig(baseName, nil)

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

func verifyServerSnippet(ingress *networkingv1.Ingress, expected string) {
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(Succeed())
		g.Expect(ingress.GetAnnotations()[serverSnippetAnn]).To(Equal(expected))
	}, timeout, interval).Should(Succeed())
}

func verifyServerSnippetAbsent(ingress *networkingv1.Ingress) {
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(Succeed())
		g.Expect(metav1.HasAnnotation(ingress.ObjectMeta, serverSnippetAnn)).To(
			BeFalseBecause("server-snippet annotation should not be present"),
		)
	}, timeout, interval).Should(Succeed())
}

func verifySpecHashMatch(ingress *networkingv1.Ingress, ingressConfig *v1alpha1.IngressConfig) {
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

func verifySpecHashAbsent(ingress *networkingv1.Ingress) {
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(ingress), ingress)).To(Succeed())
		g.Expect(metav1.HasAnnotation(ingress.ObjectMeta, ingSpecHashAnn)).To(
			BeFalseBecause("IngressConfigSpecHash annotation should not be present"),
		)
	}, timeout, interval).Should(Succeed())
}

func updateIngressAnnotations(ingress *networkingv1.Ingress, updateFunc func(map[string]string)) {
	ann := ingress.GetAnnotations()
	if ann == nil {
		ann = make(map[string]string)
	}
	updateFunc(ann)
	ingress.SetAnnotations(ann)
	Expect(k8sClient.Update(ctx, ingress)).To(Succeed())
}
