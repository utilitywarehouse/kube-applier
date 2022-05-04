package webserver

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var varTrue = true
var varFalse = false

type formattingTestCases struct {
	Start                   time.Time
	Finish                  time.Time
	ExpectedLatency         string
	ExpectedFormattedFinish string
	ExpectedFinished        bool
}

var formattingTestCasess = []formattingTestCases{
	// Unfinished
	{time.Time{}, time.Time{}, "0 sec", "0001-01-01 00:00:00 +0000 UTC", false},
	// Zero
	{time.Unix(0, 0).UTC(), time.Unix(0, 0).UTC(), "0 sec", "1970-01-01 00:00:00 +0000 UTC", true},
	// Integer
	{time.Unix(0, 0).UTC(), time.Unix(5, 0).UTC(), "5 sec", "1970-01-01 00:00:05 +0000 UTC", true},
	// Simple float
	{time.Unix(0, 0).UTC(), time.Unix(2, 500000000).UTC(), "2 sec", "1970-01-01 00:00:02 +0000 UTC", true},
	// Complex float - round down
	{time.Unix(0, 0).UTC(), time.Unix(2, 137454234).UTC(), "2 sec", "1970-01-01 00:00:02 +0000 UTC", true},
	// Complex float - round up
	{time.Unix(0, 0).UTC(), time.Unix(2, 537554234).UTC(), "3 sec", "1970-01-01 00:00:02 +0000 UTC", true},
}

func TestResultFormattedTime(t *testing.T) {
	assert := assert.New(t)
	for _, tc := range formattingTestCasess {
		status := kubeapplierv1alpha1.WaybillStatusRun{
			Started:  metav1.NewTime(tc.Start),
			Finished: metav1.NewTime(tc.Finish),
		}
		assert.Equal(tc.ExpectedFormattedFinish, formattedTime(status.Finished))
	}
}

func TestResultLatency(t *testing.T) {
	assert := assert.New(t)
	for _, tc := range formattingTestCasess {
		status := kubeapplierv1alpha1.WaybillStatusRun{
			Started:  metav1.NewTime(tc.Start),
			Finished: metav1.NewTime(tc.Finish),
		}
		assert.Equal(tc.ExpectedLatency, latency(status.Started, status.Finished))
	}
}

