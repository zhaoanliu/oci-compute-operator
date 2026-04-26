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
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"

	computev1alpha1 "github.com/zhaoanliu/oci-compute-operator/api/v1alpha1"
)

const (
	// ociInstanceFinalizer is added to OCIInstance resources to ensure
	// the OCI instance is deleted before the Kubernetes resource is removed
	ociInstanceFinalizer = "compute.nvcne-demo.io/finalizer"

	// requeueAfter is how long to wait before rechecking a provisioning instance
	requeueAfter = 30 * time.Second
)

// OCIInstanceReconciler reconciles a OCIInstance object
type OCIInstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=compute.nvcne-demo.io,resources=ociinstances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compute.nvcne-demo.io,resources=ociinstances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=compute.nvcne-demo.io,resources=ociinstances/finalizers,verbs=update

// Reconcile moves the current state of the OCIInstance closer to the desired state.
func (r *OCIInstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Step 1: Fetch the OCIInstance resource
	instance := &computev1alpha1.OCIInstance{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			// Resource was deleted before we could reconcile - nothing to do
			log.Info("OCIInstance not found, likely deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch OCIInstance")
		return ctrl.Result{}, err
	}

	// Step 2: Handle deletion - check if resource is being deleted
	if !instance.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, instance)
	}

	// Step 3: Ensure finalizer is present
	if !controllerutil.ContainsFinalizer(instance, ociInstanceFinalizer) {
		log.Info("Adding finalizer to OCIInstance")
		controllerutil.AddFinalizer(instance, ociInstanceFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		// Return here - the Update will trigger another reconcile
		return ctrl.Result{}, nil
	}

	// If already failed, stop retrying unless spec changed
	if instance.Status.Phase == computev1alpha1.InstancePhaseFailed &&
		instance.Status.ObservedGeneration == instance.Generation {
		log.Info("OCIInstance is in Failed phase, not retrying until spec changes")
		return ctrl.Result{}, nil
	}

	// Step 4: Create OCI compute client
	computeClient, err := r.newComputeClient()
	if err != nil {
		log.Error(err, "Failed to create OCI compute client")
		return r.setFailedStatus(ctx, instance, fmt.Sprintf("Failed to create OCI client: %v", err))
	}

	// Step 5: Check if OCI instance already exists
	if instance.Status.InstanceID != "" {
		return r.reconcileExisting(ctx, instance, computeClient)
	}

	// Step 6: Provision new OCI instance
	return r.reconcileNew(ctx, instance, computeClient)
}

// handleDeletion cleans up the OCI instance when the Kubernetes resource is deleted
func (r *OCIInstanceReconciler) handleDeletion(ctx context.Context, instance *computev1alpha1.OCIInstance) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(instance, ociInstanceFinalizer) {
		// No finalizer means nothing to clean up
		return ctrl.Result{}, nil
	}

	// If we have an OCI instance ID, terminate it first
	if instance.Status.InstanceID != "" {
		log.Info("Terminating OCI instance", "instanceId", instance.Status.InstanceID)

		computeClient, err := r.newComputeClient()
		if err != nil {
			log.Error(err, "Failed to create OCI client during deletion")
			return ctrl.Result{}, err
		}

		_, err = computeClient.TerminateInstance(ctx, core.TerminateInstanceRequest{
			InstanceId: &instance.Status.InstanceID,
		})
		if err != nil {
			log.Error(err, "Failed to terminate OCI instance")
			return ctrl.Result{RequeueAfter: requeueAfter}, err
		}

		log.Info("OCI instance termination initiated", "instanceId", instance.Status.InstanceID)
	}

	// Remove finalizer so Kubernetes can delete the resource
	log.Info("Removing finalizer from OCIInstance")
	controllerutil.RemoveFinalizer(instance, ociInstanceFinalizer)
	if err := r.Update(ctx, instance); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileNew provisions a brand new OCI compute instance
