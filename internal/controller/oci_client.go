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

// OCIComputeClient defines the OCI compute operations used by OCIInstanceReconciler.
// Using an interface instead of the concrete SDK type allows us to inject
// mock implementations in tests without making real OCI API calls.
type OCIComputeClient interface {
	LaunchInstance(ctx context.Context, request core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error)
	GetInstance(ctx context.Context, request core.GetInstanceRequest) (core.GetInstanceResponse, error)
	TerminateInstance(ctx context.Context, request core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error)
}

// OCIVirtualNetworkClient defines the OCI VCN operations used by OCISecurityPolicyReconciler.
// Same rationale as OCIComputeClient — interface enables test injection.
type OCIVirtualNetworkClient interface {
	CreateSecurityList(ctx context.Context, request core.CreateSecurityListRequest) (core.CreateSecurityListResponse, error)
	GetSecurityList(ctx context.Context, request core.GetSecurityListRequest) (core.GetSecurityListResponse, error)
	DeleteSecurityList(ctx context.Context, request core.DeleteSecurityListRequest) (core.DeleteSecurityListResponse, error)
}
