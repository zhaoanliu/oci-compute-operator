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
	// ociSecurityPolicyFinalizer ensures the OCI security list is deleted
	// before the Kubernetes resource is removed
	ociSecurityPolicyFinalizer = "compute.nvcne-demo.io/security-policy-finalizer"

	// securityPolicyRequeueAfter is how long to wait before rechecking
	securityPolicyRequeueAfter = 30 * time.Second
)

// OCISecurityPolicyReconciler reconciles a OCISecurityPolicy object
type OCISecurityPolicyReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=compute.nvcne-demo.io,resources=ocisecuritypolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=compute.nvcne-demo.io,resources=ocisecuritypolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=compute.nvcne-demo.io,resources=ocisecuritypolicies/finalizers,verbs=update

// Reconcile moves the current state of the OCISecurityPolicy closer to desired state.
func (r *OCISecurityPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Step 1: Fetch the OCISecurityPolicy resource
	policy := &computev1alpha1.OCISecurityPolicy{}
	if err := r.Get(ctx, req.NamespacedName, policy); err != nil {
		if errors.IsNotFound(err) {
			log.Info("OCISecurityPolicy not found, likely deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to fetch OCISecurityPolicy")
		return ctrl.Result{}, err
	}

	// Step 2: Handle deletion
	if !policy.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, policy)
	}

	// Step 3: Ensure finalizer is present
	if !controllerutil.ContainsFinalizer(policy, ociSecurityPolicyFinalizer) {
		log.Info("Adding finalizer to OCISecurityPolicy")
		controllerutil.AddFinalizer(policy, ociSecurityPolicyFinalizer)
		if err := r.Update(ctx, policy); err != nil {
			log.Error(err, "Failed to add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// Step 4: Create OCI virtual network client
	vcnClient, err := r.newVirtualNetworkClient()
	if err != nil {
		log.Error(err, "Failed to create OCI VCN client")
		return r.setFailedStatus(ctx, policy, fmt.Sprintf("Failed to create OCI client: %v", err))
	}

	// Step 5: Check if security list already exists
	if policy.Status.SecurityListID != "" {
		return r.reconcileExisting(ctx, policy, vcnClient)
	}

	// Step 6: Create new security list
	return r.reconcileNew(ctx, policy, vcnClient)
}

// handleDeletion cleans up the OCI security list when the Kubernetes resource is deleted
func (r *OCISecurityPolicyReconciler) handleDeletion(ctx context.Context, policy *computev1alpha1.OCISecurityPolicy) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(policy, ociSecurityPolicyFinalizer) {
		return ctrl.Result{}, nil
	}

	// Delete the OCI security list if it exists
	if policy.Status.SecurityListID != "" {
		log.Info("Deleting OCI security list", "securityListId", policy.Status.SecurityListID)

		vcnClient, err := r.newVirtualNetworkClient()
		if err != nil {
			log.Error(err, "Failed to create OCI VCN client during deletion")
			return ctrl.Result{}, err
		}

		_, err = vcnClient.DeleteSecurityList(ctx, core.DeleteSecurityListRequest{
			SecurityListId: &policy.Status.SecurityListID,
		})
		if err != nil {
			log.Error(err, "Failed to delete OCI security list")
			return ctrl.Result{RequeueAfter: securityPolicyRequeueAfter}, err
		}

		log.Info("OCI security list deleted", "securityListId", policy.Status.SecurityListID)
	}

	// Remove finalizer so Kubernetes can complete deletion
	log.Info("Removing finalizer from OCISecurityPolicy")
	controllerutil.RemoveFinalizer(policy, ociSecurityPolicyFinalizer)
	if err := r.Update(ctx, policy); err != nil {
		log.Error(err, "Failed to remove finalizer")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// reconcileNew creates a brand new OCI security list
func (r *OCISecurityPolicyReconciler) reconcileNew(ctx context.Context, policy *computev1alpha1.OCISecurityPolicy, vcnClient core.VirtualNetworkClient) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Creating new OCI security list", "displayName", policy.Spec.DisplayName)

	// Set status to Creating
	policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseCreating
	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update status to Creating")
		return ctrl.Result{}, err
	}

	// Convert our rules to OCI ingress/egress rules
	ingressRules, egressRules := r.convertRules(policy.Spec.Rules)

	// Call OCI API to create the security list
	response, err := vcnClient.CreateSecurityList(ctx, core.CreateSecurityListRequest{
		CreateSecurityListDetails: core.CreateSecurityListDetails{
			CompartmentId:        &policy.Spec.CompartmentID,
			VcnId:                &policy.Spec.VcnID,
			DisplayName:          &policy.Spec.DisplayName,
			IngressSecurityRules: ingressRules,
			EgressSecurityRules:  egressRules,
			FreeformTags:         policy.Spec.FreeformTags,
		},
	})
	if err != nil {
		log.Error(err, "Failed to create OCI security list")
		return r.setFailedStatus(ctx, policy, fmt.Sprintf("Failed to create security list: %v", err))
	}

	// Update status with new security list ID
	policy.Status.SecurityListID = *response.Id
	policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseActive
	policy.Status.ObservedGeneration = policy.Generation

	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update status with security list ID")
		return ctrl.Result{}, err
	}

	log.Info("OCI security list created", "securityListId", policy.Status.SecurityListID)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// reconcileExisting syncs the state of an already-created OCI security list
func (r *OCISecurityPolicyReconciler) reconcileExisting(ctx context.Context, policy *computev1alpha1.OCISecurityPolicy, vcnClient core.VirtualNetworkClient) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Checking existing OCI security list", "securityListId", policy.Status.SecurityListID)

	// Fetch current state from OCI
	response, err := vcnClient.GetSecurityList(ctx, core.GetSecurityListRequest{
		SecurityListId: &policy.Status.SecurityListID,
	})
	if err != nil {
		log.Error(err, "Failed to get OCI security list")
		return ctrl.Result{RequeueAfter: securityPolicyRequeueAfter}, err
	}

	// Map OCI lifecycle state to our phase
	switch response.LifecycleState {
	case core.SecurityListLifecycleStateAvailable:
		policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseActive
	case core.SecurityListLifecycleStateProvisioning:
		policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseCreating
		if err := r.Status().Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: securityPolicyRequeueAfter}, nil
	case core.SecurityListLifecycleStateTerminating:
		policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseDeleting
	case core.SecurityListLifecycleStateTerminated:
		policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseDeleted
	default:
		policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseFailed
	}

	policy.Status.ObservedGeneration = policy.Generation

	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update security policy status")
		return ctrl.Result{}, err
	}

	log.Info("OCI security list status synced", "phase", policy.Status.Phase)
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// convertRules converts our CRD rules to OCI ingress and egress rule types
func (r *OCISecurityPolicyReconciler) convertRules(rules []computev1alpha1.SecurityRule) ([]core.IngressSecurityRule, []core.EgressSecurityRule) {
	var ingressRules []core.IngressSecurityRule
	var egressRules []core.EgressSecurityRule

	for _, rule := range rules {
		ruleCopy := rule
		if ruleCopy.Direction == computev1alpha1.SecurityRuleDirectionIngress {
			ingressRule := core.IngressSecurityRule{
				Protocol:    &ruleCopy.Protocol,
				Source:      &ruleCopy.Source,
				Description: &ruleCopy.Description,
			}
			// Add TCP options if ports are specified
			if ruleCopy.MinPort != nil && ruleCopy.MaxPort != nil {
				minPort := int(*ruleCopy.MinPort)
				maxPort := int(*ruleCopy.MaxPort)
				ingressRule.TcpOptions = &core.TcpOptions{
					DestinationPortRange: &core.PortRange{
						Min: &minPort,
						Max: &maxPort,
					},
				}
			}
			ingressRules = append(ingressRules, ingressRule)
		} else {
			egressRule := core.EgressSecurityRule{
				Protocol:    &ruleCopy.Protocol,
				Destination: &ruleCopy.Destination,
				Description: &ruleCopy.Description,
			}
			if ruleCopy.MinPort != nil && ruleCopy.MaxPort != nil {
				minPort := int(*ruleCopy.MinPort)
				maxPort := int(*ruleCopy.MaxPort)
				egressRule.TcpOptions = &core.TcpOptions{
					DestinationPortRange: &core.PortRange{
						Min: &minPort,
						Max: &maxPort,
					},
				}
			}
			egressRules = append(egressRules, egressRule)
		}
	}

	return ingressRules, egressRules
}

// setFailedStatus updates the policy status to Failed with a reason
func (r *OCISecurityPolicyReconciler) setFailedStatus(ctx context.Context, policy *computev1alpha1.OCISecurityPolicy, reason string) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	policy.Status.Phase = computev1alpha1.SecurityPolicyPhaseFailed
	policy.Status.FailureReason = reason
	if err := r.Status().Update(ctx, policy); err != nil {
		log.Error(err, "Failed to update failure status")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// newVirtualNetworkClient creates an OCI VCN client using the default config provider
func (r *OCISecurityPolicyReconciler) newVirtualNetworkClient() (core.VirtualNetworkClient, error) {
	vcnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(
		common.DefaultConfigProvider(),
	)
	if err != nil {
		return core.VirtualNetworkClient{}, fmt.Errorf("failed to create OCI VCN client: %w", err)
	}
	return vcnClient, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OCISecurityPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1alpha1.OCISecurityPolicy{}).
		Named("ocisecuritypolicy").
		Complete(r)
}
