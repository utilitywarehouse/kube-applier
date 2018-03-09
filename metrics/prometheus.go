package metrics

import (
	"path/filepath"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusInterface allows for mocking out the functionality of Prometheus when testing the full process of an apply run.
type PrometheusInterface interface {
	UpdateNamespaceSuccess(string, bool)
	UpdateRunLatency(float64, bool)
	UpdateLogCount(level string)
}

// Prometheus implements instrumentation of metrics for kube-applier.
// fileApplyCount is a Counter vector to increment the number of successful and failed apply attempts for each file in the repo.
// runLatency is a Summary vector that keeps track of the duration for apply runs.
type Prometheus struct {
	namespaceApplyCount *prometheus.CounterVec
	runLatency          *prometheus.HistogramVec
	logCount            *prometheus.CounterVec
}

// Init creates and registers the custom metrics for kube-applier.
func (p *Prometheus) Init() {
	p.namespaceApplyCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "namespace_apply_count",
		Help: "Success metric for every namespace applied",
	},
		[]string{
			// Path of the file that was applied
			"namespace",
			// Result: true if the apply was successful, false otherwise
			"success",
		},
	)
	p.runLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "run_latency_seconds",
		Help: "Latency for completed apply runs",
	},
		[]string{
			// Result: true if the run was successful, false otherwise
			"success",
		},
	)

	p.logCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "log_count",
		Help: "Logs produced",
	},
		[]string{
			// Level
			"level",
		},
	)
	prometheus.MustRegister(p.namespaceApplyCount)
	prometheus.MustRegister(p.runLatency)
	prometheus.MustRegister(p.logCount)
}

// UpdateNamespaceSuccess increments the given namespace's Counter for either successful apply attempts or failed apply attempts.
func (p *Prometheus) UpdateNamespaceSuccess(file string, success bool) {
	p.namespaceApplyCount.With(prometheus.Labels{
		"namespace": filepath.Base(file), "success": strconv.FormatBool(success),
	}).Inc()
}

// UpdateRunLatency adds a data point (latency of the most recent run) to the run_latency_seconds Summary metric, with a tag indicating whether or not the run was successful.
func (p *Prometheus) UpdateRunLatency(runLatency float64, success bool) {
	p.runLatency.With(prometheus.Labels{
		"success": strconv.FormatBool(success),
	}).Observe(runLatency)
}

// UpdateLogCount increments logs counter for given log level
func (p *Prometheus) UpdateLogCount(level string) {
	p.logCount.With(prometheus.Labels{
		"level": level,
	}).Inc()
}
