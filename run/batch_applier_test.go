package run

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/utilitywarehouse/kube-applier/kube"
	"github.com/utilitywarehouse/kube-applier/metrics"
)

type batchTestCase struct {
	ba        BatchApplier
	applyList []string

	expectedSuccesses []ApplyAttempt
	expectedFailures  []ApplyAttempt
}

func TestBatchApplierApply(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kubeClient := kube.NewMockClientInterface(mockCtrl)
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)

	// Empty apply list
	tc := batchTestCase{BatchApplier{KubeClient: kubeClient, Metrics: metrics}, []string{}, []ApplyAttempt{}, []ApplyAttempt{}}
	expectCheckVersionAndReturnNil(kubeClient)
	applyAndAssert(t, tc)

	// All files succeed
	applyList := []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectCheckVersionAndReturnNil(kubeClient),
		expectApplyAndReturnSuccess("file1", false, kubeClient),
		expectSuccessMetric("file1", metrics),
		expectApplyAndReturnSuccess("file2", false, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectApplyAndReturnSuccess("file3", false, kubeClient),
		expectSuccessMetric("file3", metrics),
	)
	successes := []ApplyAttempt{
		{"file1", "cmd file1", "output file1", ""},
		{"file2", "cmd file2", "output file2", ""},
		{"file3", "cmd file3", "output file3", ""},
	}
	tc = batchTestCase{BatchApplier{KubeClient: kubeClient, Metrics: metrics}, applyList, successes, []ApplyAttempt{}}
	applyAndAssert(t, tc)

	// All files fail
	applyList = []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectCheckVersionAndReturnNil(kubeClient),
		expectApplyAndReturnFailure("file1", false, kubeClient),
		expectFailureMetric("file1", metrics),
		expectApplyAndReturnFailure("file2", false, kubeClient),
		expectFailureMetric("file2", metrics),
		expectApplyAndReturnFailure("file3", false, kubeClient),
		expectFailureMetric("file3", metrics),
	)
	failures := []ApplyAttempt{
		{"file1", "cmd file1", "output file1", "error file1"},
		{"file2", "cmd file2", "output file2", "error file2"},
		{"file3", "cmd file3", "output file3", "error file3"},
	}
	tc = batchTestCase{BatchApplier{KubeClient: kubeClient, Metrics: metrics}, applyList, []ApplyAttempt{}, failures}
	applyAndAssert(t, tc)

	// Some successes, some failures
	applyList = []string{"file1", "file2", "file3", "file4"}
	gomock.InOrder(
		expectCheckVersionAndReturnNil(kubeClient),
		expectApplyAndReturnSuccess("file1", false, kubeClient),
		expectSuccessMetric("file1", metrics),
		expectApplyAndReturnFailure("file2", false, kubeClient),
		expectFailureMetric("file2", metrics),
		expectApplyAndReturnSuccess("file3", false, kubeClient),
		expectSuccessMetric("file3", metrics),
		expectApplyAndReturnFailure("file4", false, kubeClient),
		expectFailureMetric("file4", metrics),
	)
	successes = []ApplyAttempt{
		{"file1", "cmd file1", "output file1", ""},
		{"file3", "cmd file3", "output file3", ""},
	}
	failures = []ApplyAttempt{
		{"file2", "cmd file2", "output file2", "error file2"},
		{"file4", "cmd file4", "output file4", "error file4"},
	}
	tc = batchTestCase{BatchApplier{KubeClient: kubeClient, Metrics: metrics}, applyList, successes, failures}
	applyAndAssert(t, tc)

	// All files succeed dry-run
	applyList = []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectCheckVersionAndReturnNil(kubeClient),
		expectApplyAndReturnSuccess("file1", true, kubeClient),
		expectSuccessMetric("file1", metrics),
		expectApplyAndReturnSuccess("file2", true, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectApplyAndReturnSuccess("file3", true, kubeClient),
		expectSuccessMetric("file3", metrics),
	)
	successes = []ApplyAttempt{
		{"file1", "cmd file1", "output file1", ""},
		{"file2", "cmd file2", "output file2", ""},
		{"file3", "cmd file3", "output file3", ""},
	}
	tc = batchTestCase{BatchApplier{KubeClient: kubeClient, Metrics: metrics, DryRun: true}, applyList, successes, []ApplyAttempt{}}
	applyAndAssert(t, tc)

	// All files succeed disabled namespaces
	applyList = []string{"repo/file1", "file2", "repo/file3"}
	gomock.InOrder(
		expectCheckVersionAndReturnNil(kubeClient),
		expectApplyAndReturnSuccess("repo/file1", true, kubeClient),
		expectSuccessMetric("repo/file1", metrics),
		expectApplyAndReturnSuccess("file2", false, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectApplyAndReturnSuccess("repo/file3", true, kubeClient),
		expectSuccessMetric("repo/file3", metrics),
	)
	successes = []ApplyAttempt{
		{"repo/file1", "cmd repo/file1", "output repo/file1", ""},
		{"file2", "cmd file2", "output file2", ""},
		{"repo/file3", "cmd repo/file3", "output repo/file3", ""},
	}
	tc = batchTestCase{BatchApplier{KubeClient: kubeClient, Metrics: metrics, DryRun: false, DisabledNamespaces: []string{"file1", "file3"}}, applyList, successes, []ApplyAttempt{}}
	applyAndAssert(t, tc)

	// All files succeed dry-run and disabled namespaces
	applyList = []string{"file1", "file2", "file3"}
	gomock.InOrder(
		expectCheckVersionAndReturnNil(kubeClient),
		expectApplyAndReturnSuccess("file1", true, kubeClient),
		expectSuccessMetric("file1", metrics),
		expectApplyAndReturnSuccess("file2", true, kubeClient),
		expectSuccessMetric("file2", metrics),
		expectApplyAndReturnSuccess("file3", true, kubeClient),
		expectSuccessMetric("file3", metrics),
	)
	successes = []ApplyAttempt{
		{"file1", "cmd file1", "output file1", ""},
		{"file2", "cmd file2", "output file2", ""},
		{"file3", "cmd file3", "output file3", ""},
	}
	tc = batchTestCase{BatchApplier{KubeClient: kubeClient, Metrics: metrics, DryRun: true, DisabledNamespaces: []string{"file1", "file3"}}, applyList, successes, []ApplyAttempt{}}
	applyAndAssert(t, tc)
}

