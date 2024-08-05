package run

import (
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
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

// The ssh keys below are base64-encoded in order to work around
// GitHub's notifications about committing a private key in a public
// repo, which triggers for the deploy key. This key is a read-only
// deploy key for a public repository and is safe to commit.
var (
	randomKey, _ = base64.StdEncoding.DecodeString(
		`LS0tLS1CRUdJTiBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0KYjNCbGJuTnphQzFyWlhrdGRqRUFB
QUFBQkc1dmJtVUFBQUFFYm05dVpRQUFBQUFBQUFBQkFBQUFNd0FBQUF0emMyZ3RaVwpReU5UVXhP
UUFBQUNDbjArQUw1bzNDU1g3U2UwOTY5SUgvYWc4b2hlUkJkUXlwd1dXN1M0N1NMUUFBQUpBYVNL
MmxHa2l0CnBRQUFBQXR6YzJndFpXUXlOVFV4T1FBQUFDQ24wK0FMNW8zQ1NYN1NlMDk2OUlIL2Fn
OG9oZVJCZFF5cHdXVzdTNDdTTFEKQUFBRUJTMUpJNnhwa0lYN1JxK3Nnc1YyM2FrY1FBeGFDaUI4
SjM3b0ZKVkViUHhLZlQ0QXZtamNKSmZ0SjdUM3IwZ2Y5cQpEeWlGNUVGMURLbkJaYnRManRJdEFB
QUFER0ZzYTJGeVFHdDFhbWx5WVFFPQotLS0tLUVORCBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0K`)

	deployKey, _ = base64.StdEncoding.DecodeString(
		`LS0tLS1CRUdJTiBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0KYjNCbGJuTnphQzFyWlhrdGRqRUFB
QUFBQkc1dmJtVUFBQUFFYm05dVpRQUFBQUFBQUFBQkFBQUFNd0FBQUF0emMyZ3RaVwpReU5UVXhP
UUFBQUNEMnlBVGFaZHZGOXFvQU9QWnkrejBSaHI3dm1IdVZ3WldvUkFwYjhuZ3hLQUFBQUpCMm1j
VlZkcG5GClZRQUFBQXR6YzJndFpXUXlOVFV4T1FBQUFDRDJ5QVRhWmR2Rjlxb0FPUFp5K3owUmhy
N3ZtSHVWd1pXb1JBcGI4bmd4S0EKQUFBRUI1VDBoKzNGV0J0M0xaZXpyL00rZzd5Q2NtaHFjYWRQ
V0dTRjltUDh1L21mYklCTnBsMjhYMnFnQTQ5bkw3UFJHRwp2dStZZTVYQmxhaEVDbHZ5ZURFb0FB
QUFER0ZzYTJGeVFHdDFhbWx5WVFFPQotLS0tLUVORCBPUEVOU1NIIFBSSVZBVEUgS0VZLS0tLS0K`)
)

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
Some error output has been omitted because it may contain sensitive data
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

	Context("When operating on a Waybill that defines a git ssh Secret", func() {
		It("Should be able to use it to pull remote kustomize bases", func() {
			// 1. app-b-kustomize-nokey
			// 2. app-b-kustomize-notfound
			// 3. app-b-kustomize-noaccess
			// 4. app-b-kustomize
			// 5. app-b-kustomize-twokeys
			// 6. app-c-kustomize-withkey
			wbList := []*kubeapplierv1alpha1.Waybill{
				{ // when trying to pull a base over ssh without specifying a key, kustomize will return an error
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-b-kustomize",
						Namespace: "app-b-kustomize-nokey",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:      ptr.To(true),
						Prune:          ptr.To(true),
						RepositoryPath: "app-b-kustomize",
					},
				},
				{ // if they key Secret does not exist, we should get an event
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-b-kustomize",
						Namespace: "app-b-kustomize-notfound",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:       ptr.To(true),
						Prune:           ptr.To(true),
						RepositoryPath:  "app-b-kustomize",
						GitSSHSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "git-ssh"},
					},
				},
				{ // if the key does not have access to the repository, kustomize will return an error
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-b-kustomize",
						Namespace: "app-b-kustomize-noaccess",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:       ptr.To(true),
						Prune:           ptr.To(true),
						RepositoryPath:  "app-b-kustomize",
						GitSSHSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "git-ssh"},
					},
				},
				{ // this namespace has a deploy key configured
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-b-kustomize",
						Namespace: "app-b-kustomize",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:       ptr.To(true),
						Prune:           ptr.To(true),
						RepositoryPath:  "app-b-kustomize",
						GitSSHSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "git-ssh"},
					},
				},
				{ // this namespace has a deploy key configured
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-b-kustomize",
						Namespace: "app-b-kustomize-twokeys",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:       ptr.To(true),
						Prune:           ptr.To(true),
						RepositoryPath:  "app-b-kustomize",
						GitSSHSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "git-ssh"},
					},
				},
				{ // the key is irrelevant if the https (default) scheme is used
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-c-kustomize",
						Namespace: "app-c-kustomize-withkey",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:       ptr.To(true),
						Prune:           ptr.To(true),
						RepositoryPath:  "app-c-kustomize",
						GitSSHSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "git-ssh"},
					},
				},
			}

			testEnsureWaybills(wbList)

			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh",
					Namespace: "app-b-kustomize-noaccess",
				},
				StringData: map[string]string{"key_random": string(randomKey)},
				Type:       corev1.SecretTypeOpaque,
			})).To(BeNil())
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh",
					Namespace: "app-b-kustomize",
				},
				StringData: map[string]string{"key_deploy": string(deployKey)},
				Type:       corev1.SecretTypeOpaque,
			})).To(BeNil())
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh",
					Namespace: "app-b-kustomize-twokeys",
				},
				StringData: map[string]string{
					"key_random": string(randomKey),
					"key_deploy": string(deployKey),
				},
				Type: corev1.SecretTypeOpaque,
			})).To(BeNil())
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "git-ssh",
					Namespace: "app-c-kustomize-withkey",
				},
				StringData: map[string]string{"key_random": string(randomKey)},
				Type:       corev1.SecretTypeOpaque,
			})).To(BeNil())

			bHeadCommitHash, err := runner.Repository.HashForPath(context.TODO(), filepath.Join(runner.RepoPath, "app-b-kustomize"))
			Expect(err).To(BeNil())
			Expect(bHeadCommitHash).ToNot(BeEmpty())
			cHeadCommitHash, err := runner.Repository.HashForPath(context.TODO(), filepath.Join(runner.RepoPath, "app-c-kustomize"))
			Expect(err).To(BeNil())
			Expect(cHeadCommitHash).ToNot(BeEmpty())

			expectedStatus := []*kubeapplierv1alpha1.WaybillStatusRun{
				// 1. app-b-kustomize-nokey
				{
					Command:      fmt.Sprintf("^%s build /.*$", kustomizePath),
					Commit:       bHeadCommitHash,
					ErrorMessage: "exit status 1",
					Finished:     metav1.Time{},
					Output:       `(?s).*Please make sure you have the correct access rights.*exit status 128`,
					Started:      metav1.Time{},
					Success:      false,
					Type:         PollingRun.String(),
				},
				// 2. app-b-kustomize-notfound
				nil,
				// 3. app-b-kustomize-noaccess
				//
				// `(na)?` at the end of the pattern never
				// matches, but is there to help us identify which
				// test failed
				{
					Command:      fmt.Sprintf("^%s build /.*$", kustomizePath),
					Commit:       bHeadCommitHash,
					ErrorMessage: "exit status 1",
					Finished:     metav1.Time{},
					Output:       `(?s)Error: accumulating resources:.*'ssh:\/\/deploy_github_com\/utilitywarehouse\/kube-applier\/\/testdata\/bases\/simple-deployment\?ref=master'.*exit status 128(na)?`,
					Started:      metav1.Time{},
					Success:      false,
					Type:         PollingRun.String(),
				},
				// 4. app-b-kustomize
				{
					Command:      "",
					Commit:       bHeadCommitHash,
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-b-kustomize configured
deployment.apps/test-deployment created
`,
					Started: metav1.Time{},
					Success: true,
					Type:    PollingRun.String(),
				},
				// 5. app-b-kustomize-twokeys
				{
					Command:      "",
					Commit:       bHeadCommitHash,
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-b-kustomize unchanged
deployment.apps/test-deployment created
`,
					Started: metav1.Time{},
					Success: true,
					Type:    PollingRun.String(),
				},
				// 6. app-c-kustomize-withkey
				{
					Command:      "",
					Commit:       cHeadCommitHash,
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-c-kustomize created
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
					return k8sClient.GetAPIReader().Get(context.TODO(), client.ObjectKey{Namespace: "app-c-kustomize-withkey", Name: "test-deployment"}, deployment)
				},
				time.Second*120,
				time.Second,
			).Should(BeNil())

			runner.Stop()

			for i := range wbList {
				if wbList[i].Status.LastRun != nil {
					wbList[i].Status.LastRun.Output = testStripKubectlWarnings(wbList[i].Status.LastRun.Output)
				}
				Expect(*wbList[i]).Should(matchWaybill(expected[i], kubeCtlPath, kustomizePath, runner.RepoPath, applyOptions.pruneWhitelist(wbList[i], runner.PruneBlacklist)))
			}

			testMatchEvents([]gomegatypes.GomegaMatcher{
				matchEvent(*wbList[1], corev1.EventTypeWarning, "WaybillRunRequestFailed", `failed setting up repository clone: secrets "git-ssh" not found`),
			})

			testMetrics([]string{
				`kube_applier_last_run_timestamp_seconds`,
				`kube_applier_namespace_apply_count{namespace="app-b-kustomize-nokey",success="false"} 1`,
				`kube_applier_namespace_apply_count{namespace="app-b-kustomize-noaccess",success="false"} 1`,
				`kube_applier_namespace_apply_count{namespace="app-b-kustomize",success="true"} 1`,
				`kube_applier_namespace_apply_count{namespace="app-c-kustomize-withkey",success="true"} 1`,
				`kube_applier_run_latency_seconds`,
				`kube_applier_run_queue{namespace="[^"]+",type="Git polling run"} 0`,
			})
		})
	})

	Context("When operating on a Waybill that uses kustomize with no git secret it should fall back to KA ssh key", func() {
		It("Should be able to build and apply", func() {
			sshKey, err := ioutil.TempFile("", "testGitSSHKey")
			if err != nil {
				panic(err)
			}
			defer syscall.Unlink(sshKey.Name())
			ioutil.WriteFile(sshKey.Name(), []byte(deployKey), 0700)
			runner.DefaultGitSSHKeyPath = sshKey.Name()

			waybill := kubeapplierv1alpha1.Waybill{
				TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "app-d",
					Namespace: "app-d-kustomize",
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
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-d-kustomize configured
deployment.apps/test-deployment created
`,
					Started: metav1.Time{},
					Success: true,
					Type:    PollingRun.String(),
				},
			}

			Enqueue(runQueue, PollingRun, &waybill)
			runner.Stop()

			waybill.Status.LastRun.Output = testStripKubectlWarnings(waybill.Status.LastRun.Output)
			Expect(waybill).Should(matchWaybill(expected, kubeCtlPath, kustomizePath, runner.RepoPath, applyOptions.pruneWhitelist(&waybill, runner.PruneBlacklist)))

			testMetrics([]string{
				`kube_applier_kubectl_exit_code_count{exit_code="0",namespace="app-d-kustomize"} 1`,
				`kube_applier_last_run_timestamp_seconds{namespace="app-d-kustomize"}`,
				`kube_applier_namespace_apply_count{namespace="app-d-kustomize",success="true"} 1`,
				`kube_applier_run_latency_seconds`,
				`kube_applier_run_queue{namespace="app-d-kustomize",type="Git polling run"} 0`,
			})
		})
	})

	Context("When operating on a Waybill that defines a strongbox keyring", func() {
		It("Should be able to apply encrypted files, given a strongbox keyring secret", func() {
			wbList := []*kubeapplierv1alpha1.Waybill{
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-d",
						Namespace: "app-d-missing",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:      ptr.To(true),
						Prune:          ptr.To(true),
						RepositoryPath: "app-d",
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-d",
						Namespace: "app-d-notfound",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:                 ptr.To(true),
						Prune:                     ptr.To(true),
						StrongboxKeyringSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "invalid"},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-d",
						Namespace: "app-d-empty",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:                 ptr.To(true),
						Prune:                     ptr.To(true),
						StrongboxKeyringSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "strongbox-empty"},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-d",
						Namespace: "app-d",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:                 ptr.To(true),
						Prune:                     ptr.To(true),
						StrongboxKeyringSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "strongbox"},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-d",
						Namespace: "app-d-strongbox-shared-not-allowed",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:                 ptr.To(true),
						Prune:                     ptr.To(true),
						RepositoryPath:            "app-d",
						StrongboxKeyringSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "strongbox", Namespace: "app-d"},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-d",
						Namespace: "app-d-strongbox-shared",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:                 ptr.To(true),
						Prune:                     ptr.To(true),
						RepositoryPath:            "app-d",
						StrongboxKeyringSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "strongbox", Namespace: "app-d"},
					},
				},
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "app-d",
						Namespace: "app-d-strongbox-shared-is-allowed",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						AutoApply:                 ptr.To(true),
						Prune:                     ptr.To(true),
						RepositoryPath:            "app-d",
						StrongboxKeyringSecretRef: &kubeapplierv1alpha1.ObjectReference{Name: "strongbox", Namespace: "app-d"},
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
			Expect(k8sClient.GetClient().Create(context.TODO(), &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "strongbox-empty",
					Namespace: "app-d-empty",
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
					ErrorMessage: "exit status 1",
					Finished:     metav1.Time{},
					Output: `namespace/app-d configured
error: error validating "testdata/manifests/app-d/deployment.yaml": error validating data: invalid object to validate; if you choose to ignore these errors, turn validation off with --validate=false
`,
					Started: metav1.Time{},
					Success: false,
					Type:    PollingRun.String(),
				},
				nil,
				nil,
				{
					Command:      "",
					Commit:       headCommitHash,
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-d unchanged
deployment.apps/test-deployment created
`,
					Started: metav1.Time{},
					Success: true,
					Type:    PollingRun.String(),
				},
				nil,
				{
					Command:      "",
					Commit:       headCommitHash,
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-d unchanged
deployment.apps/test-deployment created
`,
					Started: metav1.Time{},
					Success: true,
					Type:    PollingRun.String(),
				},
				{
					Command:      "",
					Commit:       headCommitHash,
					ErrorMessage: "",
					Finished:     metav1.Time{},
					Output: `namespace/app-d unchanged
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
					return k8sClient.GetAPIReader().Get(context.TODO(), client.ObjectKey{Namespace: "app-d", Name: "test-deployment"}, deployment)
				},
				time.Second*15,
				time.Second,
			).Should(BeNil())

			testMatchEvents([]gomegatypes.GomegaMatcher{
				matchEvent(*wbList[1], corev1.EventTypeWarning, "WaybillRunRequestFailed", `failed setting up repository clone: secrets "invalid" not found`),
				matchEvent(*wbList[2], corev1.EventTypeWarning, "WaybillRunRequestFailed", `failed setting up repository clone: secret "app-d-empty/strongbox-empty" does not contain key '.strongbox_keyring'`),
			})

			runner.Stop()

			for i := range wbList {
				if wbList[i].Status.LastRun != nil {
					wbList[i].Status.LastRun.Output = testStripKubectlWarnings(wbList[i].Status.LastRun.Output)
				}
				Expect(*wbList[i]).Should(matchWaybill(expected[i], kubeCtlPath, "", runner.RepoPath, applyOptions.pruneWhitelist(wbList[i], runner.PruneBlacklist)))
			}

			testMetrics([]string{
				`kube_applier_kubectl_exit_code_count{exit_code="1",namespace="app-d-missing"} 1`,
				`kube_applier_last_run_timestamp_seconds{namespace="app-d"}`,
				`kube_applier_namespace_apply_count{namespace="app-d-missing",success="false"} 1`,
				`kube_applier_namespace_apply_count{namespace="app-d",success="true"} 1`,
				`kube_applier_run_latency_seconds`,
				`kube_applier_run_queue{namespace="app-d",type="Git polling run"} 0`,
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
