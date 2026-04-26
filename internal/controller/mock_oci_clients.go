/*
Copyright 2026.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package controller

import (
	"context"

	"github.com/oracle/oci-go-sdk/v65/core"
)

// MockComputeClient is a fake OCI compute client for use in tests.
// Each operation is a function field that can be set per test to simulate
// different OCI responses without making real API calls.
type MockComputeClient struct {
	LaunchInstanceFn    func(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error)
	GetInstanceFn       func(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error)
	TerminateInstanceFn func(ctx context.Context, request core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error)
}

func (m *MockComputeClient) LaunchInstance(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
	if m.LaunchInstanceFn != nil {
		return m.LaunchInstanceFn(ctx, request)
	}
	return core.LaunchInstanceResponse{}, nil
}

func (m *MockComputeClient) GetInstance(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error) {
	if m.GetInstanceFn != nil {
		return m.GetInstanceFn(ctx, request)
	}
	return core.GetInstanceResponse{}, nil
}

func (m *MockComputeClient) TerminateInstance(ctx context.Context, request core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error) {
	if m.TerminateInstanceFn != nil {
		return m.TerminateInstanceFn(ctx, request)
	}
	return core.TerminateInstanceResponse{}, nil
}

// MockVirtualNetworkClient is a fake OCI VCN client for use in tests.
type MockVirtualNetworkClient struct {
	CreateSecurityListFn func(ctx context.Context, request core.CreateSecurityListRequest) (core.CreateSecurityListResponse, error)
	GetSecurityListFn    func(ctx context.Context, request core.GetSecurityListRequest) (core.GetSecurityListResponse, error)
	DeleteSecurityListFn func(ctx context.Context, request core.DeleteSecurityListRequest) (core.DeleteSecurityListResponse, error)
}

func (m *MockVirtualNetworkClient) CreateSecurityList(ctx context.Context, request core.CreateSecurityListRequest) (core.CreateSecurityListResponse, error) {
	if m.CreateSecurityListFn != nil {
		return m.CreateSecurityListFn(ctx, request)
	}
	return core.CreateSecurityListResponse{}, nil
}

func (m *MockVirtualNetworkClient) GetSecurityList(ctx context.Context, request core.GetSecurityListRequest) (core.GetSecurityListResponse, error) {
	if m.GetSecurityListFn != nil {
		return m.GetSecurityListFn(ctx, request)
	}
	return core.GetSecurityListResponse{}, nil
}

func (m *MockVirtualNetworkClient) DeleteSecurityList(ctx context.Context, request core.DeleteSecurityListRequest) (core.DeleteSecurityListResponse, error) {
	if m.DeleteSecurityListFn != nil {
		return m.DeleteSecurityListFn(ctx, request)
	}
	return core.DeleteSecurityListResponse{}, nil
}
