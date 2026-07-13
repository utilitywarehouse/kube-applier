package run

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/kubectl"
	"github.com/utilitywarehouse/kube-applier/metrics"
)

// skipUnlessStrongbox skips specs that rely on the strongbox binary (used as a
// git filter to decrypt the encrypted fixtures) when it is not installed on
// PATH. Everything else in this suite always runs.
func skipUnlessStrongbox() {
	if _, err := exec.LookPath("strongbox"); err != nil {
		Skip("strongbox binary not found on PATH; skipping strongbox spec")
	}
}

func TestApplyOptions_pruneWhitelist(t *testing.T) {
	assert := assert.New(t)

	applyOptions := &ApplyOptions{
		NamespacedResources: []string{"a", "b", "c"},
		ClusterResources:    []string{"d", "e", "f"},
	}

	testCases := []struct {
		options   *ApplyOptions
		waybill   *kubeapplierv1alpha1.Waybill
		blacklist []string
		expected  []string
	}{
		{
			&ApplyOptions{},
			&kubeapplierv1alpha1.Waybill{},
			[]string{},
			nil,
		},
		{
			&ApplyOptions{},
			&kubeapplierv1alpha1.Waybill{
				Spec: kubeapplierv1alpha1.WaybillSpec{
					Prune: ptr.To(true),
				},
			},
			[]string{},
			nil,
		},
		{
			applyOptions,
			&kubeapplierv1alpha1.Waybill{
				Spec: kubeapplierv1alpha1.WaybillSpec{
					Prune: ptr.To(true),
				},
			},
			[]string{},
			[]string{"a", "b", "c"},
		},
		{
			applyOptions,
			&kubeapplierv1alpha1.Waybill{
				Spec: kubeapplierv1alpha1.WaybillSpec{
					Prune:          ptr.To(true),
					PruneBlacklist: []string{"b"},
				},
			},
			[]string{"c"},
			[]string{"a"},
		},
		{
			applyOptions,
			&kubeapplierv1alpha1.Waybill{
				Spec: kubeapplierv1alpha1.WaybillSpec{
					Prune:                 ptr.To(true),
					PruneBlacklist:        []string{"b"},
					PruneClusterResources: true,
				},
			},
			[]string{"c"},
			[]string{"a", "d", "e", "f"},
		},
	}

	for _, tc := range testCases {
		assert.Equal(tc.options.pruneWhitelist(tc.waybill, tc.blacklist), tc.expected)
	}
}

