package run

import (
	"context"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/git"
	"github.com/utilitywarehouse/kube-applier/metrics"
)

func testSchedulerDrainRequests(requests <-chan Request) (func() []Request, func()) {
	m := sync.Mutex{}
	reqs := []Request{}
	finished := make(chan bool)

	go func() {
		for r := range requests {
			m.Lock()
			reqs = append(reqs, r)
			m.Unlock()
		}
		close(finished)
	}()

	return func() []Request {
			m.Lock()
			defer m.Unlock()
			ret := make([]Request, len(reqs))
			for i := range reqs {
				ret[i] = reqs[i]
			}
			return ret
		}, func() {
			<-finished
		}
}

var _ = Describe("Scheduler", func() {
	var (
		testRunQueue              chan Request
		testScheduler             Scheduler
		testSchedulerRequests     func() []Request
		testSchedulerRequestsWait func()
	)

	BeforeEach(func() {
		testRunQueue = make(chan Request)
		testSchedulerRequests, testSchedulerRequestsWait = testSchedulerDrainRequests(testRunQueue)
		testScheduler = Scheduler{
			WaybillPollInterval: time.Second * 5,
			Clock:               &zeroClock{},
			GitPollInterval:     time.Second * 5,
			KubeClient:          testKubeClient,
			RepoPath:            "../testdata/manifests",
			RunQueue:            testRunQueue,
		}
		testScheduler.Start()

		metrics.Reset()
	})

	AfterEach(func() {
		testScheduler.Stop()
		testCleanupNamespaces()
	})

	Context("When running", func() {
		It("Should keep track of Waybill resources on the server", func() {
			By("Listing all the Waybills in the cluster initially")
			wbList := []*kubeapplierv1alpha1.Waybill{
				{
					TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "main",
						Namespace: "foo",
					},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						RepositoryPath: "foo",
						RunInterval:    5,
					},
				},
			}
			testEnsureWaybills(wbList)
			testWaitForSchedulerToUpdate(&testScheduler, wbList)

			lastSyncedAt := time.Now()

			By("Listing all the Waybills in the cluster regularly")
			wbList = append(wbList, &kubeapplierv1alpha1.Waybill{
				TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "main",
					Namespace: "bar",
				},
				Spec: kubeapplierv1alpha1.WaybillSpec{
					RepositoryPath: "bar",
				},
			})
			testEnsureWaybills(wbList)
			testWaitForSchedulerToUpdate(&testScheduler, wbList)

			t := time.Second*15 - time.Since(lastSyncedAt)
			if t > 0 {
				fmt.Printf("Sleeping for ~%v to record queued runs\n", t.Truncate(time.Second))
				time.Sleep(t)
			}
			lastSyncedAt = time.Now()

			By("Acknowledging changes in the Waybill Specs")
			wbList[0].Spec.RunInterval = 3600
			wbList[0].Status = kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Started:  metav1.NewTime(time.Now()), // this is to prevent an "initial" run to be queued
					Finished: metav1.NewTime(time.Now()), // the rest is for the status subresource to pass validation
					Success:  true,
				},
			}
			// remove the "bar" Waybill
			testKubeClient.Delete(context.TODO(), wbList[1])
			wbList = wbList[:1]
			testEnsureWaybills(wbList)
			testWaitForSchedulerToUpdate(&testScheduler, wbList)

			t = time.Second*15 - time.Since(lastSyncedAt)
			if t > 0 {
				fmt.Printf("Sleeping for ~%v to record queued runs\n", t.Truncate(time.Second))
				time.Sleep(t)
			}

			testWaitForRequests(testSchedulerRequests, MatchAllKeys(Keys{
				"foo": MatchAllKeys(Keys{
					// RunInterval is 5s and ~15s have elapsed until it is updated to 3600s.
					ScheduledRun: BeNumerically(">=", 4),
				}),
				"bar": MatchAllKeys(Keys{
					// RunInterval is 3600s and then the Waybill is removed.
					ScheduledRun: Equal(1),
				}),
			}))

			testScheduler.Stop()
			close(testRunQueue)
			testSchedulerRequestsWait()
		})

		It("Should trigger runs for Waybills that have had their source change in git", func() {
			gitUtil := &git.Util{RepoPath: "../testdata/manifests"}
			headHash, err := gitUtil.HeadHashForPaths(".")
			Expect(err).To(BeNil())
			Expect(headHash).ToNot(BeEmpty())
			appAHeadHash, err := gitUtil.HeadHashForPaths("app-a")
			Expect(err).To(BeNil())
			Expect(appAHeadHash).ToNot(BeEmpty())
			appAKHeadHash, err := gitUtil.HeadHashForPaths("app-a-kustomize")
			Expect(err).To(BeNil())
			Expect(appAKHeadHash).ToNot(BeEmpty())

			wbList := []*kubeapplierv1alpha1.Waybill{
				{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "ignored"},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						RepositoryPath: "ignored",
					},
					Status: kubeapplierv1alpha1.WaybillStatus{
						LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
							Finished: metav1.NewTime(time.Now()),
							Started:  metav1.NewTime(time.Now()),
						},
					},
				},
				{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "up-to-date"},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						RepositoryPath: "up-to-date",
					},
					Status: kubeapplierv1alpha1.WaybillStatus{
						LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
							Finished: metav1.NewTime(time.Now()),
							Started:  metav1.NewTime(time.Now()),
							Commit:   headHash,
						},
					},
				},
				{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "scheduler-polling-app-a"},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						RepositoryPath: "app-a",
					},
					Status: kubeapplierv1alpha1.WaybillStatus{
						LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
							Finished: metav1.NewTime(time.Now()),
							Started:  metav1.NewTime(time.Now()),
							Commit:   appAHeadHash, // this is the app-a dir head hash, no changes since
						},
					},
				},
				{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "scheduler-polling-app-a-kustomize"},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						RepositoryPath: "app-a-kustomize",
					},
					Status: kubeapplierv1alpha1.WaybillStatus{
						LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
							Finished: metav1.NewTime(time.Now()),
							Started:  metav1.NewTime(time.Now()),
							Commit:   fmt.Sprintf("%s^1", appAKHeadHash), // this is a hack that should always return changes
						},
					},
				},
			}
			testEnsureWaybills(wbList)
			testWaitForSchedulerToUpdate(&testScheduler, wbList)

			// This is a hack to force the scheduler to re-check all
			// Waybills for this test. Otherwise, the test is sensitive to
			// timing and can fail if the git polling check runs before the
			// Scheduler has synced all Waybills from the apiserver.
			testScheduler.gitLastQueuedHash = ""

			testWaitForRequests(testSchedulerRequests, MatchAllKeys(Keys{
				"scheduler-polling-app-a-kustomize": MatchAllKeys(Keys{
					PollingRun: Equal(1),
				}),
			}))

			testScheduler.Stop()
			close(testRunQueue)
			testSchedulerRequestsWait()
		})

		It("Should export metrics about resources applied", func() {
			By("Listing all the Waybills in the cluster")
			// The status sub-resource only contains the Output field and this
			// is is only used to test that metrics are properly exported
			wbList := []*kubeapplierv1alpha1.Waybill{
				{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "metrics-foo"},
					Status: kubeapplierv1alpha1.WaybillStatus{
						LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
							Finished: metav1.NewTime(time.Now()),
							Started:  metav1.NewTime(time.Now()),
							Output: `namespace/metrics-foo created
deployment.apps/test-a created (server dry run)
deployment.apps/test-b unchanged
deployment.apps/test-c configured
error: error validating "../testdata/manifests/app-d/deployment.yaml": error validating data: invalid object to validate; if you choose to ignore these errors, turn validation off with --validate=false
Some error output has been omitted because it may contain sensitive data
`,
						},
					},
				},
				{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "metrics-bar"},
				},
			}
			testEnsureWaybills(wbList)
			testWaitForSchedulerToUpdate(&testScheduler, wbList)

			testScheduler.Stop()
			close(testRunQueue)

			By("Parsing the Output field in the Waybill status and exporting metrics about individual resources")
			testMetrics([]string{
				`kube_applier_result_summary{action="created",name="metrics-foo",namespace="metrics-foo",type="namespace"} 1`,
				`kube_applier_result_summary{action="created",name="test-a",namespace="metrics-foo",type="deployment.apps"} 1`,
				`kube_applier_result_summary{action="unchanged",name="test-b",namespace="metrics-foo",type="deployment.apps"} 1`,
				`kube_applier_result_summary{action="configured",name="test-c",namespace="metrics-foo",type="deployment.apps"} 1`,
			})
		})

		It("Should export Waybill spec metrics from the cluster state", func() {
			wbList := []*kubeapplierv1alpha1.Waybill{
				{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "spec-foo"},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						RepositoryPath: "foo",
						RunInterval:    5,
					},
				},
				{
					TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "spec-bar"},
					Spec: kubeapplierv1alpha1.WaybillSpec{
						RepositoryPath: "bar",
						DryRun:         true,
					},
				},
			}
			testEnsureWaybills(wbList)
			testWaitForSchedulerToUpdate(&testScheduler, wbList)

			testScheduler.Stop()
			close(testRunQueue)

			testMetrics([]string{
				`kube_applier_waybill_spec_dry_run{namespace="spec-foo"} 0`,
				`kube_applier_waybill_spec_run_interval{namespace="spec-foo"} 5`,
				`kube_applier_waybill_spec_dry_run{namespace="spec-bar"} 1`,
				`kube_applier_waybill_spec_run_interval{namespace="spec-bar"} 3600`,
			})
		})
	})
})