func expectCheckVersionAndReturnNil(kubeClient *kube.MockClientInterface) *gomock.Call {
	return kubeClient.EXPECT().CheckVersion().Times(1).Return(nil)
}

func expectApplyAndReturnSuccess(file string, dryRun bool, kubeClient *kube.MockClientInterface) *gomock.Call {
	return kubeClient.EXPECT().Apply(file, dryRun).Times(1).Return("cmd "+file, "output "+file, nil)
}

func expectApplyAndReturnFailure(file string, dryRun bool, kubeClient *kube.MockClientInterface) *gomock.Call {
	return kubeClient.EXPECT().Apply(file, dryRun).Times(1).Return("cmd "+file, "output "+file, fmt.Errorf("error "+file))
}

func expectSuccessMetric(file string, metrics *metrics.MockPrometheusInterface) *gomock.Call {
	return metrics.EXPECT().UpdateNamespaceSuccess(file, true).Times(1)
}

func expectFailureMetric(file string, metrics *metrics.MockPrometheusInterface) *gomock.Call {
	return metrics.EXPECT().UpdateNamespaceSuccess(file, false).Times(1)
}

func applyAndAssert(t *testing.T, tc batchTestCase) {
	assert := assert.New(t)
	successes, failures := tc.ba.Apply(tc.applyList)
	assert.Equal(tc.expectedSuccesses, successes)
	assert.Equal(tc.expectedFailures, failures)
}
