package webserver

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	kubeapplierv1alpha1 "github.com/utilitywarehouse/kube-applier/apis/kubeapplier/v1alpha1"
)

var warningCheckReg = regexp.MustCompile("^Warning:.*")

// namespace stores the current state of the waybill and events of a namespace.
type Namespace struct {
	Waybill       kubeapplierv1alpha1.Waybill
	Events        []corev1.Event
	DiffURLFormat string
}

// GetNamespaces will create Namespace object combining wayBill and its corresponding events
func GetNamespaces(waybills []kubeapplierv1alpha1.Waybill, allEvents []corev1.Event, diffURL string) []Namespace {
	var ns []Namespace
	for _, wb := range waybills {
		ns = append(ns, Namespace{
			Waybill:       wb,
			DiffURLFormat: diffURL,
			Events:        waybillEvents(&wb, allEvents),
		})
	}

	return ns
}

// waybillEvents returns all the events for the given Waybill
func waybillEvents(wb *kubeapplierv1alpha1.Waybill, allEvents []corev1.Event) []corev1.Event {
	var events []corev1.Event
	for _, e := range allEvents {
		if e.InvolvedObject.Name == wb.Name && e.InvolvedObject.Namespace == wb.Namespace {
			events = append(events, e)
		}
	}

	return events
}

// Filtered stores collections of Namespaces with same outsome
type Filtered struct {
	Outcome    string
	Total      int
	Namespaces []Namespace
}

func filter(Namespaces []Namespace, outcome string) Filtered {
	filtered := Filtered{
		Outcome: outcome,
		Total:   len(Namespaces),
	}
	for _, ns := range Namespaces {
		switch outcome {
		case "pending":
			if ns.Waybill.Status.LastRun == nil {
				filtered.Namespaces = append(filtered.Namespaces, ns)
			}
		case "failure":
			if ns.Waybill.Status.LastRun != nil && !ns.Waybill.Status.LastRun.Success {
				filtered.Namespaces = append(filtered.Namespaces, ns)
			}
		case "warning":
			if ns.Waybill.Status.LastRun != nil && ns.Waybill.Status.LastRun.Success &&
				isOutcomeHasWarnings(ns.Waybill.Status.LastRun.Output) {
				filtered.Namespaces = append(filtered.Namespaces, ns)
			}
		case "success":
			if ns.Waybill.Status.LastRun != nil && ns.Waybill.Status.LastRun.Success &&
				!isOutcomeHasWarnings(ns.Waybill.Status.LastRun.Output) {
				filtered.Namespaces = append(filtered.Namespaces, ns)
			}
		}
	}
	return filtered
}

func isOutcomeHasWarnings(output string) bool {
	for _, l := range strings.Split(output, "\n") {
		if warningCheckReg.MatchString(strings.TrimSpace(l)) {
			return true
		}
	}
	return false
}

// Helper functions used in templates

// FormattedTime returns the Time in the format "YYYY-MM-DD hh:mm:ss -0000 GMT"
func formattedTime(t metav1.Time) string {
	return t.Time.Truncate(time.Second).String()
}

// Latency returns the latency between the two Times in seconds.
func latency(t1, t2 metav1.Time) string {
	return fmt.Sprintf("%.0f sec", t2.Time.Sub(t1.Time).Seconds())
}

// CommitLink returns a URL for the commit most recently applied or it returns
// an empty string if it cannot construct the URL.
func commitLink(diffUrl, commit string) string {
	if commit == "" || diffUrl == "" || !strings.Contains(diffUrl, "%s") {
		return ""
	}
	return fmt.Sprintf(diffUrl, commit)
}

// Status returns a human-readable string that describes the Waybill in terms
// of its autoApply and dryRun attributes.
func status(wb kubeapplierv1alpha1.Waybill) string {
	ret := []string{}
	if !pointer.BoolPtrDerefOr(wb.Spec.AutoApply, true) {
		ret = append(ret, "auto-apply disabled")
	}
	if wb.Spec.DryRun {
		ret = append(ret, "dry-run")
	}
	if len(ret) == 0 {
		return ""
	}
	return fmt.Sprintf("(%s)", strings.Join(ret, ", "))
}

// AppliedRecently checks whether the provided Waybill was applied in the last
// 15 minutes.
func appliedRecently(waybill kubeapplierv1alpha1.Waybill) bool {
	return waybill.Status.LastRun != nil &&
		time.Since(waybill.Status.LastRun.Started.Time) < time.Minute*15
}

func splitByNewline(output string) []string {
	return strings.Split(output, "\n")
}

func getOutputClass(l string) string {
	l = strings.TrimSpace(l)
	if warningCheckReg.MatchString(l) {
		return "text-warning"
	}
	if strings.HasSuffix(l, "configured") {
		return "text-primary"
	}
	if strings.Contains(l, "unable to recognize") ||
		strings.HasPrefix(l, "error:") {
		return "text-danger"
	}
	if strings.Contains(l, "dry run") {
		return "text-info"
	}
	return ""
}