func testEnsureWaybills(wbList []*kubeapplierv1alpha1.Waybill) {
	for i := range wbList {
		err := testKubeClient.Create(context.TODO(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: wbList[i].Namespace}})
		if err != nil && !errors.IsAlreadyExists(err) {
			Expect(err).To(BeNil())
		}
		err = testKubeClient.Create(context.TODO(), wbList[i])
		if err != nil {
			Expect(testKubeClient.UpdateWaybill(context.TODO(), wbList[i])).To(BeNil())
		}
		if wbList[i].Status.LastRun != nil {
			// UpdateStatus changes SelfLink to the status sub-resource but we
			// should revert the change for tests to pass
			selfLink := wbList[i].ObjectMeta.SelfLink
			Expect(testKubeClient.UpdateWaybillStatus(context.TODO(), wbList[i])).To(BeNil())
			wbList[i].ObjectMeta.SelfLink = selfLink
		}
		// This is a workaround for Equal checks to work below.
		// Apparently, List will return Waybills with TypeMeta but
		// Get and Create (which updates the struct) do not.
		wbList[i].TypeMeta = metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"}
	}
}

func testWaitForSchedulerToUpdate(s *Scheduler, wbList []*kubeapplierv1alpha1.Waybill) {
	Eventually(
		testSchedulerCopyWaybillsMap(s),
		time.Second*15,
		time.Second,
	).Should(Equal(testSchedulerExpectedWaybillsMap(wbList)))
}

