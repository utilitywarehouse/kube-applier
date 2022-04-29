package webserver

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
		ObjectMeta: metav1.ObjectMeta{Namespace: "app-a", Name: "main"},
		Status: kubeapplierv1alpha1.WaybillStatus{
			LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
				Success: true,
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "app-b", Name: "main"},
		Status: kubeapplierv1alpha1.WaybillStatus{
			LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
				Success: false,
			},
		},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Namespace: "app-c", Name: "main"},
		Status:     kubeapplierv1alpha1.WaybillStatus{},
	},
}

var events = []corev1.Event{
	{
		TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
		ObjectMeta:     metav1.ObjectMeta{Name: "main", Namespace: "app-b"},
		Type:           "Warning",
		InvolvedObject: corev1.ObjectReference{Kind: "Waybill", Namespace: "app-b", Name: "main"},
		Reason:         "WaybillRunRequestFailed",
	},
	{
		TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
		ObjectMeta:     metav1.ObjectMeta{Name: "main", Namespace: "app-c"},
		Type:           "Error",
		InvolvedObject: corev1.ObjectReference{Kind: "Waybill", Namespace: "app-c", Name: "main"},
		Reason:         "WaybillRunRequestFailed",
	},
}

func Test_GetNamespaces(t *testing.T) {
	want := []Namespace{
		{
			Waybill:       v1alpha1.Waybill{ObjectMeta: metav1.ObjectMeta{Namespace: "app-a", Name: "main"}, Status: v1alpha1.WaybillStatus{LastRun: &v1alpha1.WaybillStatusRun{Success: true}}},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: v1alpha1.Waybill{ObjectMeta: metav1.ObjectMeta{Namespace: "app-b", Name: "main"}, Status: v1alpha1.WaybillStatus{LastRun: &v1alpha1.WaybillStatusRun{}}},
			Events: []corev1.Event{
				{
					TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
					ObjectMeta:     metav1.ObjectMeta{Name: "main", Namespace: "app-b"},
					InvolvedObject: corev1.ObjectReference{Kind: "Waybill", Namespace: "app-b", Name: "main"},
					Reason:         "WaybillRunRequestFailed",
					Type:           "Warning",
				},
			},
			DiffURLFormat: "https://github.com/org/repo/commit/%s",
		},
		{
			Waybill: v1alpha1.Waybill{ObjectMeta: metav1.ObjectMeta{Namespace: "app-c", Name: "main"}},
			Events: []corev1.Event{
				{
					TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
					ObjectMeta:     metav1.ObjectMeta{Name: "main", Namespace: "app-c"},
					InvolvedObject: corev1.ObjectReference{Kind: "Waybill", Namespace: "app-c", Name: "main"},
					Reason:         "WaybillRunRequestFailed",
					Type:           "Error",
				},
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
		{"unknown", args{"unknown"}, Filtered{Outcome: "unknown", Total: 3}},
		{"pending", args{"pending"}, Filtered{Outcome: "pending", Total: 3, Namespaces: []Namespace{{
			Waybill:       v1alpha1.Waybill{ObjectMeta: metav1.ObjectMeta{Namespace: "app-c", Name: "main"}},
			DiffURLFormat: "https://github.com/org/repo/commit/%s"}}},
		},
		{"failure", args{"failure"}, Filtered{Outcome: "failure", Total: 3, Namespaces: []Namespace{{
			Waybill: v1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "app-b", Name: "main"},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success: false,
					},
				}},
			DiffURLFormat: "https://github.com/org/repo/commit/%s"}}},
		},
		{"success", args{"success"}, Filtered{Outcome: "success", Total: 3, Namespaces: []Namespace{{
			Waybill: v1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Namespace: "app-c", Name: "main"},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success: true,
					},
				}},
			DiffURLFormat: "https://github.com/org/repo/commit/%s"}}},
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