var waybills = []kubeapplierv1alpha1.Waybill{
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-success", Name: "main"},
		Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{Success: true}},
		Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-failure", Name: "main"},
		Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{}},
		Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-pending", Name: "main"},
		Status:     kubeapplierv1alpha1.WaybillStatus{},
		Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-warning", Name: "main"},
		Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
		Status: kubeapplierv1alpha1.WaybillStatus{
			LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
				Success: true,
				Output: `namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-dryrun", Name: "main"},
		Status:     kubeapplierv1alpha1.WaybillStatus{},
		Spec:       kubeapplierv1alpha1.WaybillSpec{DryRun: true},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-dryrun-warning", Name: "main"},
		Spec:       kubeapplierv1alpha1.WaybillSpec{DryRun: true},
		Status: kubeapplierv1alpha1.WaybillStatus{
			LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
				Success: true,
				Output: `namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-dryrun-failure", Name: "main"},
		Spec:       kubeapplierv1alpha1.WaybillSpec{DryRun: true},
		Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{}},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-disabled-AA", Name: "main"},
		Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varFalse},
		Status:     kubeapplierv1alpha1.WaybillStatus{},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-disabled-AA-warning", Name: "main"},
		Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varFalse},
		Status: kubeapplierv1alpha1.WaybillStatus{
			LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
				Success: true,
				Output: `namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{Namespace: "test-disabled-AA-failure", Name: "main"},
		Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varFalse},
		Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{}},
	},
}

var events = []corev1.Event{
	{
		TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
		ObjectMeta:     metav1.ObjectMeta{Name: "main", Namespace: "test-failure"},
		Type:           "Warning",
		InvolvedObject: corev1.ObjectReference{Kind: "Waybill", Namespace: "test-failure", Name: "main"},
		Reason:         "WaybillRunRequestFailed",
	},
	{
		TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
		ObjectMeta:     metav1.ObjectMeta{Name: "main", Namespace: "test-pending"},
		Type:           "Error",
		InvolvedObject: corev1.ObjectReference{Kind: "Waybill", Namespace: "test-pending", Name: "main"},
		Reason:         "WaybillRunRequestFailed",
	},
}

func Test_GetNamespaces(t *testing.T) {
	want := []Namespace{
		{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-success", Name: "main"},
				Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{Success: true}},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-failure", Name: "main"},
				Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{}},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
			},
			Events: []corev1.Event{
				{
					TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
					ObjectMeta:     metav1.ObjectMeta{Name: "main", Namespace: "test-failure"},
					InvolvedObject: corev1.ObjectReference{Kind: "Waybill", Namespace: "test-failure", Name: "main"},
					Reason:         "WaybillRunRequestFailed",
					Type:           "Warning",
				},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-pending", Name: "main"},
				Status:     kubeapplierv1alpha1.WaybillStatus{},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
			},
			Events: []corev1.Event{
				{
					TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
					ObjectMeta:     metav1.ObjectMeta{Name: "main", Namespace: "test-pending"},
					InvolvedObject: corev1.ObjectReference{Kind: "Waybill", Namespace: "test-pending", Name: "main"},
					Reason:         "WaybillRunRequestFailed",
					Type:           "Error",
				},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-warning", Name: "main"},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success: true,
						Output: `namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`},
				},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		}, {
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-dryrun", Name: "main"},
				Status:     kubeapplierv1alpha1.WaybillStatus{},
				Spec:       kubeapplierv1alpha1.WaybillSpec{DryRun: true},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-dryrun-warning", Name: "main"},
				Spec:       kubeapplierv1alpha1.WaybillSpec{DryRun: true},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success: true,
						Output: `namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`},
				},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "test-dryrun-failure"},
				Spec:       kubeapplierv1alpha1.WaybillSpec{DryRun: true},
				Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{}},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "test-disabled-AA"},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varFalse},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "test-disabled-AA-warning"},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varFalse},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success: true,
						Output: `namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`},
				},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "test-disabled-AA-failure"},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varFalse},
				Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{}},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
	}

	got := GetNamespaces(waybills, events, diffURL)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("filter() mismatch (-want +got):\n%s", diff)
	}
}

func Test_filter(t *testing.T) {
	Namespaces := GetNamespaces(waybills, []corev1.Event{}, diffURL)

	type args struct {
		outcome string
	}
	tests := []struct {
		name string
		args args
		want Filtered
	}{
		{"unknown", args{"unknown"}, Filtered{FilteredBy: "unknown", Total: 10}},
		{"pending", args{"pending"}, Filtered{FilteredBy: "pending", Total: 10, Namespaces: []Namespace{{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-pending", Name: "main"},
				Status:     kubeapplierv1alpha1.WaybillStatus{},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s"}}},
		},
		{"failure", args{"failure"}, Filtered{FilteredBy: "failure", Total: 10, Namespaces: []Namespace{{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-failure", Name: "main"},
				Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{}},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s"}}},
		},
		{"warning", args{"warning"}, Filtered{FilteredBy: "warning", Total: 10, Namespaces: []Namespace{{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-warning", Name: "main"},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success: true,
						Output: `namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`},
				},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s"}}},
		},
		{"success", args{"success"}, Filtered{FilteredBy: "success", Total: 10, Namespaces: []Namespace{{
			Waybill: kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-success", Name: "main"},
				Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{Success: true}},
				Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varTrue},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s"}}},
		},
		{"dry-run", args{"dry-run"}, Filtered{FilteredBy: "dry-run", Total: 10, Namespaces: []Namespace{
			{
				Waybill: kubeapplierv1alpha1.Waybill{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test-dryrun", Name: "main"},
					Status:     kubeapplierv1alpha1.WaybillStatus{},
					Spec:       kubeapplierv1alpha1.WaybillSpec{DryRun: true},
				},
				DiffURLFormat: "https://github.com/org/repo/commit/%s",
			},
			{
				Waybill: kubeapplierv1alpha1.Waybill{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test-dryrun-warning", Name: "main"},
					Spec:       kubeapplierv1alpha1.WaybillSpec{DryRun: true},
					Status: kubeapplierv1alpha1.WaybillStatus{
						LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
							Success: true,
							Output: `namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`},
					},
				},
				DiffURLFormat: "https://github.com/org/repo/commit/%s",
			},
			{
				Waybill: kubeapplierv1alpha1.Waybill{
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "test-dryrun-failure"},
					Spec:       kubeapplierv1alpha1.WaybillSpec{DryRun: true},
					Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{}},
				},
				DiffURLFormat: "https://github.com/org/repo/commit/%s",
			},
		}},
		},
		{"auto-apply-disabled", args{"auto-apply-disabled"}, Filtered{FilteredBy: "auto-apply-disabled", Total: 10, Namespaces: []Namespace{
			{
				Waybill: kubeapplierv1alpha1.Waybill{
					ObjectMeta: metav1.ObjectMeta{Namespace: "test-disabled-AA", Name: "main"},
					Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varFalse},
					Status:     kubeapplierv1alpha1.WaybillStatus{},
				},
				DiffURLFormat: "https://github.com/org/repo/commit/%s",
			},
			{
				Waybill: kubeapplierv1alpha1.Waybill{
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "test-disabled-AA-warning"},
					Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varFalse},
					Status: kubeapplierv1alpha1.WaybillStatus{
						LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
							Success: true,
							Output: `namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`},
					},
				},
				DiffURLFormat: "https://github.com/org/repo/commit/%s",
			},
			{
				Waybill: kubeapplierv1alpha1.Waybill{
					ObjectMeta: metav1.ObjectMeta{Name: "main", Namespace: "test-disabled-AA-failure"},
					Spec:       kubeapplierv1alpha1.WaybillSpec{AutoApply: &varFalse},
					Status:     kubeapplierv1alpha1.WaybillStatus{LastRun: &kubeapplierv1alpha1.WaybillStatusRun{}},
				},
				DiffURLFormat: "https://github.com/org/repo/commit/%s",
			},
		}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filter(Namespaces, tt.args.outcome)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("filter() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

type lastCommitLinkTestCase struct {
	DiffURLFormat string
	CommitHash    string
	ExpectedLink  string
}

var lastCommitLinkTestCases = []lastCommitLinkTestCase{
	// All empty
	{"", "", ""},
	// Empty URL, non-empty hash
	{"", "hash", ""},
	// URL missing %s, empty hash
	{"https://badurl.com/", "", ""},
	// URL missing %s, non-empty hash
	{"https://badurl.com/", "hash", ""},
	// %s at end of URL, empty hash
	{"https://goodurl.com/%s/", "", ""},
	// %s at end of URL, non-empty hash
	{"https://goodurl.com/%s", "hash", "https://goodurl.com/hash"},
	// %s in middle of URL, empty hash
	{"https://goodurl.com/commit/%s/show", "", ""},
	// %s in middle of URL, non-empty hash
	{"https://goodurl.com/commit/%s/show", "hash", "https://goodurl.com/commit/hash/show"},
}

type eventTestCase struct {
	Events        []corev1.Event
	WaybillEvents []corev1.Event
	Waybill       kubeapplierv1alpha1.Waybill
}

var eventTestCases = []eventTestCase{
	{
		[]corev1.Event{
			{
				InvolvedObject: corev1.ObjectReference{
					Name:      "foobar-0",
					Namespace: "foobar",
				},
				Message: "testing",
			},
			{
				InvolvedObject: corev1.ObjectReference{
					Name:      "foobar-0",
					Namespace: "foobar",
				},
				Message: "testing again",
			},
			{
				InvolvedObject: corev1.ObjectReference{
					Name:      "foobar-0",
					Namespace: "not-foobar",
				},
				Message: "foo",
			},
		},
		[]corev1.Event{
			{
				InvolvedObject: corev1.ObjectReference{
					Name:      "foobar-0",
					Namespace: "foobar",
				},
				Message: "testing",
			},
			{
				InvolvedObject: corev1.ObjectReference{
					Name:      "foobar-0",
					Namespace: "foobar",
				},
				Message: "testing again",
			},
		},
		kubeapplierv1alpha1.Waybill{
			ObjectMeta: metav1.ObjectMeta{Name: "foobar-0", Namespace: "foobar"},
		},
	},
}

func TestResultWaybillEvents(t *testing.T) {
	assert := assert.New(t)
	for _, tc := range eventTestCases {
		assert.Equal(tc.WaybillEvents, waybillEvents(&tc.Waybill, tc.Events))
	}
}

func TestResultLastCommitLink(t *testing.T) {
	assert := assert.New(t)
	for _, tc := range lastCommitLinkTestCases {
		assert.Equal(tc.ExpectedLink, commitLink(tc.DiffURLFormat, tc.CommitHash))
	}
}

func TestResultAppliedRecently(t *testing.T) {
	assert := assert.New(t)

	now := time.Now()
	testCases := []struct {
		t time.Time
		e bool
	}{
		{
			time.Time{},
			false,
		},
		{
			now,
			true,
		},
		{
			now.Add(-time.Minute),
			true,
		},
		{
			now.Add(-time.Minute * 15),
			false,
		},
		{
			now.Add(-time.Minute * 16),
			false,
		},
		{
			now.Add(time.Minute),
			true,
		},
		{
			now.Add(time.Minute * 15),
			true,
		},
		{
			now.Add(time.Minute * 16),
			true,
		},
	}

	assert.Equal(false, appliedRecently(kubeapplierv1alpha1.Waybill{}))

	for _, tc := range testCases {
		assert.Equal(
			tc.e,
			appliedRecently(kubeapplierv1alpha1.Waybill{
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Started: metav1.NewTime(tc.t),
					},
				},
			}),
		)
	}
}

func Test_isOutcomeHasWarnings(t *testing.T) {
	type args struct {
		output string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"empty", args{output: ""}, false},
		{"null", args{}, false},
		{"valid", args{`namespace/zoo unchanged
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`}, true},
		{"no-warning", args{`namespace/zoo unchanged
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged`}, false},
		{"error", args{`networkpolicy.networking.k8s.io/default unchanged
unable to recognize "STDIN": no matches for kind "Ingress" in version "extensions/v1beta1"
unable to recognize "STDIN": no matches for kind "Ingress" in version "extensions/v1beta1"`}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isOutcomeHasWarnings(tt.args.output); got != tt.want {
				t.Errorf("isOutcomeHasWarnings() = %v, want %v", got, tt.want)
			}
		})
	}
}
