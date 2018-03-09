package log

import (
	hclog "github.com/hashicorp/go-hclog"
	"github.com/utilitywarehouse/kube-applier/metrics"
)

type loggerInterface interface {
	StartMetrics(m metrics.PrometheusInterface)
	Debug(msg string, args ...interface{})
	Info(msg string, args ...interface{})
	Warn(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

type logger struct {
	hcLogger  hclog.Logger
	metricsOn bool
	Metrics   metrics.PrometheusInterface
}

var Logger *logger

func InitLogger(logLevel string) {

	Logger = &logger{}
	Logger.hcLogger = hclog.New(&hclog.LoggerOptions{
		Name:  "kube-applier",
		Level: hclog.LevelFromString(logLevel),
	})
	Logger.metricsOn = false
}

func (l *logger) StartMetrics(m metrics.PrometheusInterface) {
	l.metricsOn = true
	l.Metrics = m
}

func (l *logger) Debug(msg string, args ...interface{}) {
	if l.metricsOn {
		l.Metrics.UpdateLogCount("Debug")
	}
	l.hcLogger.Debug(msg, args...)
}

func (l *logger) Info(msg string, args ...interface{}) {
	if l.metricsOn {
		l.Metrics.UpdateLogCount("Info")
	}
	l.hcLogger.Info(msg, args...)
}

func (l *logger) Warn(msg string, args ...interface{}) {
	if l.metricsOn {
		l.Metrics.UpdateLogCount("Warn")
	}
	l.hcLogger.Warn(msg, args...)
}

func (l *logger) Error(msg string, args ...interface{}) {
	if l.metricsOn {
		l.Metrics.UpdateLogCount("Error")
	}
	l.hcLogger.Error(msg, args...)
}
