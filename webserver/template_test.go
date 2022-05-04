package webserver

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	diffURL      = "https://github.com/org/repo/commit/%s"
	fixedTime    = time.Date(2022, time.April, 26, 13, 36, 05, 0, time.UTC)
	emptyLineReg = regexp.MustCompile(`[\t\r\n]+`)

	removeSpaceEmptyLine = cmp.Transformer("RemoveSpace", func(in string) string {
		t := strings.ReplaceAll(in, "  ", "")
		return emptyLineReg.ReplaceAllString(strings.TrimSpace(t), "\n")
	})
)

func Test_ExecuteTemplate(t *testing.T) {
	wbList := []kubeapplierv1alpha1.Waybill{
		{
			TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "main",
				Namespace: "foo",
			},
			Spec: kubeapplierv1alpha1.WaybillSpec{
				AutoApply: &varTrue,
			},
		},
		{
			TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "main",
				Namespace: "bar",
			},
			Spec: kubeapplierv1alpha1.WaybillSpec{
				AutoApply: &varFalse,
			},
		},
		{
			TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "main",
				Namespace: "biz",
			},
			Spec: kubeapplierv1alpha1.WaybillSpec{DryRun: true},
			Status: kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Command:      "/usr/local/bin/kustomize build /tmp/run_biz_main_repo_2305192794/dev/biz | /usr/local/bin/kubectl apply -f - --token=<omitted> -n biz --dry-run=none --prune --all --prune-whitelist=core/v1/ConfigMap serviceaccount/job-trigger unchanged serviceaccount/kube-applier-delegate unchanged",
					Commit:       "22c815614b",
					ErrorMessage: `exit status 1`,
					Started:      metav1.Time{Time: fixedTime.Add(-time.Minute)},
					Finished:     metav1.Time{Time: fixedTime},
					Type:         "Scheduled run",
					Output: `namespace/biz unchanged (server dry run)
serviceaccount/fluentd unchanged (server dry run)
serviceaccount/forwarder unchanged (server dry run)
serviceaccount/kube-applier-delegate unchanged (server dry run)
serviceaccount/loki unchanged (server dry run)
rolebinding.rbac.authorization.k8s.io/admin configured (server dry run)
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged (server dry run)
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
secret/kube-applier-delegate-token unchanged (server dry run)
`,
				},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "main",
				Namespace: "zot",
			},
			Status: kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Commit:       "22c815614b",
					ErrorMessage: `exit status 1`,
					Started:      metav1.Time{Time: fixedTime.Add(-time.Minute)},
					Finished:     metav1.Time{Time: fixedTime},
					Type:         "Git polling run",
				},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "main",
				Namespace: "zoo",
			},
			Status: kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Command:      "/usr/local/bin/kustomize build /tmp/run_zoo_main_repo_2305192794/dev/zoo | /usr/local/bin/kubectl apply -f - --token=<omitted> -n zoo --dry-run=none --prune --all --prune-whitelist=core/v1/ConfigMap serviceaccount/job-trigger unchanged serviceaccount/kube-applier-delegate unchanged",
					Commit:       "22c815614b",
					ErrorMessage: `exit status 1`,
					Started:      metav1.Time{Time: fixedTime.Add(-time.Minute)},
					Finished:     metav1.Time{Time: fixedTime},
					Type:         "Scheduled run",
					Output: `namespace/zoo unchanged
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged
configmap/postgres-init unchanged
configmap/postgres-env unchanged
service/postgres unchanged
service/webapp unchanged
limitrange/default configured
persistentvolumeclaim/postgres unchanged
deployment.apps/postgres unchanged
deployment.apps/webapp configured
deployment.apps/scheduler configured
Warning: autoscaling/v2beta1 HorizontalPodAutoscaler is deprecated in v1.22+, unavailable in v1.25+; use autoscaling/v2beta2 HorizontalPodAutoscaler
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
waybill.kube-applier.io/main unchanged
networkpolicy.networking.k8s.io/default unchanged
error: error validating "/tmp/dev/secrets.yaml": 
unable to recognize "STDIN": no matches for kind "Ingress" in version "extensions/v1beta1"
unable to recognize "STDIN": no matches for kind "Ingress" in version "extensions/v1beta1"
`,
				},
			},
		}, {
			TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "main",
				Namespace: "buz",
			},
			Status: kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Command:  "/usr/local/bin/kustomize build /tmp/run_buz_main_repo_2305192794/dev/buz | /usr/local/bin/kubectl apply -f - --token=<omitted> -n buz --dry-run=none --prune --all --prune-whitelist=core/v1/ConfigMap serviceaccount/job-trigger unchanged serviceaccount/kube-applier-delegate unchanged",
					Commit:   "22c815614b",
					Started:  metav1.Time{Time: fixedTime.Add(-time.Minute)},
					Finished: metav1.Time{Time: fixedTime},
					Success:  true,
					Type:     "Scheduled run",
					Output: `namespace/buz unchanged
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged
configmap/postgres-init unchanged
configmap/postgres-env unchanged
service/postgres unchanged
service/webapp unchanged
limitrange/default configured
persistentvolumeclaim/postgres unchanged
deployment.apps/postgres unchanged
deployment.apps/webapp configured
waybill.kube-applier.io/main unchanged
networkpolicy.networking.k8s.io/default unchanged
`,
				},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "main",
				Namespace: "eng",
			},
			Status: kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Command:  "/usr/local/bin/kustomize build /tmp/run_eng_main_repo_2305192794/dev/eng | /usr/local/bin/kubectl apply -f - --token=<omitted> -n eng --dry-run=none --prune --all --prune-whitelist=core/v1/ConfigMap serviceaccount/job-trigger unchanged serviceaccount/kube-applier-delegate unchanged",
					Commit:   "22c815614b",
					Started:  metav1.Time{Time: fixedTime.Add(-time.Minute)},
					Finished: metav1.Time{Time: fixedTime},
					Success:  true,
					Type:     "Scheduled run",
					Output: `namespace/eng unchanged
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged
waybill.kube-applier.io/main unchanged
networkpolicy.networking.k8s.io/default unchanged
`,
				},
			},
		},
		{
			TypeMeta: metav1.TypeMeta{APIVersion: "kube-applier.io/v1alpha1", Kind: "Waybill"},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "main",
				Namespace: "fuz",
			},
			Status: kubeapplierv1alpha1.WaybillStatus{
				LastRun: &kubeapplierv1alpha1.WaybillStatusRun{
					Command:  "/usr/local/bin/kustomize build /tmp/run_fuz_main_repo_2305192794/dev/fuz | /usr/local/bin/kubectl apply -f - --token=<omitted> -n fuz --dry-run=none --prune --all --prune-whitelist=core/v1/ConfigMap serviceaccount/job-trigger unchanged serviceaccount/kube-applier-delegate unchanged",
					Commit:   "22c815614b",
					Started:  metav1.Time{Time: fixedTime.Add(-time.Minute)},
					Finished: metav1.Time{Time: fixedTime},
					Success:  true,
					Type:     "Scheduled run",
					Output: `namespace/fuz unchanged
serviceaccount/kube-applier-delegate unchanged
rolebinding.rbac.authorization.k8s.io/kube-applier-delegate unchanged
configmap/postgres-init unchanged
configmap/postgres-env unchanged
service/postgres unchanged
service/webapp unchanged
limitrange/default configured
persistentvolumeclaim/postgres unchanged
deployment.apps/postgres unchanged
deployment.apps/webapp configured
waybill.kube-applier.io/main unchanged
networkpolicy.networking.k8s.io/default unchanged
Warning: autoscaling/v2beta1 HorizontalPodAutoscaler is deprecated in v1.22+, unavailable in v1.25+; use autoscaling/v2beta2 HorizontalPodAutoscaler
Warning: batch/v1beta1 CronJob is deprecated in v1.21+, unavailable in v1.25+; use batch/v1 CronJob
Warning: policy/v1beta1 PodDisruptionBudget is deprecated in v1.21+, unavailable in v1.25+; use policy/v1 PodDisruptionBudget
Warning: discovery.k8s.io/v1beta1 EndpointSlice is deprecated in v1.21+, unavailable in v1.25+; use discovery.k8s.io/v1 EndpointSlice
`,
				},
			},
		},
	}

	events := []corev1.Event{
		{
			TypeMeta:       metav1.TypeMeta{Kind: "Event", APIVersion: "v1"},
			FirstTimestamp: metav1.Time{Time: fixedTime.Add(-4 * time.Minute)},
			LastTimestamp:  metav1.Time{Time: fixedTime.Add(4 * time.Minute)},
			Source:         corev1.EventSource{Component: "kube-applier"},
			ObjectMeta:     metav1.ObjectMeta{Name: "main", Namespace: "zot"},
			Type:           "Warning",
			InvolvedObject: corev1.ObjectReference{Kind: "Waybill", Namespace: "zot", Name: "main"},
			Message:        `failed fetching delegate token: secrets "kube-applier-delegate-token" not found`,
			Reason:         "WaybillRunRequestFailed",
		},
	}

	result := GetNamespaces(wbList, events, diffURL)

	templt, err := createTemplate("../templates/status.html")
	if err != nil {
		t.Errorf("error parsing template: %v\n", err)
		return
	}

	rendered := &bytes.Buffer{}
	err = templt.ExecuteTemplate(rendered, "index", result)
	if err != nil {
		t.Errorf("error executing template: %v\n", err)
		return
	}

	// read actual test html output file
	want, err := os.ReadFile("../testdata/web/testStatusPage.html")
	if err != nil {
		t.Errorf("error reading test file:  %v\n", err)
		return
	}

	if diff := cmp.Diff(string(want), rendered.String(), removeSpaceEmptyLine); diff != "" {
		t.Errorf("ExecuteTemplate mismatch (-want +got):\n%s", diff)
	}
}