var _ = Describe("Runner", func() {
	var (
		runner        Runner
		runQueue      chan<- Request
		applyOptions  *ApplyOptions
		kustomizePath string
	)

	BeforeEach(func() {
		kubeCtlClient := kubectl.NewClient(cfg.Host, "", kubeCtlPath, kubeCtlOpts)

		runner = Runner{
			Clock:          &zeroClock{},
			DryRun:         false,
			KubeClient:     k8sClient,
			KubeCtlClient:  kubeCtlClient,
			PruneBlacklist: []string{"apps/v1/ControllerRevision"},
			Repository:     repo,
			RepoPath:       "testdata/manifests",
			Strongbox:      &mockStrongboxer{},
			WorkerCount:    1, // limit to one to prevent race issues
		}

		runQueue = runner.Start()
		runnerKubeCtlPath := runner.KubeCtlClient.KubectlPath()
		Expect(runnerKubeCtlPath).ShouldNot(BeEmpty())
		kubeCtlPath = runnerKubeCtlPath

		runnerKustomizePath := runner.KubeCtlClient.KustomizePath()
		Expect(runnerKustomizePath).ShouldNot(BeEmpty())
		kustomizePath = runnerKustomizePath

		cr, nr, err := runner.KubeClient.PrunableResourceGVKs(context.TODO(), "foobar")
		Expect(err).Should(BeNil())
		applyOptions = &ApplyOptions{
			ClusterResources:    cr,
			NamespacedResources: nr,
		}
		metrics.Reset()
	})

	AfterEach(func() {
		runner.Stop()
		testCleanupNamespaces()
	})

	Context("When operating on an empty Waybill list", func() {
		It("Should be a no-op", func() {
			wbList := []kubeapplierv1alpha1.Waybill{}
			wbListExpected := []kubeapplierv1alpha1.Waybill{}

			for i := range wbList {
				Enqueue(runQueue, PollingRun, &wbList[i])
			}
			runner.Stop()

			Expect(wbList).Should(Equal(wbListExpected))
		})
	})

	Context("When operating on a Waybill list", func() {
		It("Should update the Status subresources accordingly", func() {
			wbList := []*kubeapplierv1alpha1.Waybill{
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-a",
						Namespace: "app-a",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply: ptr.To(true),
						Prune:     ptr.To(true),
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-b",
						Namespace: "app-b",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:             ptr.To(true),
						Prune:                 ptr.To(true),
						PruneClusterResources: true,
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-c",
						Namespace: "app-c",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:      ptr.To(true),
						DryRun:         true,
						Prune:          ptr.To(true),
						PruneBlacklist: []string{"core/v1/Pod"},
					},
				},
			}

			testEnsureWaybills(wbList)

			expectedStatus := []*kubeapplierv1alpha1.WaybillStatusRun{
				{
					Command:      "",
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-a configured
deployment.apps/test-deployment created
`,
					Started: metav1.Time{},
					Success: true,
					Type:    PollingRun.String(),
				},
				{
					Command:      "",
					ErrorMessage: "exit status 1",
					Finished:     metav1.Time{},
					Output: `namespace/app-b configured
The Deployment "test-deployment" is invalid: spec.template.spec.containers: Required value
`,
					Started: metav1.Time{},
					Success: false,
					Type:    PollingRun.String(),
				},
				{
					Command:      "",
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-c configured (server dry run)
deployment.apps/test-deployment created (server dry run)
`,
					Started: metav1.Time{},
					Success: true,
					Type:    PollingRun.String(),
				},
			}

			// construct expected waybill list
			expected := make([]kubeapplierv1alpha1.Waybill, len(wbList))
			for i := range wbList {
				expected[i] = *wbList[i]
				expected[i].Status = kubeapplierv1alpha1.WaybillStatus{LastRun: expectedStatus[i]}
				repositoryPath := expected[i].Spec.RepositoryPath
				if repositoryPath == "" {
					repositoryPath = expected[i].Namespace
				}
				headCommitHash, err := runner.Repository.HashForPath(context.TODO(), filepath.Join(runner.RepoPath, repositoryPath))
				Expect(err).To(BeNil())
				expected[i].Status.LastRun.Commit = headCommitHash
			}

			By("Applying all the Waybills and populating their Status subresource with the results")

			for i := range wbList {
				Enqueue(runQueue, PollingRun, wbList[i])
			}
			runner.Stop()

			for i := range wbList {
				wbList[i].Status.LastRun.Output = testStripKubectlWarnings(wbList[i].Status.LastRun.Output)
				Expect(*wbList[i]).Should(matchWaybill(expected[i], kubeCtlPath, "", runner.RepoPath, applyOptions.pruneWhitelist(wbList[i], runner.PruneBlacklist)))
			}

			testMetrics([]string{
				`kube_applier_kubectl_exit_code_count{exit_code="0",namespace="app-a"} 1`,
				`kube_applier_kubectl_exit_code_count{exit_code="1",namespace="app-b"} 1`,
				`kube_applier_kubectl_exit_code_count{exit_code="0",namespace="app-c"} 1`,
				`kube_applier_last_run_timestamp_seconds{namespace="app-a"}`,
				`kube_applier_last_run_timestamp_seconds{namespace="app-b"}`,
				`kube_applier_last_run_timestamp_seconds{namespace="app-c"}`,
				`kube_applier_namespace_apply_count{namespace="app-a",success="true"} 1`,
				`kube_applier_namespace_apply_count{namespace="app-b",success="false"} 1`,
				`kube_applier_namespace_apply_count{namespace="app-c",success="true"} 1`,
				`kube_applier_run_latency_seconds`,
				`kube_applier_run_queue{namespace="app-a",type="Git polling run"} 0`,
				`kube_applier_run_queue{namespace="app-b",type="Git polling run"} 0`,
				`kube_applier_run_queue{namespace="app-c",type="Git polling run"} 0`,
			})
		})
	})

	Context("When operating on a Waybill that uses kustomize", func() {
		It("Should be able to build and apply", func() {
			waybill := kubeapplierv1alpha1.Waybill{
				TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-a",
					Namespace: "app-a-kustomize",
				},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					AutoApply: ptr.To(true),
					Prune:     ptr.To(true),
				},
			}

			testEnsureWaybills([]*kubeapplierv1alpha1.Waybill{&waybill})

			repositoryPath := waybill.Spec.RepositoryPath
			if repositoryPath == "" {
				repositoryPath = waybill.Namespace
			}
			headCommitHash, err := runner.Repository.HashForPath(context.TODO(), filepath.Join(runner.RepoPath, repositoryPath))
			Expect(err).To(BeNil())
			expected := waybill
			expected.Status = kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Command:      "",
					Commit:       headCommitHash,
					ErrorMessage: "exit status 1",
					Finished:     metav1.Time{},
					Output: `namespace/app-a-kustomize configured
deployment.apps/test-deployment created
Error applying Secret(s) [-invalid]; kubectl output has been omitted as it may contain sensitive data.
`,
					Started: metav1.Time{},
					Success: false,
					Type:    PollingRun.String(),
				},
			}

			Enqueue(runQueue, PollingRun, &waybill)
			runner.Stop()

			waybill.Status.LastRun.Output = testStripKubectlWarnings(waybill.Status.LastRun.Output)
			Expect(waybill).Should(matchWaybill(expected, kubeCtlPath, kustomizePath, runner.RepoPath, applyOptions.pruneWhitelist(&waybill, runner.PruneBlacklist)))

			testMetrics([]string{
				`kube_applier_kubectl_exit_code_count{exit_code="0",namespace="app-a-kustomize"} 1`,
				`kube_applier_kubectl_exit_code_count{exit_code="1",namespace="app-a-kustomize"} 1`,
				`kube_applier_last_run_timestamp_seconds{namespace="app-a-kustomize"}`,
				`kube_applier_namespace_apply_count{namespace="app-a-kustomize",success="false"} 1`,
				`kube_applier_run_latency_seconds`,
				`kube_applier_run_queue{namespace="app-a-kustomize",type="Git polling run"} 0`,
			})
		})
	})

	Context("When operating on a Waybill that defines a strongbox keyring", func() {
		It("Should be able to apply encrypted files, given a strongbox keyring secret", func() {
			skipUnlessStrongbox()

			wbList := []*kubeapplierv1alpha1.Waybill{
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-d",
						Namespace: "app-d",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:                 ptr.To(true),
						Prune:                     ptr.To(true),
						RepositoryPath:            "app-d",
						StrongboxKeyringSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "strongbox"},
					},
				},
			}

			testEnsureWaybills(wbList)

			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "strongbox",
					Namespace:   "app-d",
					Annotations: map[string]string{secretAllowedNamespacesAnnotation: "app-d-strongbox-shared,app-d-strongbox-shared-is-*"},
				},
				StringData: map[string]string{
					".strongbox_keyring": `keyentries:
- description: foobar
  key-id: G4M/cCqr+LZtEyQbAjSu5SMEcnVTj2IkWahrkOUq/J4=
  key: QxK6PHX37IybXRshJZy4IXRjCdFFsE0wdiYlfeGP1QA=`,
				},
				Type: corev1.SecretTypeOpaque,
			})).To(BeNil())
			headCommitHash, err := runner.Repository.HashForPath(context.TODO(), filepath.Join(runner.RepoPath, "app-d"))
			Expect(err).To(BeNil())
			Expect(headCommitHash).ToNot(BeEmpty())

			expectedStatus := []*kubeapplierv1alpha1.WaybillStatusRun{
				{
					Command:      "",
					Commit:       headCommitHash,
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output:       `(?s)namespace/app-d (unchanged|configured)\ndeployment\.apps/test-deployment created\n`,
					Started:      metav1.Time{},
					Success:      true,
					Type:         PollingRun.String(),
				},
			}

			// construct expected waybill list
			expected := make([]kubeapplierv1alpha1.Waybill, len(wbList))
			for i := range wbList {
				expected[i] = *wbList[i]
				expected[i].Status = kubeapplierv1alpha1.WaybillStatus{LastRun: expectedStatus[i]}
			}

			Enqueue(runQueue, PollingRun, wbList[0])

			Eventually(
				func() error {
					deployment := &appsv1.Deployment{}
					return k8sClient.GetAPIReader().Get(context.TODO(), client.ObjectKey{Namespace: "app-d", Name: "test-deployment"}, deployment)
				},
				time.Second*45,
				time.Second,
			).Should(BeNil())

			runner.Stop()

			for i := range wbList {
				if wbList[i].Status.LastRun != nil {
					wbList[i].Status.LastRun.Output = testStripKubectlWarnings(wbList[i].Status.LastRun.Output)
				}
				Expect(*wbList[i]).Should(matchWaybill(expected[i], kubeCtlPath, "", runner.RepoPath, applyOptions.pruneWhitelist(wbList[i], runner.PruneBlacklist)))
			}

			testMetrics([]string{
				`kube_applier_last_run_timestamp_seconds{namespace="app-d"}`,
				`kube_applier_namespace_apply_count{namespace="app-d",success="true"} 1`,
				`kube_applier_run_latency_seconds`,
				`kube_applier_run_queue{namespace="app-d",type="Git polling run"} 0`,
			})
		})
	})

	Context("When operating on a Waybill that defines a Strongbox identity", func() {
		It("Should be able to apply encrypted files, given a Strongbox identity Secret", func() {
			skipUnlessStrongbox()

			wbList := []*kubeapplierv1alpha1.Waybill{
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "strongbox-age",
						Namespace: "strongbox-age",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:                 ptr.To(true),
						Prune:                     ptr.To(true),
						RepositoryPath:            "strongbox-age",
						StrongboxKeyringSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "strongbox"},
					},
				},
			}

			testEnsureWaybills(wbList)

			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "strongbox",
					Namespace:   "strongbox-age",
					Annotations: map[string]string{secretAllowedNamespacesAnnotation: "strongbox-age-strongbox-shared,strongbox-age-strongbox-shared-is-*"},
				},
				StringData: map[string]string{
					".strongbox_identity": `# description: ident1
# public key: age1ex4ph3ryaathfac0xpjhxk50utn50mtprke7h0vsmdlh6j63q5dsafxehs
AGE-SECRET-KEY-1GNC98E3WNPAXE49FATT434CFC2THV5Q0SLW45T3VNYUVZ4F8TY6SREQR9Q`,
				},
				Type: corev1.SecretTypeOpaque,
			})).To(BeNil())
			headCommitHash, err := runner.Repository.HashForPath(context.TODO(), filepath.Join(runner.RepoPath, "strongbox-age"))
			Expect(err).To(BeNil())
			Expect(headCommitHash).ToNot(BeEmpty())

			expectedStatus := []*kubeapplierv1alpha1.WaybillStatusRun{
				{
					Command:      "",
					Commit:       headCommitHash,
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output:       `(?s)namespace/strongbox-age (unchanged|configured)\ndeployment\.apps/test-deployment created\n`,
					Started:      metav1.Time{},
					Success:      true,
					Type:         PollingRun.String(),
				},
			}

			// construct expected waybill list
			expected := make([]kubeapplierv1alpha1.Waybill, len(wbList))
			for i := range wbList {
				expected[i] = *wbList[i]
				expected[i].Status = kubeapplierv1alpha1.WaybillStatus{LastRun: expectedStatus[i]}
			}

			Enqueue(runQueue, PollingRun, wbList[0])

			Eventually(
				func() error {
					deployment := &appsv1.Deployment{}
					return k8sClient.GetAPIReader().Get(context.TODO(), client.ObjectKey{Namespace: "strongbox-age", Name: "test-deployment"}, deployment)
				},
				time.Second*45,
				time.Second,
			).Should(BeNil())

			runner.Stop()

			for i := range wbList {
				if wbList[i].Status.LastRun != nil {
					wbList[i].Status.LastRun.Output = testStripKubectlWarnings(wbList[i].Status.LastRun.Output)
				}
				Expect(*wbList[i]).Should(matchWaybill(expected[i], kubeCtlPath, "", runner.RepoPath, applyOptions.pruneWhitelist(wbList[i], runner.PruneBlacklist)))
			}

			testMetrics([]string{
				`kube_applier_last_run_timestamp_seconds{namespace="strongbox-age"}`,
				`kube_applier_namespace_apply_count{namespace="strongbox-age",success="true"} 1`,
				`kube_applier_run_latency_seconds`,
				`kube_applier_run_queue{namespace="strongbox-age",type="Git polling run"} 0`,
			})
		})
	})

	Context("When setting up the apply environment", func() {
		It("Should properly validate the delegate Service Account secret", func() {
			wbList := []*kubeapplierv1alpha1.Waybill{
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-e",
						Namespace: "app-e-notfound",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						DelegateServiceAccountSecretRef: "ka-notfound",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-e",
						Namespace: "app-e-wrongtype",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						DelegateServiceAccountSecretRef: "ka-wrongtype",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-e",
						Namespace: "app-e-notoken",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						DelegateServiceAccountSecretRef: "ka-notoken",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-e",
						Namespace: "app-e",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						DelegateServiceAccountSecretRef: "ka",
					},
				},
			}

			testEnsureWaybills(wbList)

			// Manipulate the delegate Secrets that have been create above
			Expect(k8sClient.GetClient().Delete(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "app-e-notfound", Name: "ka-notfound"}})).To(BeNil())
			Expect(k8sClient.GetClient().Delete(context.TODO(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: "app-e-wrongtype", Name: "ka-wrongtype"}})).To(BeNil())
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Namespace: "app-e-wrongtype", Name: "ka-wrongtype"},
				Type:       corev1.SecretTypeOpaque,
			})).To(BeNil())
			Expect(k8sClient.GetClient().Update(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "app-e-notoken",
					Name:        "ka-notoken",
					Annotations: map[string]string{corev1.ServiceAccountNameKey: "ka-notoken"},
				},
				Type: corev1.SecretTypeServiceAccountToken,
				Data: map[string][]byte{},
			})).To(BeNil())

			headCommitHash, err := runner.Repository.HashForPath(context.TODO(), filepath.Join(runner.RepoPath, "app-e"))
			Expect(err).To(BeNil())
			Expect(headCommitHash).ToNot(BeEmpty())

			expectedStatus := []*kubeapplierv1alpha1.WaybillStatusRun{
				nil,
				nil,
				nil,
				{
					Command:      "",
					Commit:       headCommitHash,
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-e configured
deployment.apps/test-deployment created
`,
					Started: metav1.Time{},
					Success: true,
					Type:    PollingRun.String(),
				},
			}

			// construct expected waybill list
			expected := make([]kubeapplierv1alpha1.Waybill, len(wbList))
			for i := range wbList {
				expected[i] = *wbList[i]
				expected[i].Status = kubeapplierv1alpha1.WaybillStatus{LastRun: expectedStatus[i]}
			}

			for i := range wbList {
				Enqueue(runQueue, PollingRun, wbList[i])
			}

			Eventually(
				func() error {
					deployment := &appsv1.Deployment{}
					return k8sClient.GetAPIReader().Get(context.TODO(), client.ObjectKey{Namespace: "app-e", Name: "test-deployment"}, deployment)
				},
				time.Second*15,
				time.Second,
			).Should(BeNil())

			testMatchEvents([]gomegatypes.GomegaMatcher{
				matchEvent(*wbList[0], corev1.EventTypeWarning, "WaybillRunRequestFailed", `failed fetching delegate token: secrets "ka-notfound" not found`),
				matchEvent(*wbList[1], corev1.EventTypeWarning, "WaybillRunRequestFailed", `failed fetching delegate token: secret "app-e-wrongtype/ka-wrongtype" is not of type `+string(corev1.SecretTypeServiceAccountToken)),
				matchEvent(*wbList[2], corev1.EventTypeWarning, "WaybillRunRequestFailed", `failed fetching delegate token: secret "app-e-notoken/ka-notoken" does not contain key 'token'`),
			})

			runner.Stop()

			for i := range wbList {
				if wbList[i].Status.LastRun != nil {
					wbList[i].Status.LastRun.Output = testStripKubectlWarnings(wbList[i].Status.LastRun.Output)
				}
				Expect(*wbList[i]).Should(matchWaybill(expected[i], kubeCtlPath, "", runner.RepoPath, applyOptions.pruneWhitelist(wbList[i], runner.PruneBlacklist)))
			}

			testMetrics([]string{
				`kube_applier_kubectl_exit_code_count{exit_code="0",namespace="app-e"} 1`,
				`kube_applier_namespace_apply_count{namespace="app-e",success="true"} 1`,
				`kube_applier_run_latency_seconds`,
				`kube_applier_run_queue{namespace="app-e-notfound",type="Git polling run"} 0`,
				`kube_applier_run_queue{namespace="app-e-wrongtype",type="Git polling run"} 0`,
				`kube_applier_run_queue{namespace="app-e",type="Git polling run"} 0`,
			})
		})
	})

	Context("When it fails to enqueue a run request", func() {
		It("Should increase the respective metrics counter", func() {
			smallRunQueue := make(chan Request, 1)
			Enqueue(smallRunQueue, PollingRun, &kubeapplierv1alpha1.Waybill{
				TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
				ObjectMeta: metav1.ObjectMeta{Name: "appD", Namespace: "queued-ok"},
			})
			Enqueue(smallRunQueue, PollingRun, &kubeapplierv1alpha1.Waybill{
				TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
				ObjectMeta: metav1.ObjectMeta{Name: "appD", Namespace: "failed-to-queue"},
			})
			testMetrics([]string{
				`kube_applier_run_queue_failures{namespace="failed-to-queue",type="Git polling run"} 1`,
			})
			Enqueue(smallRunQueue, PollingRun, &kubeapplierv1alpha1.Waybill{
				TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
				ObjectMeta: metav1.ObjectMeta{Name: "appD", Namespace: "failed-to-queue"},
			})
			testMetrics([]string{
				`kube_applier_run_queue_failures{namespace="failed-to-queue",type="Git polling run"} 2`,
			})
		})
	})

	Context("When the Waybill has a GitSSHSecretRef", func() {
		It("Should set up SSH config, key files, and known_hosts from the secret", func() {
			tmpDir := GinkgoT().TempDir()
			ns := "git-ssh-multi-test"

			By("creating the namespace")
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: ns},
			})).To(Succeed())

			By("creating the SSH secret with multiple keys and known_hosts")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh",
					Namespace: ns,
				},
				Data: map[string][]byte{
					"key_deploy":  []byte("fake-deploy-key\n"),
					"key_random":  []byte("fake-random-key\n"),
					"known_hosts": []byte("github.com ssh-ed25519 AAAA...\n"),
				},
			}
			Expect(k8sClient.GetClient().Create(context.TODO(), secret)).To(Succeed())

			waybill := &kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-multi",
					Namespace: ns,
				},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					GitSSHSecretRef: &kubeapplierv1alpha1.ObjectReference{
						Name: "git-ssh",
					},
				},
			}

			By("calling setupGitSSH")
			env, err := runner.setupGitSSH(context.Background(), waybill, tmpDir)
			Expect(err).To(BeNil())

			By("verifying GIT_SSH_COMMAND")
			Expect(env).To(ContainSubstring("GIT_SSH_COMMAND=ssh -q -F " + filepath.Join(tmpDir, ".ssh", "config")))
			Expect(env).To(ContainSubstring("UserKnownHostsFile=" + filepath.Join(tmpDir, ".ssh", "known_hosts")))

			By("verifying the SSH config file contains host blocks for both keys")
			configData, err := os.ReadFile(filepath.Join(tmpDir, ".ssh", "config"))
			Expect(err).To(BeNil())
			configStr := string(configData)
			Expect(configStr).To(ContainSubstring("Host deploy_github_com"))
			Expect(configStr).To(ContainSubstring("Host random_github_com"))
			// Two keys: no Host github.com fallback
			Expect(configStr).NotTo(ContainSubstring("\nHost github.com"))

			By("verifying key files were written")
			deployKey, err := os.ReadFile(filepath.Join(tmpDir, ".ssh", "key_deploy"))
			Expect(err).To(BeNil())
			Expect(string(deployKey)).To(Equal("fake-deploy-key\n"))
			randomKey, err := os.ReadFile(filepath.Join(tmpDir, ".ssh", "key_random"))
			Expect(err).To(BeNil())
			Expect(string(randomKey)).To(Equal("fake-random-key\n"))

			By("verifying known_hosts file was written")
			knownHosts, err := os.ReadFile(filepath.Join(tmpDir, ".ssh", "known_hosts"))
			Expect(err).To(BeNil())
			Expect(string(knownHosts)).To(Equal("github.com ssh-ed25519 AAAA...\n"))
		})

		It("Should add Host github.com fallback for single key", func() {
			tmpDir := GinkgoT().TempDir()
			ns := "git-ssh-single-test"

			By("creating namespace")
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: ns},
			})).To(Succeed())

			By("creating secret with a single key")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh-single",
					Namespace: ns,
				},
				Data: map[string][]byte{
					"key_deploy": []byte("single-key\n"),
				},
			}
			Expect(k8sClient.GetClient().Create(context.TODO(), secret)).To(Succeed())

			waybill := &kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-single",
					Namespace: ns,
				},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					GitSSHSecretRef: &kubeapplierv1alpha1.ObjectReference{
						Name: "git-ssh-single",
					},
				},
			}

			By("calling setupGitSSH")
			env, err := runner.setupGitSSH(context.Background(), waybill, tmpDir)
			Expect(err).To(BeNil())

			By("verifying the Host github.com fallback is present")
			configData, err := os.ReadFile(filepath.Join(tmpDir, ".ssh", "config"))
			Expect(err).To(BeNil())
			configStr := string(configData)
			Expect(configStr).To(ContainSubstring("Host deploy_github_com"))
			Expect(configStr).To(ContainSubstring("Host github.com"))
			// known_hosts not in secret: fragment defaults to /dev/null
			Expect(env).To(ContainSubstring("UserKnownHostsFile=/dev/null"))
		})

		It("Should default known_hosts to /dev/null when not present in secret", func() {
			tmpDir := GinkgoT().TempDir()
			ns := "git-ssh-nokh-test"

			By("creating namespace")
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: ns},
			})).To(Succeed())

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh-nokh",
					Namespace: ns,
				},
				Data: map[string][]byte{
					"key_deploy": []byte("deploy-key\n"),
				},
			}
			Expect(k8sClient.GetClient().Create(context.TODO(), secret)).To(Succeed())

			waybill := &kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nokh",
					Namespace: ns,
				},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					GitSSHSecretRef: &kubeapplierv1alpha1.ObjectReference{
						Name: "git-ssh-nokh",
					},
				},
			}

			env, err := runner.setupGitSSH(context.Background(), waybill, tmpDir)
			Expect(err).To(BeNil())

			// No known_hosts in secret => fragment defaults to /dev/null
			Expect(env).To(ContainSubstring("UserKnownHostsFile=/dev/null"))
			Expect(env).To(ContainSubstring("StrictHostKeyChecking=no"))
		})

		It("Should error when secret is not in the waybill namespace and lacks the allowed-namespaces annotation", func() {
			tmpDir := GinkgoT().TempDir()
			ns := "git-ssh-crossns-test"
			secretNs := "git-ssh-other-ns"

			By("creating both namespaces")
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: ns},
			})).To(Succeed())
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: secretNs},
			})).To(Succeed())

			By("creating secret in a different namespace without the allowed-namespaces annotation")
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh-other",
					Namespace: secretNs,
				},
				Data: map[string][]byte{
					"key_deploy": []byte("cross-ns-key\n"),
				},
			}
			Expect(k8sClient.GetClient().Create(context.TODO(), secret)).To(Succeed())

			waybill := &kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-crossns",
					Namespace: ns,
				},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					GitSSHSecretRef: &kubeapplierv1alpha1.ObjectReference{
						Name:      "git-ssh-other",
						Namespace: secretNs,
					},
				},
			}

			_, err := runner.setupGitSSH(context.Background(), waybill, tmpDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot be used in namespace"))
			Expect(err.Error()).To(ContainSubstring(secretAllowedNamespacesAnnotation))
		})
	})
})

var _ = Describe("Run Queue", func() {
	Context("When a Waybill autoApply is disabled", func() {
		It("Should only only be applied for forced run requests", func() {
			waybill := kubeapplierv1alpha1.Waybill{
				TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "waybill-auto-apply-disabled",
				},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					AutoApply: ptr.To(false),
					Prune:     ptr.To(true),
				},
			}

			fakeRunQueue := make(chan Request, 4)
			Enqueue(fakeRunQueue, ScheduledRun, &waybill)
			Enqueue(fakeRunQueue, PollingRun, &waybill)
			Enqueue(fakeRunQueue, ForcedRun, &waybill)

			close(fakeRunQueue)

			res := []Request{}
			for req := range fakeRunQueue {
				res = append(res, req)
			}
			Expect(res).To(Equal([]Request{
				{Type: ForcedRun, Waybill: &waybill},
			}))
		})
	})
})

func matchWaybill(expected kubeapplierv1alpha1.Waybill, kubectlPath, kustomizePath, repoPath string, pruneWhitelist []string) gomegatypes.GomegaMatcher {
	lastRunMatcher := BeNil()
	if expected.Status.LastRun != nil {
		var commandMatcher gomegatypes.GomegaMatcher
		if strings.HasPrefix(expected.Status.LastRun.Command, "^") ||
			strings.HasPrefix(expected.Status.LastRun.Command, "(?") {
			commandMatcher = MatchRegexp(expected.Status.LastRun.Command)
		} else {
			commandExtraArgs := expected.Status.LastRun.Command
			if expected.Spec.DryRun {
				commandExtraArgs += " --dry-run=server"
			} else {
				commandExtraArgs += " --dry-run=none"
			}
			if ptr.Deref(expected.Spec.Prune, true) {
				commandExtraArgs += fmt.Sprintf(" --prune --all --prune-allowlist=%s", strings.Join(pruneWhitelist, " --prune-allowlist="))
			}
			repositoryPath := expected.Spec.RepositoryPath
			if repositoryPath == "" {
				repositoryPath = expected.Namespace
			}
			if kustomizePath == "" {
				commandMatcher = MatchRegexp(
					`^%s( --kubeconfig=.*\.kubecfg)? --server %s apply -f \S+/%s -R --token=<omitted> -n %s%s`,
					kubectlPath,
					cfg.Host,
					repositoryPath,
					expected.Namespace,
					commandExtraArgs,
				)
			} else {
				commandMatcher = MatchRegexp(
					`^%s build \S+/%s \| %s( --kubeconfig=.*\.kubecfg)? --server %s apply -f - --token=<omitted> -n %s%s`,
					kustomizePath,
					repositoryPath,
					kubectlPath,
					cfg.Host,
					expected.Namespace,
					commandExtraArgs,
				)
			}
		}
		var outputMatcher gomegatypes.GomegaMatcher
		if strings.HasPrefix(expected.Status.LastRun.Output, "(") ||
			strings.HasPrefix(expected.Status.LastRun.Output, "(?") {
			outputMatcher = MatchRegexp(expected.Status.LastRun.Output)
		} else {
			outputMatcher = MatchRegexp("^%s$", strings.Replace(
				regexp.QuoteMeta(expected.Status.LastRun.Output),
				regexp.QuoteMeta(repoPath),
				"[^ ]+",
				-1,
			))
		}
		lastRunMatcher = PointTo(MatchAllFields(Fields{
			"Command":      commandMatcher,
			"Commit":       Equal(expected.Status.LastRun.Commit),
			"ErrorMessage": Equal(expected.Status.LastRun.ErrorMessage),
			"Finished": And(
				Equal(expected.Status.LastRun.Finished),
				// Ideally we would be comparing to actual's Started but since it
				// should be equal to expected' Started, this is equivalent.
				MatchAllFields(Fields{
					"Time": BeTemporally(">=", expected.Status.LastRun.Started.Time),
				}),
			),
			"Output":  outputMatcher,
			"Started": Equal(expected.Status.LastRun.Started),
			"Success": Equal(expected.Status.LastRun.Success),
			"Type":    Equal(expected.Status.LastRun.Type),
		}))
	}
	return MatchAllFields(Fields{
		"TypeMeta":   Equal(expected.TypeMeta),
		"ObjectMeta": Equal(expected.ObjectMeta),
		"Spec":       Equal(expected.Spec),
		"Status": MatchAllFields(Fields{
			"LastRun": lastRunMatcher,
		}),
	})
}

func testStripKubectlWarnings(output string) string {
	lines := strings.Split(output, "\n")
	ret := []string{}
	for _, l := range lines {
		if !strings.HasPrefix(l, "Warning:") {
			ret = append(ret, l)
		}
	}
	return strings.Join(ret, "\n")
}
