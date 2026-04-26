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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/oracle/oci-go-sdk/v65/core"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	computev1alpha1 "github.com/zhaoanliu/oci-compute-operator/api/v1alpha1"
)

var _ = Describe("OCISecurityPolicy Controller OCI Integration", func() {
	const (
		ociSPTestNamespace = "default"
	)

	ctx := context.Background()

	// helper to build reconciler with injected mock
	newSPReconcilerWithMock := func(mock OCIVirtualNetworkClient) *OCISecurityPolicyReconciler {
		return &OCISecurityPolicyReconciler{
			Client:               k8sClient,
			Scheme:               k8sClient.Scheme(),
			VirtualNetworkClient: mock,
		}
	}

	// helper to reconcile once with a given mock
	reconcileSPWithMock := func(name string, mock OCIVirtualNetworkClient) error {
		_, err := newSPReconcilerWithMock(mock).Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: ociSPTestNamespace,
			},
		})
		return err
	}

	// helper to fetch policy
	fetchOCISecurityPolicy := func(name string) *computev1alpha1.OCISecurityPolicy {
		policy := &computev1alpha1.OCISecurityPolicy{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: ociSPTestNamespace,
		}, policy)).To(Succeed())
		return policy
	}

	Context("When OCI CreateSecurityList succeeds", func() {
		const policyName = "mock-create-success"
		fakeSecurityListID := "ocid1.securitylist.oc1..fake"

		BeforeEach(func() {
			By("creating the OCISecurityPolicy resource")
			policy := newTestOCISecurityPolicy(policyName, ociSPTestNamespace)
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())
		})

		AfterEach(func() {
			policy := &computev1alpha1.OCISecurityPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName, Namespace: ociSPTestNamespace}, policy)
			if err == nil {
				controllerutil.RemoveFinalizer(policy, ociSecurityPolicyFinalizer)
				Expect(k8sClient.Update(ctx, policy)).To(Succeed())
				Expect(k8sClient.Delete(ctx, policy)).To(Succeed())
			}
		})

		It("should set SecurityListID and phase to Active after creation", func() {
			By("running first reconcile to add finalizer")
			mock := &MockVirtualNetworkClient{}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("running second reconcile with successful CreateSecurityList")
			mock.CreateSecurityListFn = func(ctx context.Context, req core.CreateSecurityListRequest) (core.CreateSecurityListResponse, error) {
				return core.CreateSecurityListResponse{
					SecurityList: core.SecurityList{
						Id:             &fakeSecurityListID,
						LifecycleState: core.SecurityListLifecycleStateAvailable,
					},
				}, nil
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("checking SecurityListID and phase are set correctly")
			policy := fetchOCISecurityPolicy(policyName)
			Expect(policy.Status.SecurityListID).To(Equal(fakeSecurityListID))
			Expect(policy.Status.Phase).To(Equal(computev1alpha1.SecurityPolicyPhaseActive))
		})

		It("should pass correct compartmentId and vcnId to OCI", func() {
			By("running first reconcile to add finalizer")
			mock := &MockVirtualNetworkClient{}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("running second reconcile and capturing request params")
			var capturedCompartmentID string
			var capturedVcnID string
			mock.CreateSecurityListFn = func(ctx context.Context, req core.CreateSecurityListRequest) (core.CreateSecurityListResponse, error) {
				capturedCompartmentID = *req.CompartmentId
				capturedVcnID = *req.VcnId
				return core.CreateSecurityListResponse{
					SecurityList: core.SecurityList{
						Id:             &fakeSecurityListID,
						LifecycleState: core.SecurityListLifecycleStateAvailable,
					},
				}, nil
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("verifying correct values were passed to OCI")
			Expect(capturedCompartmentID).To(Equal("ocid1.compartment.oc1..test"))
			Expect(capturedVcnID).To(Equal("ocid1.vcn.oc1..test"))
		})
	})

	Context("When OCI CreateSecurityList fails", func() {
		const policyName = "mock-create-failure"

		BeforeEach(func() {
			By("creating the OCISecurityPolicy resource")
			policy := newTestOCISecurityPolicy(policyName, ociSPTestNamespace)
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())
		})

		AfterEach(func() {
			policy := &computev1alpha1.OCISecurityPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName, Namespace: ociSPTestNamespace}, policy)
			if err == nil {
				controllerutil.RemoveFinalizer(policy, ociSecurityPolicyFinalizer)
				Expect(k8sClient.Update(ctx, policy)).To(Succeed())
				Expect(k8sClient.Delete(ctx, policy)).To(Succeed())
			}
		})

		It("should set phase to Failed with failure reason", func() {
			By("running first reconcile to add finalizer")
			mock := &MockVirtualNetworkClient{}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("running second reconcile with failing CreateSecurityList")
			mock.CreateSecurityListFn = func(ctx context.Context, req core.CreateSecurityListRequest) (core.CreateSecurityListResponse, error) {
				return core.CreateSecurityListResponse{}, fmt.Errorf("OCI: quota exceeded")
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("checking phase is Failed with reason")
			policy := fetchOCISecurityPolicy(policyName)
			Expect(policy.Status.Phase).To(Equal(computev1alpha1.SecurityPolicyPhaseFailed))
			Expect(policy.Status.FailureReason).To(ContainSubstring("quota exceeded"))
		})

		It("should not retry after entering Failed phase", func() {
			By("running first reconcile to add finalizer")
			mock := &MockVirtualNetworkClient{}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("running second reconcile with failing CreateSecurityList")
			callCount := 0
			mock.CreateSecurityListFn = func(ctx context.Context, req core.CreateSecurityListRequest) (core.CreateSecurityListResponse, error) {
				callCount++
				return core.CreateSecurityListResponse{}, fmt.Errorf("OCI: quota exceeded")
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("running third reconcile — should not call OCI again")
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("verifying OCI was only called once")
			Expect(callCount).To(Equal(1))
		})
	})

	Context("When OCI GetSecurityList returns Available", func() {
		const policyName = "mock-get-active"
		fakeSecurityListID := "ocid1.securitylist.oc1..fakeactive"

		BeforeEach(func() {
			By("creating the OCISecurityPolicy resource with existing SecurityListID")
			policy := newTestOCISecurityPolicy(policyName, ociSPTestNamespace)
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			// Simulate already-created security list
			policy.Status.SecurityListID = fakeSecurityListID
			policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseCreating
			Expect(k8sClient.Status().Update(ctx, policy)).To(Succeed())

			// Add finalizer manually
			policy.Finalizers = []string{ociSecurityPolicyFinalizer}
			Expect(k8sClient.Update(ctx, policy)).To(Succeed())
		})

		AfterEach(func() {
			policy := &computev1alpha1.OCISecurityPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName, Namespace: ociSPTestNamespace}, policy)
			if err == nil {
				controllerutil.RemoveFinalizer(policy, ociSecurityPolicyFinalizer)
				Expect(k8sClient.Update(ctx, policy)).To(Succeed())
				Expect(k8sClient.Delete(ctx, policy)).To(Succeed())
			}
		})

		It("should set phase to Active when OCI reports Available", func() {
			By("reconciling with GetSecurityList returning Available")
			mock := &MockVirtualNetworkClient{
				GetSecurityListFn: func(ctx context.Context, req core.GetSecurityListRequest) (core.GetSecurityListResponse, error) {
					return core.GetSecurityListResponse{
						SecurityList: core.SecurityList{
							Id:             &fakeSecurityListID,
							LifecycleState: core.SecurityListLifecycleStateAvailable,
						},
					}, nil
				},
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("checking phase is Active")
			policy := fetchOCISecurityPolicy(policyName)
			Expect(policy.Status.Phase).To(Equal(computev1alpha1.SecurityPolicyPhaseActive))
		})
	})

	Context("When deleting an OCISecurityPolicy with an existing security list", func() {
		const policyName = "mock-delete-policy"
		fakeSecurityListID := "ocid1.securitylist.oc1..fakedelete"

		BeforeEach(func() {
			By("creating the OCISecurityPolicy resource")
			policy := newTestOCISecurityPolicy(policyName, ociSPTestNamespace)
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			// Set up as already created
			policy.Status.SecurityListID = fakeSecurityListID
			policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseActive
			Expect(k8sClient.Status().Update(ctx, policy)).To(Succeed())

			// Add finalizer
			policy.Finalizers = []string{ociSecurityPolicyFinalizer}
			Expect(k8sClient.Update(ctx, policy)).To(Succeed())
		})

		It("should call DeleteSecurityList and remove finalizer on deletion", func() {
			By("deleting the OCISecurityPolicy resource")
			policy := fetchOCISecurityPolicy(policyName)
			Expect(k8sClient.Delete(ctx, policy)).To(Succeed())

			By("reconciling with DeleteSecurityList mock")
			deleteCalled := false
			mock := &MockVirtualNetworkClient{
				DeleteSecurityListFn: func(ctx context.Context, req core.DeleteSecurityListRequest) (core.DeleteSecurityListResponse, error) {
					deleteCalled = true
					Expect(*req.SecurityListId).To(Equal(fakeSecurityListID))
					return core.DeleteSecurityListResponse{}, nil
				},
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("verifying DeleteSecurityList was called")
			Expect(deleteCalled).To(BeTrue())

			By("verifying finalizer was removed and resource is gone")
			policy = &computev1alpha1.OCISecurityPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName, Namespace: ociSPTestNamespace}, policy)
			Expect(err).To(HaveOccurred())
		})
	})
	Context("When OCI GetSecurityList returns Provisioning", func() {
		const policyName = "mock-get-provisioning-sp"
		fakeSecurityListID := "ocid1.securitylist.oc1..fakeprovisioning"

		BeforeEach(func() {
			By("creating the OCISecurityPolicy resource with existing SecurityListID")
			policy := newTestOCISecurityPolicy(policyName, ociSPTestNamespace)
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			policy.Status.SecurityListID = fakeSecurityListID
			policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseCreating
			Expect(k8sClient.Status().Update(ctx, policy)).To(Succeed())

			policy.Finalizers = []string{ociSecurityPolicyFinalizer}
			Expect(k8sClient.Update(ctx, policy)).To(Succeed())
		})

		AfterEach(func() {
			policy := &computev1alpha1.OCISecurityPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName, Namespace: ociSPTestNamespace}, policy)
			if err == nil {
				controllerutil.RemoveFinalizer(policy, ociSecurityPolicyFinalizer)
				Expect(k8sClient.Update(ctx, policy)).To(Succeed())
				Expect(k8sClient.Delete(ctx, policy)).To(Succeed())
			}
		})

		It("should keep phase as Creating when OCI reports Provisioning", func() {
			By("reconciling with GetSecurityList returning Provisioning")
			mock := &MockVirtualNetworkClient{
				GetSecurityListFn: func(ctx context.Context, req core.GetSecurityListRequest) (core.GetSecurityListResponse, error) {
					return core.GetSecurityListResponse{
						SecurityList: core.SecurityList{
							Id:             &fakeSecurityListID,
							LifecycleState: core.SecurityListLifecycleStateProvisioning,
						},
					}, nil
				},
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("checking phase remains Creating")
			policy := fetchOCISecurityPolicy(policyName)
			Expect(policy.Status.Phase).To(Equal(computev1alpha1.SecurityPolicyPhaseCreating))
		})

		It("should set phase to Deleting when OCI reports Terminating", func() {
			By("reconciling with GetSecurityList returning Terminating")
			mock := &MockVirtualNetworkClient{
				GetSecurityListFn: func(ctx context.Context, req core.GetSecurityListRequest) (core.GetSecurityListResponse, error) {
					return core.GetSecurityListResponse{
						SecurityList: core.SecurityList{
							Id:             &fakeSecurityListID,
							LifecycleState: core.SecurityListLifecycleStateTerminating,
						},
					}, nil
				},
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("checking phase is Deleting")
			policy := fetchOCISecurityPolicy(policyName)
			Expect(policy.Status.Phase).To(Equal(computev1alpha1.SecurityPolicyPhaseDeleting))
		})

		It("should set phase to Deleted when OCI reports Terminated", func() {
			By("reconciling with GetSecurityList returning Terminated")
			mock := &MockVirtualNetworkClient{
				GetSecurityListFn: func(ctx context.Context, req core.GetSecurityListRequest) (core.GetSecurityListResponse, error) {
					return core.GetSecurityListResponse{
						SecurityList: core.SecurityList{
							Id:             &fakeSecurityListID,
							LifecycleState: core.SecurityListLifecycleStateTerminated,
						},
					}, nil
				},
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("checking phase is Deleted")
			policy := fetchOCISecurityPolicy(policyName)
			Expect(policy.Status.Phase).To(Equal(computev1alpha1.SecurityPolicyPhaseDeleted))
		})
	})

	Context("When deleting an OCISecurityPolicy without an existing security list", func() {
		const policyName = "mock-delete-no-list"

		BeforeEach(func() {
			By("creating the OCISecurityPolicy with no SecurityListID")
			policy := newTestOCISecurityPolicy(policyName, ociSPTestNamespace)
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())

			// Add finalizer but no SecurityListID
			policy.Finalizers = []string{ociSecurityPolicyFinalizer}
			Expect(k8sClient.Update(ctx, policy)).To(Succeed())
		})

		It("should remove finalizer without calling DeleteSecurityList", func() {
			By("deleting the OCISecurityPolicy resource")
			policy := fetchOCISecurityPolicy(policyName)
			Expect(k8sClient.Delete(ctx, policy)).To(Succeed())

			By("reconciling — should skip OCI call since no SecurityListID")
			deleteCalled := false
			mock := &MockVirtualNetworkClient{
				DeleteSecurityListFn: func(ctx context.Context, req core.DeleteSecurityListRequest) (core.DeleteSecurityListResponse, error) {
					deleteCalled = true
					return core.DeleteSecurityListResponse{}, nil
				},
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("verifying DeleteSecurityList was NOT called")
			Expect(deleteCalled).To(BeFalse())

			By("verifying resource is gone")
			policy = &computev1alpha1.OCISecurityPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName, Namespace: ociSPTestNamespace}, policy)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When CreateSecurityList is called with egress rules", func() {
		const policyName = "mock-egress-rules"
		fakeSecurityListID := "ocid1.securitylist.oc1..fakeegress"

		BeforeEach(func() {
			By("creating an OCISecurityPolicy with egress rules")
			policy := newTestOCISecurityPolicy(policyName, ociSPTestNamespace)
			policy.Spec.Rules = []computev1alpha1.SecurityRule{
				{
					Direction:   computev1alpha1.SecurityRuleDirectionEgress,
					Protocol:    "6",
					Destination: "0.0.0.0/0",
					MinPort:     int32Ptr(443),
					MaxPort:     int32Ptr(443),
					Description: "Allow HTTPS outbound",
				},
			}
			Expect(k8sClient.Create(ctx, policy)).To(Succeed())
		})

		AfterEach(func() {
			policy := &computev1alpha1.OCISecurityPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: policyName, Namespace: ociSPTestNamespace}, policy)
			if err == nil {
				controllerutil.RemoveFinalizer(policy, ociSecurityPolicyFinalizer)
				Expect(k8sClient.Update(ctx, policy)).To(Succeed())
				Expect(k8sClient.Delete(ctx, policy)).To(Succeed())
			}
		})

		It("should pass egress rules correctly to OCI", func() {
			By("running first reconcile to add finalizer")
			mock := &MockVirtualNetworkClient{}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("running second reconcile and capturing egress rules")
			var capturedEgressRules []core.EgressSecurityRule
			var capturedIngressRules []core.IngressSecurityRule
			mock.CreateSecurityListFn = func(ctx context.Context, req core.CreateSecurityListRequest) (core.CreateSecurityListResponse, error) {
				capturedEgressRules = req.EgressSecurityRules
				capturedIngressRules = req.IngressSecurityRules
				return core.CreateSecurityListResponse{
					SecurityList: core.SecurityList{
						Id:             &fakeSecurityListID,
						LifecycleState: core.SecurityListLifecycleStateAvailable,
					},
				}, nil
			}
			Expect(reconcileSPWithMock(policyName, mock)).To(Succeed())

			By("verifying egress rules were passed and ingress rules are empty")
			Expect(capturedEgressRules).To(HaveLen(1))
			Expect(capturedIngressRules).To(BeEmpty())
			Expect(*capturedEgressRules[0].Destination).To(Equal("0.0.0.0/0"))
			Expect(*capturedEgressRules[0].Protocol).To(Equal("6"))
		})
	})
})
