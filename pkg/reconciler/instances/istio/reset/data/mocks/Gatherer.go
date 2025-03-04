// Code generated by mockery v2.20.2. DO NOT EDIT.

package mocks

import (
	data "github.com/kyma-incubator/reconciler/pkg/reconciler/instances/istio/reset/data"
	kubernetes "k8s.io/client-go/kubernetes"

	mock "github.com/stretchr/testify/mock"

	retry "github.com/avast/retry-go"

	v1 "k8s.io/api/core/v1"

	zap "go.uber.org/zap"
)

// Gatherer is an autogenerated mock type for the Gatherer type
type Gatherer struct {
	mock.Mock
}

// GetAllPods provides a mock function with given fields: kubeClient, retryOpts
func (_m *Gatherer) GetAllPods(kubeClient kubernetes.Interface, retryOpts []retry.Option) (*v1.PodList, error) {
	ret := _m.Called(kubeClient, retryOpts)

	var r0 *v1.PodList
	var r1 error
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option) (*v1.PodList, error)); ok {
		return rf(kubeClient, retryOpts)
	}
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option) *v1.PodList); ok {
		r0 = rf(kubeClient, retryOpts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.PodList)
		}
	}

	if rf, ok := ret.Get(1).(func(kubernetes.Interface, []retry.Option) error); ok {
		r1 = rf(kubeClient, retryOpts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetInstalledIstioVersion provides a mock function with given fields: kubeClient, retryOpts, logger
func (_m *Gatherer) GetInstalledIstioVersion(kubeClient kubernetes.Interface, retryOpts []retry.Option, logger *zap.SugaredLogger) (string, error) {
	ret := _m.Called(kubeClient, retryOpts, logger)

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option, *zap.SugaredLogger) (string, error)); ok {
		return rf(kubeClient, retryOpts, logger)
	}
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option, *zap.SugaredLogger) string); ok {
		r0 = rf(kubeClient, retryOpts, logger)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(kubernetes.Interface, []retry.Option, *zap.SugaredLogger) error); ok {
		r1 = rf(kubeClient, retryOpts, logger)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetIstioCPPods provides a mock function with given fields: kubeClient, retryOpts
func (_m *Gatherer) GetIstioCPPods(kubeClient kubernetes.Interface, retryOpts []retry.Option) (*v1.PodList, error) {
	ret := _m.Called(kubeClient, retryOpts)

	var r0 *v1.PodList
	var r1 error
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option) (*v1.PodList, error)); ok {
		return rf(kubeClient, retryOpts)
	}
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option) *v1.PodList); ok {
		r0 = rf(kubeClient, retryOpts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*v1.PodList)
		}
	}

	if rf, ok := ret.Get(1).(func(kubernetes.Interface, []retry.Option) error); ok {
		r1 = rf(kubeClient, retryOpts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPodsForCNIChange provides a mock function with given fields: kubeClient, retryOpts, cniEnabled
func (_m *Gatherer) GetPodsForCNIChange(kubeClient kubernetes.Interface, retryOpts []retry.Option, cniEnabled bool) (v1.PodList, error) {
	ret := _m.Called(kubeClient, retryOpts, cniEnabled)

	var r0 v1.PodList
	var r1 error
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option, bool) (v1.PodList, error)); ok {
		return rf(kubeClient, retryOpts, cniEnabled)
	}
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option, bool) v1.PodList); ok {
		r0 = rf(kubeClient, retryOpts, cniEnabled)
	} else {
		r0 = ret.Get(0).(v1.PodList)
	}

	if rf, ok := ret.Get(1).(func(kubernetes.Interface, []retry.Option, bool) error); ok {
		r1 = rf(kubeClient, retryOpts, cniEnabled)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPodsWithDifferentImage provides a mock function with given fields: inputPodsList, image
func (_m *Gatherer) GetPodsWithDifferentImage(inputPodsList v1.PodList, image data.ExpectedImage) v1.PodList {
	ret := _m.Called(inputPodsList, image)

	var r0 v1.PodList
	if rf, ok := ret.Get(0).(func(v1.PodList, data.ExpectedImage) v1.PodList); ok {
		r0 = rf(inputPodsList, image)
	} else {
		r0 = ret.Get(0).(v1.PodList)
	}

	return r0
}

// GetPodsWithoutSidecar provides a mock function with given fields: kubeClient, retryOpts, sidecarInjectionEnabledbyDefault
func (_m *Gatherer) GetPodsWithoutSidecar(kubeClient kubernetes.Interface, retryOpts []retry.Option, sidecarInjectionEnabledbyDefault bool) (v1.PodList, error) {
	ret := _m.Called(kubeClient, retryOpts, sidecarInjectionEnabledbyDefault)

	var r0 v1.PodList
	var r1 error
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option, bool) (v1.PodList, error)); ok {
		return rf(kubeClient, retryOpts, sidecarInjectionEnabledbyDefault)
	}
	if rf, ok := ret.Get(0).(func(kubernetes.Interface, []retry.Option, bool) v1.PodList); ok {
		r0 = rf(kubeClient, retryOpts, sidecarInjectionEnabledbyDefault)
	} else {
		r0 = ret.Get(0).(v1.PodList)
	}

	if rf, ok := ret.Get(1).(func(kubernetes.Interface, []retry.Option, bool) error); ok {
		r1 = rf(kubeClient, retryOpts, sidecarInjectionEnabledbyDefault)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTNewGatherer interface {
	mock.TestingT
	Cleanup(func())
}

// NewGatherer creates a new instance of Gatherer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewGatherer(t mockConstructorTestingTNewGatherer) *Gatherer {
	mock := &Gatherer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