func testWaitForRequests(actual func() []Request, expected gomegatypes.GomegaMatcher) {
	Eventually(
		func() map[string]map[Type]int {
			requestCount := map[string]map[Type]int{}
			for _, req := range actual() {
				if _, ok := requestCount[req.Waybill.Namespace]; !ok {
					requestCount[req.Waybill.Namespace] = map[Type]int{}
				}
				requestCount[req.Waybill.Namespace][req.Type]++
			}
			return requestCount
		},
		time.Second*30,
		time.Second,
	).Should(expected)
}

func testSchedulerExpectedWaybillsMap(wbList []*kubeapplierv1alpha1.Waybill) map[string]*kubeapplierv1alpha1.Waybill {
	expectedWaybillMap := map[string]*kubeapplierv1alpha1.Waybill{}
	for i := range wbList {
		expectedWaybillMap[wbList[i].Namespace] = wbList[i]
	}
	return expectedWaybillMap
}

func testSchedulerCopyWaybillsMap(scheduler *Scheduler) func() map[string]*kubeapplierv1alpha1.Waybill {
	return func() map[string]*kubeapplierv1alpha1.Waybill {
		scheduler.waybillsMutex.Lock()
		defer scheduler.waybillsMutex.Unlock()
		waybills := map[string]*kubeapplierv1alpha1.Waybill{}
		for i := range scheduler.waybills {
			waybills[i] = scheduler.waybills[i]
		}
		return waybills
	}
}