func (r *OCIInstanceReconciler) reconcileNew(ctx context.Context, instance *computev1alpha1.OCIInstance, computeClient core.ComputeClient) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Provisioning new OCI instance", "displayName", instance.Spec.DisplayName)

	// Set status to Provisioning
	instance.Status.Phase = computev1alpha1.InstancePhaseProvisioning
	if err := r.Status().Update(ctx, instance); err != nil {
		log.Error(err, "Failed to update status to Provisioning")
		return ctrl.Result{}, err
	}

	// Build the launch request
	launchDetails := core.LaunchInstanceDetails{
		CompartmentId:      &instance.Spec.CompartmentID,
		DisplayName:        &instance.Spec.DisplayName,
		Shape:              &instance.Spec.Shape,
		ImageId:            &instance.Spec.ImageID,
		AvailabilityDomain: &instance.Spec.AvailabilityDomain,
		CreateVnicDetails: &core.CreateVnicDetails{
			SubnetId: &instance.Spec.SubnetID,
		},
	}

	// Add flex shape config if both OCPUs and MemoryInGBs are specified
	// OCI requires both fields together for flex shapes
	if instance.Spec.OCPUs != nil && instance.Spec.MemoryInGBs != nil {
		ocpus, _ := strconv.ParseFloat(*instance.Spec.OCPUs, 32)
		memoryInGBs, _ := strconv.ParseFloat(*instance.Spec.MemoryInGBs, 32)
		ocpusFloat := float32(ocpus)
		memoryFloat := float32(memoryInGBs)
		launchDetails.ShapeConfig = &core.LaunchInstanceShapeConfigDetails{
			Ocpus:       &ocpusFloat,
			MemoryInGBs: &memoryFloat,
		}
	}

	// Add freeform tags if specified
	if len(instance.Spec.FreeformTags) > 0 {
		launchDetails.FreeformTags = instance.Spec.FreeformTags
	}

	// Call OCI API to launch the instance
	response, err := computeClient.LaunchInstance(ctx, core.LaunchInstanceRequest{
		LaunchInstanceDetails: launchDetails,
	})
	if err != nil {
		log.Error(err, "Failed to launch OCI instance")
		return r.setFailedStatus(ctx, instance, fmt.Sprintf("Failed to launch instance: %v", err))
	}

	// Update status with the new instance ID
	instance.Status.InstanceID = *response.Id
	instance.Status.Phase = computev1alpha1.InstancePhaseProvisioning
	instance.Status.ObservedGeneration = instance.Generation

	if err := r.Status().Update(ctx, instance); err != nil {
		log.Error(err, "Failed to update status with instance ID")
		return ctrl.Result{}, err
	}

	log.Info("OCI instance launch initiated", "instanceId", instance.Status.InstanceID)

	// Requeue to check provisioning status
	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// reconcileExisting checks and syncs the status of an already-provisioned OCI instance
func (r *OCIInstanceReconciler) reconcileExisting(ctx context.Context, instance *computev1alpha1.OCIInstance, computeClient core.ComputeClient) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Checking existing OCI instance", "instanceId", instance.Status.InstanceID)

	// Fetch current state from OCI
	response, err := computeClient.GetInstance(ctx, core.GetInstanceRequest{
		InstanceId: &instance.Status.InstanceID,
	})
	if err != nil {
		log.Error(err, "Failed to get OCI instance state")
		return ctrl.Result{RequeueAfter: requeueAfter}, err
	}

	ociInstance := response.Instance

	// Map OCI lifecycle state to our phase
	switch ociInstance.LifecycleState {
	case core.InstanceLifecycleStateRunning:
		instance.Status.Phase = computev1alpha1.InstancePhaseRunning
	case core.InstanceLifecycleStateProvisioning,
		core.InstanceLifecycleStateStarting:
		instance.Status.Phase = computev1alpha1.InstancePhaseProvisioning
		// Still provisioning - requeue to check again
		if err := r.Status().Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	case core.InstanceLifecycleStateTerminating:
		instance.Status.Phase = computev1alpha1.InstancePhaseTerminating
	case core.InstanceLifecycleStateTerminated:
		instance.Status.Phase = computev1alpha1.InstancePhaseTerminated
	default:
		instance.Status.Phase = computev1alpha1.InstancePhaseFailed
	}

	instance.Status.ObservedGeneration = instance.Generation

	if err := r.Status().Update(ctx, instance); err != nil {
		log.Error(err, "Failed to update instance status")
		return ctrl.Result{}, err
	}

	log.Info("OCI instance status synced", "phase", instance.Status.Phase)

	// If running, no need to requeue frequently
	if instance.Status.Phase == computev1alpha1.InstancePhaseRunning {
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	return ctrl.Result{RequeueAfter: requeueAfter}, nil
}

// setFailedStatus updates the instance status to Failed with a reason
func (r *OCIInstanceReconciler) setFailedStatus(ctx context.Context, instance *computev1alpha1.OCIInstance, reason string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Re-fetch the latest version to avoid conflict errors
	latest := &computev1alpha1.OCIInstance{}
	if err := r.Get(ctx, client.ObjectKeyFromObject(instance), latest); err != nil {
		log.Error(err, "Failed to re-fetch instance before setting failed status")
		return ctrl.Result{}, err
	}

	latest.Status.Phase = computev1alpha1.InstancePhaseFailed
	latest.Status.FailureReason = reason
	latest.Status.ObservedGeneration = latest.Generation

	if err := r.Status().Update(ctx, latest); err != nil {
		log.Error(err, "Failed to update failure status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// newComputeClient creates an OCI compute client using the default config provider
func (r *OCIInstanceReconciler) newComputeClient() (core.ComputeClient, error) {
	computeClient, err := core.NewComputeClientWithConfigurationProvider(
		common.DefaultConfigProvider(),
	)
	if err != nil {
		return core.ComputeClient{}, fmt.Errorf("failed to create OCI compute client: %w", err)
	}
	return computeClient, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCIInstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1alpha1.OCIInstance{}).
		Named("ociinstance").
		Complete(r)
}
