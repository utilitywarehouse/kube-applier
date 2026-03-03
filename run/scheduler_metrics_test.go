package run

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	"github.com/utilitywarehouse/kube-applier/metrics"
)

func TestSchedulerMetrics_resourcesApplied(t *testing.T) {
	metrics.Reset()

	wbList := []kubeapplierv1alpha1.Waybill{
		{
			TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "metrics-foo"},
			Spec: kubeapplierv1alpha1.WaybillSpec{
				AutoApply:   ptr.To(true),
				RunInterval: 3600,
			},
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
			Spec: kubeapplierv1alpha1.WaybillSpec{
				AutoApply:   ptr.To(true),
				RunInterval: 3600,
			},
		},
	}

	metrics.ReconcileFromWaybillList(wbList)
	metrics.UpdateResultSummary(wbList)

	assertMetricValue(t, "kube_applier_waybill_spec_auto_apply", map[string]string{"namespace": "metrics-bar"}, 1)
	assertMetricValue(t, "kube_applier_waybill_spec_auto_apply", map[string]string{"namespace": "metrics-foo"}, 1)
	assertMetricValue(t, "kube_applier_waybill_spec_dry_run", map[string]string{"namespace": "metrics-bar"}, 0)
	assertMetricValue(t, "kube_applier_waybill_spec_dry_run", map[string]string{"namespace": "metrics-foo"}, 0)
	assertMetricValue(t, "kube_applier_waybill_spec_run_interval", map[string]string{"namespace": "metrics-bar"}, 3600)
	assertMetricValue(t, "kube_applier_waybill_spec_run_interval", map[string]string{"namespace": "metrics-foo"}, 3600)

	assertMetricValue(t, "kube_applier_result_summary", map[string]string{
		"action":    "created",
		"name":      "metrics-foo",
		"namespace": "metrics-foo",
		"type":      "namespace",
	}, 1)
	assertMetricValue(t, "kube_applier_result_summary", map[string]string{
		"action":    "created",
		"name":      "test-a",
		"namespace": "metrics-foo",
		"type":      "deployment.apps",
	}, 1)
	assertMetricValue(t, "kube_applier_result_summary", map[string]string{
		"action":    "unchanged",
		"name":      "test-b",
		"namespace": "metrics-foo",
		"type":      "deployment.apps",
	}, 1)
	assertMetricValue(t, "kube_applier_result_summary", map[string]string{
		"action":    "configured",
		"name":      "test-c",
		"namespace": "metrics-foo",
		"type":      "deployment.apps",
	}, 1)
}

func TestSchedulerMetrics_waybillSpecFromClusterState(t *testing.T) {
	metrics.Reset()

	wbList := []kubeapplierv1alpha1.Waybill{
		{
			TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "spec-foo"},
			Spec: kubeapplierv1alpha1.WaybillSpec{
				AutoApply:   ptr.To(true),
				RunInterval: 5,
			},
		},
		{
			TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "spec-bar"},
			Spec: kubeapplierv1alpha1.WaybillSpec{
				AutoApply:   ptr.To(true),
				DryRun:      true,
				RunInterval: 3600,
			},
		},
		{
			TypeMeta:   metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "spec-baz"},
			Spec: kubeapplierv1alpha1.WaybillSpec{
				AutoApply:   ptr.To(false),
				RunInterval: 3600,
			},
		},
	}

	metrics.ReconcileFromWaybillList(wbList)

	assertMetricValue(t, "kube_applier_waybill_spec_dry_run", map[string]string{"namespace": "spec-foo"}, 0)
	assertMetricValue(t, "kube_applier_waybill_spec_auto_apply", map[string]string{"namespace": "spec-foo"}, 1)
	assertMetricValue(t, "kube_applier_waybill_spec_run_interval", map[string]string{"namespace": "spec-foo"}, 5)

	assertMetricValue(t, "kube_applier_waybill_spec_dry_run", map[string]string{"namespace": "spec-bar"}, 1)
	assertMetricValue(t, "kube_applier_waybill_spec_auto_apply", map[string]string{"namespace": "spec-bar"}, 1)
	assertMetricValue(t, "kube_applier_waybill_spec_run_interval", map[string]string{"namespace": "spec-bar"}, 3600)

	assertMetricValue(t, "kube_applier_waybill_spec_dry_run", map[string]string{"namespace": "spec-baz"}, 0)
	assertMetricValue(t, "kube_applier_waybill_spec_auto_apply", map[string]string{"namespace": "spec-baz"}, 0)
	assertMetricValue(t, "kube_applier_waybill_spec_run_interval", map[string]string{"namespace": "spec-baz"}, 3600)
}

func assertMetricValue(t *testing.T, name string, labels map[string]string, expected float64) {
	t.Helper()
	value, ok := metricValue(name, labels)
	require.Truef(t, ok, "metric %s with labels %v was not found", name, labels)
	assert.Equal(t, expected, value)
}

func metricValue(name string, labels map[string]string) (float64, bool) {
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		return 0, false
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			if !labelsMatch(metric.GetLabel(), labels) {
				continue
			}
			if metric.Gauge != nil {
				return metric.GetGauge().GetValue(), true
			}
			if metric.Counter != nil {
				return metric.GetCounter().GetValue(), true
			}
		}
	}
	return 0, false
}

func labelsMatch(actual []*dto.LabelPair, expected map[string]string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for _, label := range actual {
		value, ok := expected[label.GetName()]
		if !ok || value != label.GetValue() {
			return false
		}
	}
	return true
}
