package log

import (
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/utilitywarehouse/kube-applier/metrics"
)

func TestLogger(t *testing.T) {
	InitLogger("debug")

	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	// Logging without keeping metrics
	Logger.Debug("this is Debug")
	Logger.Info("this is Info")
	Logger.Warn("this is Warn")
	Logger.Error("this is Error")

	// Start Metrics
	metrics := metrics.NewMockPrometheusInterface(mockCtrl)
	Logger.StartMetrics(metrics)

	// Logging with metrics inc - breaks if not expected
	metrics.EXPECT().UpdateLogCount("Debug").Times(1)
	Logger.Debug("this is Debug and metrics")
	metrics.EXPECT().UpdateLogCount("Info").Times(1)
	Logger.Info("this is Info and metrics")
	metrics.EXPECT().UpdateLogCount("Warn").Times(1)
	Logger.Warn("this is Warn and metrics")
	metrics.EXPECT().UpdateLogCount("Error").Times(1)
	Logger.Error("this is Error and metrics")
}
