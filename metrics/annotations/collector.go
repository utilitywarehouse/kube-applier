package annotations

import (
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/utilitywarehouse/kube-applier/kube"
	"github.com/utilitywarehouse/kube-applier/log"
	"github.com/utilitywarehouse/kube-applier/sysutil"
)

var (
	enabledDesc = prometheus.NewDesc(
		"kube_applier_enabled",
		"Is kube-applier enabled?",
		[]string{"namespace"}, nil,
	)
	dryRunDesc = prometheus.NewDesc(
		"kube_applier_dry_run",
		"Is kube-applier set to dry-run?",
		[]string{"namespace"}, nil,
	)
	pruneDesc = prometheus.NewDesc(
		"kube_applier_prune",
		"Is kube-applier configured to prune resources?",
		[]string{"namespace"}, nil,
	)
	successDesc = prometheus.NewDesc(
		"kube_applier_annotations_get_success",
		"Were the annotations retrieved successfully?",
		[]string{}, nil,
	)
)

// Collector exports kube applier configuration annotations as Prometheus metrics
type Collector struct {
	RepoPath        string
	RepoPathFilters []string
	KubeClient      *kube.Client
}

// Describe describes the metrics exported by Collector
func (c Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- enabledDesc
	ch <- dryRunDesc
	ch <- pruneDesc
	ch <- successDesc
}

// Collect retrieves the annotations from kube for the relevant namespaces and creates metrics from them
func (c Collector) Collect(ch chan<- prometheus.Metric) {
	dirs, err := sysutil.ListDirs(c.RepoPath)
	if err != nil {
		return
	}
	dirs = sysutil.PruneDirs(dirs, c.RepoPathFilters)

	var namespaces []string
	for _, dir := range dirs {
		namespaces = append(namespaces, filepath.Base(dir))
	}

	kaaMap, err := c.KubeClient.NamespaceAnnotationsBatch(namespaces)
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			log.Logger.Error("Error when retrieving namespaces from kube", "error", err, "stderr", string(e.Stderr))
		} else {
			log.Logger.Error("Error when retrieving namespaces from kube", "error", err)
		}
		ch <- prometheus.MustNewConstMetric(
			successDesc,
			prometheus.GaugeValue,
			float64(0),
		)
		return
	}
	ch <- prometheus.MustNewConstMetric(
		successDesc,
		prometheus.GaugeValue,
		float64(1),
	)

	for namespace, annotations := range kaaMap {
		enabled, err := strconv.ParseBool(annotations.Enabled)
		if err == nil {
			ch <- prometheus.MustNewConstMetric(
				enabledDesc,
				prometheus.GaugeValue,
				boolFloat64(enabled),
				namespace,
			)
		}

		dryRun, err := strconv.ParseBool(annotations.DryRun)
		if err == nil {
			ch <- prometheus.MustNewConstMetric(
				dryRunDesc,
				prometheus.GaugeValue,
				boolFloat64(dryRun),
				namespace,
			)
		}

		prune, err := strconv.ParseBool(annotations.Prune)
		if err == nil {
			ch <- prometheus.MustNewConstMetric(
				pruneDesc,
				prometheus.GaugeValue,
				boolFloat64(prune),
				namespace,
			)
		}
	}
}

// boolFloat64 converts a bool to a float64, as expected by Prometheus
// 0=false, 1=true
func boolFloat64(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
