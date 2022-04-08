package webserver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
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
	r := Result{}
	for _, tc := range formattingTestCasess {
		status := kubeapplierv1alpha1.WaybillStatusRun{
			Started:  metav1.NewTime(tc.Start),
			Finished: metav1.NewTime(tc.Finish),
		}
		assert.Equal(tc.ExpectedFormattedFinish, r.FormattedTime(status.Finished))
	}
}

func TestResultLatency(t *testing.T) {
	assert := assert.New(t)
	r := Result{}
	for _, tc := range formattingTestCasess {
		status := kubeapplierv1alpha1.WaybillStatusRun{
			Started:  metav1.NewTime(tc.Start),
			Finished: metav1.NewTime(tc.Finish),
		}
		assert.Equal(tc.ExpectedLatency, r.Latency(status.Started, status.Finished))
	}
}

type totalFilesTestCase struct {
	Waybills  []kubeapplierv1alpha1.Waybill
	Failures  []kubeapplierv1alpha1.Waybill
	Successes []kubeapplierv1alpha1.Waybill
	Pending   []kubeapplierv1alpha1.Waybill
}

var totalFilesTestCases = []totalFilesTestCase{
	{nil, nil, nil, nil},
	{
		[]kubeapplierv1alpha1.Waybill{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-a"},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success:           true,
						RunRequestSuccess: true,
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-b"},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success:           false,
						RunRequestSuccess: true,
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-c"},
				Status:     kubeapplierv1alpha1.WaybillStatus{},
			},
		},
		[]kubeapplierv1alpha1.Waybill{
			kubeapplierv1alpha1.Waybill{
				ObjectMeta: metav1.ObjectMeta{Name: "app-b"},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success:           false,
						RunRequestSuccess: true,
					},
				},
			},
		},
		[]kubeapplierv1alpha1.Waybill{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-a"},
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Success:           true,
						RunRequestSuccess: true,
					},
				},
			},
		},
		[]kubeapplierv1alpha1.Waybill{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "app-c"},
				Status:     kubeapplierv1alpha1.WaybillStatus{},
			},
		},
	},
}

func TestResultSuccessesFailuresAndPending(t *testing.T) {
	assert := assert.New(t)
	for _, tc := range totalFilesTestCases {
		r := Result{Waybills: tc.Waybills}
		assert.Equal(tc.Successes, r.Successes())
		assert.Equal(tc.Failures, r.Failures())
		assert.Equal(tc.Pending, r.Pending())
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
		r := Result{Events: tc.Events}
		assert.Equal(tc.WaybillEvents, r.WaybillEvents(&tc.Waybill))
	}
}

func TestResultLastCommitLink(t *testing.T) {
	assert := assert.New(t)
	for _, tc := range lastCommitLinkTestCases {
		r := Result{DiffURLFormat: tc.DiffURLFormat}
		assert.Equal(tc.ExpectedLink, r.CommitLink(tc.CommitHash))
	}
}

func TestResultFinished(t *testing.T) {
	assert := assert.New(t)
	r := Result{}
	assert.Equal(r.Finished(), false)
	r.Waybills = []kubeapplierv1alpha1.Waybill{{}}
	assert.Equal(r.Finished(), true)

	for _, tc := range lastCommitLinkTestCases {
		r := Result{DiffURLFormat: tc.DiffURLFormat}
		assert.Equal(tc.ExpectedLink, r.CommitLink(tc.CommitHash))
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

	r := Result{}

	assert.Equal(false, r.AppliedRecently(kubeapplierv1alpha1.Waybill{}))

	for _, tc := range testCases {
		assert.Equal(
			tc.e,
			r.AppliedRecently(kubeapplierv1alpha1.Waybill{
				Status: kubeapplierv1alpha1.WaybillStatus{
					LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
						Started: metav1.NewTime(tc.t),
					},
				},
			}),
		)
	}
}
