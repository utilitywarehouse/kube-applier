// Code generated by MockGen. DO NOT EDIT.
// Source: metrics/prometheus.go

// Package metrics is a generated GoMock package.
package metrics

import (
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
)

// MockPrometheusInterface is a mock of PrometheusInterface interface
type MockPrometheusInterface struct {
	ctrl     *gomock.Controller
	recorder *MockPrometheusInterfaceMockRecorder
}

// MockPrometheusInterfaceMockRecorder is the mock recorder for MockPrometheusInterface
type MockPrometheusInterfaceMockRecorder struct {
	mock *MockPrometheusInterface
}

// NewMockPrometheusInterface creates a new mock instance
func NewMockPrometheusInterface(ctrl *gomock.Controller) *MockPrometheusInterface {
	mock := &MockPrometheusInterface{ctrl: ctrl}
	mock.recorder = &MockPrometheusInterfaceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockPrometheusInterface) EXPECT() *MockPrometheusInterfaceMockRecorder {
	return m.recorder
}

// UpdateKubectlExitCodeCount mocks base method
func (m *MockPrometheusInterface) UpdateKubectlExitCodeCount(arg0 string, arg1 int) {
	m.ctrl.Call(m, "UpdateKubectlExitCodeCount", arg0, arg1)
}

// UpdateKubectlExitCodeCount indicates an expected call of UpdateKubectlExitCodeCount
func (mr *MockPrometheusInterfaceMockRecorder) UpdateKubectlExitCodeCount(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateKubectlExitCodeCount", reflect.TypeOf((*MockPrometheusInterface)(nil).UpdateKubectlExitCodeCount), arg0, arg1)
}

// UpdateNamespaceSuccess mocks base method
func (m *MockPrometheusInterface) UpdateNamespaceSuccess(arg0 string, arg1 bool) {
	m.ctrl.Call(m, "UpdateNamespaceSuccess", arg0, arg1)
}

// UpdateNamespaceSuccess indicates an expected call of UpdateNamespaceSuccess
func (mr *MockPrometheusInterfaceMockRecorder) UpdateNamespaceSuccess(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateNamespaceSuccess", reflect.TypeOf((*MockPrometheusInterface)(nil).UpdateNamespaceSuccess), arg0, arg1)
}

// UpdateRunLatency mocks base method
func (m *MockPrometheusInterface) UpdateRunLatency(arg0 float64, arg1 bool) {
	m.ctrl.Call(m, "UpdateRunLatency", arg0, arg1)
}

// UpdateRunLatency indicates an expected call of UpdateRunLatency
func (mr *MockPrometheusInterfaceMockRecorder) UpdateRunLatency(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateRunLatency", reflect.TypeOf((*MockPrometheusInterface)(nil).UpdateRunLatency), arg0, arg1)
}

// UpdateResultSummary mocks base method
func (m *MockPrometheusInterface) UpdateResultSummary(arg0 map[string]string) {
	m.ctrl.Call(m, "UpdateResultSummary", arg0)
}

// UpdateResultSummary indicates an expected call of UpdateResultSummary
func (mr *MockPrometheusInterfaceMockRecorder) UpdateResultSummary(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateResultSummary", reflect.TypeOf((*MockPrometheusInterface)(nil).UpdateResultSummary), arg0)
}

// UpdateEnabled mocks base method
func (m *MockPrometheusInterface) UpdateEnabled(arg0 string, arg1 bool) {
	m.ctrl.Call(m, "UpdateEnabled", arg0, arg1)
}

// UpdateEnabled indicates an expected call of UpdateEnabled
func (mr *MockPrometheusInterfaceMockRecorder) UpdateEnabled(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateEnabled", reflect.TypeOf((*MockPrometheusInterface)(nil).UpdateEnabled), arg0, arg1)
}

// DeleteEnabled mocks base method
func (m *MockPrometheusInterface) DeleteEnabled(arg0 string) {
	m.ctrl.Call(m, "DeleteEnabled", arg0)
}

// DeleteEnabled indicates an expected call of DeleteEnabled
func (mr *MockPrometheusInterfaceMockRecorder) DeleteEnabled(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteEnabled", reflect.TypeOf((*MockPrometheusInterface)(nil).DeleteEnabled), arg0)
}

// UpdateDryRun mocks base method
func (m *MockPrometheusInterface) UpdateDryRun(arg0 string, arg1 bool) {
	m.ctrl.Call(m, "UpdateDryRun", arg0, arg1)
}

// UpdateDryRun indicates an expected call of UpdateDryRun
func (mr *MockPrometheusInterfaceMockRecorder) UpdateDryRun(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateDryRun", reflect.TypeOf((*MockPrometheusInterface)(nil).UpdateDryRun), arg0, arg1)
}

// DeleteDryRun mocks base method
func (m *MockPrometheusInterface) DeleteDryRun(arg0 string) {
	m.ctrl.Call(m, "DeleteDryRun", arg0)
}

// DeleteDryRun indicates an expected call of DeleteDryRun
func (mr *MockPrometheusInterfaceMockRecorder) DeleteDryRun(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteDryRun", reflect.TypeOf((*MockPrometheusInterface)(nil).DeleteDryRun), arg0)
}

// UpdatePrune mocks base method
func (m *MockPrometheusInterface) UpdatePrune(arg0 string, arg1 bool) {
	m.ctrl.Call(m, "UpdatePrune", arg0, arg1)
}

// UpdatePrune indicates an expected call of UpdatePrune
func (mr *MockPrometheusInterfaceMockRecorder) UpdatePrune(arg0, arg1 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePrune", reflect.TypeOf((*MockPrometheusInterface)(nil).UpdatePrune), arg0, arg1)
}

// DeletePrune mocks base method
func (m *MockPrometheusInterface) DeletePrune(arg0 string) {
	m.ctrl.Call(m, "DeletePrune", arg0)
}

// DeletePrune indicates an expected call of DeletePrune
func (mr *MockPrometheusInterfaceMockRecorder) DeletePrune(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeletePrune", reflect.TypeOf((*MockPrometheusInterface)(nil).DeletePrune), arg0)
}
