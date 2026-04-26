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

var _ = Describe("OCIInstance Controller OCI Integration", func() {
	const (
		ociTestNamespace = "default"
	)

	ctx := context.Background()

	// helper to build reconciler with injected mock
	newReconcilerWithMock := func(mock OCIComputeClient) *OCIInstanceReconciler {
		return &OCIInstanceReconciler{
			Client:        k8sClient,
			Scheme:        k8sClient.Scheme(),
			ComputeClient: mock,
		}
	}

	// helper to reconcile once with a given mock
	reconcileWithMock := func(name string, mock OCIComputeClient) error {
		_, err := newReconcilerWithMock(mock).Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      name,
				Namespace: ociTestNamespace,
			},
		})
		return err
	}

	// helper to fetch instance
	fetchOCIInstance := func(name string) *computev1alpha1.OCIInstance {
		instance := &computev1alpha1.OCIInstance{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      name,
			Namespace: ociTestNamespace,
		}, instance)).To(Succeed())
		return instance
	}

	Context("When OCI LaunchInstance succeeds", func() {
		const instanceName = "mock-launch-success"
		fakeInstanceID := "ocid1.instance.oc1..fakeinstance"

		BeforeEach(func() {
			By("creating the OCIInstance resource")
			instance := newTestOCIInstance(instanceName, ociTestNamespace)
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
		})

		AfterEach(func() {
			instance := &computev1alpha1.OCIInstance{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName, Namespace: ociTestNamespace}, instance)
			if err == nil {
				controllerutil.RemoveFinalizer(instance, ociInstanceFinalizer)
				Expect(k8sClient.Update(ctx, instance)).To(Succeed())
				Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
			}
		})

		It("should set InstanceID and phase to Provisioning after launch", func() {
			By("running first reconcile to add finalizer")
			mock := &MockComputeClient{}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("running second reconcile with successful LaunchInstance")
			mock.LaunchInstanceFn = func(ctx context.Context, req core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
				return core.LaunchInstanceResponse{
					Instance: core.Instance{
						Id:             &fakeInstanceID,
						LifecycleState: core.InstanceLifecycleStateProvisioning,
					},
				}, nil
			}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("checking InstanceID and phase are set correctly")
			instance := fetchOCIInstance(instanceName)
			Expect(instance.Status.InstanceID).To(Equal(fakeInstanceID))
			Expect(instance.Status.Phase).To(Equal(computev1alpha1.InstancePhaseProvisioning))
		})
	})

	Context("When OCI LaunchInstance fails", func() {
		const instanceName = "mock-launch-failure"

		BeforeEach(func() {
			By("creating the OCIInstance resource")
			instance := newTestOCIInstance(instanceName, ociTestNamespace)
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
		})

		AfterEach(func() {
			instance := &computev1alpha1.OCIInstance{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName, Namespace: ociTestNamespace}, instance)
			if err == nil {
				controllerutil.RemoveFinalizer(instance, ociInstanceFinalizer)
				Expect(k8sClient.Update(ctx, instance)).To(Succeed())
				Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
			}
		})

		It("should set phase to Failed with failure reason", func() {
			By("running first reconcile to add finalizer")
			mock := &MockComputeClient{}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("running second reconcile with failing LaunchInstance")
			mock.LaunchInstanceFn = func(ctx context.Context, req core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
				return core.LaunchInstanceResponse{}, fmt.Errorf("OCI: out of capacity")
			}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("checking phase is Failed with reason")
			instance := fetchOCIInstance(instanceName)
			Expect(instance.Status.Phase).To(Equal(computev1alpha1.InstancePhaseFailed))
			Expect(instance.Status.FailureReason).To(ContainSubstring("out of capacity"))
		})

		It("should not retry after entering Failed phase", func() {
			By("running first reconcile to add finalizer")
			mock := &MockComputeClient{}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("running second reconcile with failing LaunchInstance")
			callCount := 0
			mock.LaunchInstanceFn = func(ctx context.Context, req core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
				callCount++
				return core.LaunchInstanceResponse{}, fmt.Errorf("OCI: out of capacity")
			}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("running third reconcile — should not call OCI again")
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("verifying OCI was only called once")
			Expect(callCount).To(Equal(1))
		})
	})

	Context("When OCI GetInstance returns Running", func() {
		const instanceName = "mock-get-running"
		fakeInstanceID := "ocid1.instance.oc1..fakerunning"

		BeforeEach(func() {
			By("creating the OCIInstance resource with existing InstanceID in status")
			instance := newTestOCIInstance(instanceName, ociTestNamespace)
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())

			// Manually set the status to simulate already-provisioned instance
			instance.Status.InstanceID = fakeInstanceID
			instance.Status.Phase = computev1alpha1.InstancePhaseProvisioning
			Expect(k8sClient.Status().Update(ctx, instance)).To(Succeed())

			// Add finalizer manually
			instance.Finalizers = []string{ociInstanceFinalizer}
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())
		})

		AfterEach(func() {
			instance := &computev1alpha1.OCIInstance{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName, Namespace: ociTestNamespace}, instance)
			if err == nil {
				controllerutil.RemoveFinalizer(instance, ociInstanceFinalizer)
				Expect(k8sClient.Update(ctx, instance)).To(Succeed())
				Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
			}
		})

		It("should set phase to Running when OCI reports Running", func() {
			By("reconciling with GetInstance returning Running")
			mock := &MockComputeClient{
				GetInstanceFn: func(ctx context.Context, req core.GetInstanceRequest) (core.GetInstanceResponse, error) {
					return core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             &fakeInstanceID,
							LifecycleState: core.InstanceLifecycleStateRunning,
						},
					}, nil
				},
			}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("checking phase is Running")
			instance := fetchOCIInstance(instanceName)
			Expect(instance.Status.Phase).To(Equal(computev1alpha1.InstancePhaseRunning))
		})
	})

	Context("When deleting an OCIInstance with an existing OCI instance", func() {
		const instanceName = "mock-delete-instance"
		fakeInstanceID := "ocid1.instance.oc1..fakedelete"

		BeforeEach(func() {
			By("creating the OCIInstance resource")
			instance := newTestOCIInstance(instanceName, ociTestNamespace)
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())

			// Set up as already provisioned
			instance.Status.InstanceID = fakeInstanceID
			instance.Status.Phase = computev1alpha1.InstancePhaseRunning
			Expect(k8sClient.Status().Update(ctx, instance)).To(Succeed())

			// Add finalizer
			instance.Finalizers = []string{ociInstanceFinalizer}
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())
		})

		It("should call TerminateInstance and remove finalizer on deletion", func() {
			By("deleting the OCIInstance resource")
			instance := fetchOCIInstance(instanceName)
			Expect(k8sClient.Delete(ctx, instance)).To(Succeed())

			By("reconciling with TerminateInstance mock")
			terminateCalled := false
			mock := &MockComputeClient{
				TerminateInstanceFn: func(ctx context.Context, req core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error) {
					terminateCalled = true
					Expect(*req.InstanceId).To(Equal(fakeInstanceID))
					return core.TerminateInstanceResponse{}, nil
				},
			}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("verifying TerminateInstance was called")
			Expect(terminateCalled).To(BeTrue())

			By("verifying finalizer was removed and resource is gone")
			instance = &computev1alpha1.OCIInstance{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName, Namespace: ociTestNamespace}, instance)
			Expect(err).To(HaveOccurred()) // resource should be gone
		})
	})
	Context("When OCI GetInstance returns Provisioning", func() {
		const instanceName = "mock-get-provisioning"
		fakeInstanceID := "ocid1.instance.oc1..fakeprovisioning"

		BeforeEach(func() {
			By("creating the OCIInstance resource with existing InstanceID")
			instance := newTestOCIInstance(instanceName, ociTestNamespace)
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())

			instance.Status.InstanceID = fakeInstanceID
			instance.Status.Phase = computev1alpha1.InstancePhaseProvisioning
			Expect(k8sClient.Status().Update(ctx, instance)).To(Succeed())

			instance.Finalizers = []string{ociInstanceFinalizer}
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())
		})

		AfterEach(func() {
			instance := &computev1alpha1.OCIInstance{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName, Namespace: ociTestNamespace}, instance)
			if err == nil {
				controllerutil.RemoveFinalizer(instance, ociInstanceFinalizer)
				Expect(k8sClient.Update(ctx, instance)).To(Succeed())
				Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
			}
		})

		It("should keep phase as Provisioning and requeue", func() {
			By("reconciling with GetInstance returning Provisioning")
			mock := &MockComputeClient{
				GetInstanceFn: func(ctx context.Context, req core.GetInstanceRequest) (core.GetInstanceResponse, error) {
					return core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             &fakeInstanceID,
							LifecycleState: core.InstanceLifecycleStateProvisioning,
						},
					}, nil
				},
			}
			_, err := newReconcilerWithMock(mock).Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      instanceName,
					Namespace: ociTestNamespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			By("checking phase remains Provisioning")
			instance := fetchOCIInstance(instanceName)
			Expect(instance.Status.Phase).To(Equal(computev1alpha1.InstancePhaseProvisioning))
		})

		It("should set phase to Terminating when OCI reports Terminating", func() {
			By("reconciling with GetInstance returning Terminating")
			mock := &MockComputeClient{
				GetInstanceFn: func(ctx context.Context, req core.GetInstanceRequest) (core.GetInstanceResponse, error) {
					return core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             &fakeInstanceID,
							LifecycleState: core.InstanceLifecycleStateTerminating,
						},
					}, nil
				},
			}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("checking phase is Terminating")
			instance := fetchOCIInstance(instanceName)
			Expect(instance.Status.Phase).To(Equal(computev1alpha1.InstancePhaseTerminating))
		})

		It("should set phase to Terminated when OCI reports Terminated", func() {
			By("reconciling with GetInstance returning Terminated")
			mock := &MockComputeClient{
				GetInstanceFn: func(ctx context.Context, req core.GetInstanceRequest) (core.GetInstanceResponse, error) {
					return core.GetInstanceResponse{
						Instance: core.Instance{
							Id:             &fakeInstanceID,
							LifecycleState: core.InstanceLifecycleStateTerminated,
						},
					}, nil
				},
			}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("checking phase is Terminated")
			instance := fetchOCIInstance(instanceName)
			Expect(instance.Status.Phase).To(Equal(computev1alpha1.InstancePhaseTerminated))
		})
	})

	Context("When deleting an OCIInstance without an existing OCI instance", func() {
		const instanceName = "mock-delete-no-instance"

		BeforeEach(func() {
			By("creating the OCIInstance resource with no InstanceID")
			instance := newTestOCIInstance(instanceName, ociTestNamespace)
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())

			// Add finalizer but no InstanceID — simulates failed provisioning
			instance.Finalizers = []string{ociInstanceFinalizer}
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())
		})

		It("should remove finalizer without calling TerminateInstance", func() {
			By("deleting the OCIInstance resource")
			instance := fetchOCIInstance(instanceName)
			Expect(k8sClient.Delete(ctx, instance)).To(Succeed())

			By("reconciling — should skip OCI call since no InstanceID")
			terminateCalled := false
			mock := &MockComputeClient{
				TerminateInstanceFn: func(ctx context.Context, req core.TerminateInstanceRequest) (core.TerminateInstanceResponse, error) {
					terminateCalled = true
					return core.TerminateInstanceResponse{}, nil
				},
			}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("verifying TerminateInstance was NOT called")
			Expect(terminateCalled).To(BeFalse())

			By("verifying resource is gone")
			instance = &computev1alpha1.OCIInstance{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName, Namespace: ociTestNamespace}, instance)
			Expect(err).To(HaveOccurred())
		})
	})

	Context("When LaunchInstance is called with flex shape config", func() {
		const instanceName = "mock-flex-shape"
		fakeInstanceID := "ocid1.instance.oc1..fakeflex"

		BeforeEach(func() {
			By("creating the OCIInstance resource with OCPUs and MemoryInGBs")
			instance := newTestOCIInstance(instanceName, ociTestNamespace)
			ocpus := "2"
			memory := "16"
			instance.Spec.OCPUs = &ocpus
			instance.Spec.MemoryInGBs = &memory
			Expect(k8sClient.Create(ctx, instance)).To(Succeed())
		})

		AfterEach(func() {
			instance := &computev1alpha1.OCIInstance{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: instanceName, Namespace: ociTestNamespace}, instance)
			if err == nil {
				controllerutil.RemoveFinalizer(instance, ociInstanceFinalizer)
				Expect(k8sClient.Update(ctx, instance)).To(Succeed())
				Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
			}
		})

		It("should pass ShapeConfig with OCPUs and MemoryInGBs to OCI", func() {
			By("running first reconcile to add finalizer")
			mock := &MockComputeClient{}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("running second reconcile and capturing ShapeConfig")
			var capturedOCPUs float32
			var capturedMemory float32
			mock.LaunchInstanceFn = func(ctx context.Context, req core.LaunchInstanceRequest) (core.LaunchInstanceResponse, error) {
				if req.ShapeConfig != nil {
					capturedOCPUs = *req.ShapeConfig.Ocpus
					capturedMemory = *req.ShapeConfig.MemoryInGBs
				}
				return core.LaunchInstanceResponse{
					Instance: core.Instance{
						Id:             &fakeInstanceID,
						LifecycleState: core.InstanceLifecycleStateProvisioning,
					},
				}, nil
			}
			Expect(reconcileWithMock(instanceName, mock)).To(Succeed())

			By("verifying ShapeConfig was passed correctly")
			Expect(capturedOCPUs).To(Equal(float32(2)))
			Expect(capturedMemory).To(Equal(float32(16)))
		})
	})
})
