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
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OCIInstanceSpec defines the desired state of OCIInstance
type OCIInstanceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html

	// CompartmentID is the OCID of the compartment where the instance will be created.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	CompartmentID string `json:"compartmentId"`

	// DisplayName is the human-readable name for the OCI instance.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	DisplayName string `json:"displayName"`

	// Shape is the OCI compute shape (e.g. "VM.Standard.E4.Flex").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	Shape string `json:"shape"`

	// ImageID is the OCID of the image to use for the instance.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	ImageID string `json:"imageId"`

	// AvailabilityDomain is the AD where the instance will be placed.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	AvailabilityDomain string `json:"availabilityDomain"`

	// SubnetID is the OCID of the subnet to attach the instance to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	SubnetID string `json:"subnetId"`

	// OCPUs is the number of OCPUs for flex shapes, serialized as string (e.g. "4").
	// +optional
	OCPUs *string `json:"ocpus,omitempty"`

	// MemoryInGBs is the amount of memory in GBs for flex shapes, serialized as string (e.g. "64").
	// +optional
	MemoryInGBs *string `json:"memoryInGBs,omitempty"`

	// FreeformTags are key-value pairs you can attach to the instance.
	// +optional
	FreeformTags map[string]string `json:"freeformTags,omitempty"`
}

// InstancePhase represents the lifecycle phase of the OCI instance
// +kubebuilder:validation:Enum=Pending;Provisioning;Running;Terminating;Terminated;Failed
type InstancePhase string

const (
	// InstancePhasePending means the request has been received but not yet acted on
	InstancePhasePending InstancePhase = "Pending"

	// InstancePhaseProvisioning means OCI is creating the instance
	InstancePhaseProvisioning InstancePhase = "Provisioning"

	// InstancePhaseRunning means the instance is up and running
	InstancePhaseRunning InstancePhase = "Running"

	// InstancePhaseTerminating means the instance is being deleted
	InstancePhaseTerminating InstancePhase = "Terminating"

	// InstancePhaseTerminated means the instance has been deleted
	InstancePhaseTerminated InstancePhase = "Terminated"

	// InstancePhaseFailed means something went wrong
	InstancePhaseFailed InstancePhase = "Failed"
)

// OCIInstanceStatus defines the observed state of OCIInstance
type OCIInstanceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// For Kubernetes API conventions, see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties

	// conditions represent the current state of the OCIInstance resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.

	// InstanceID is the OCID of the provisioned OCI instance.
	// +optional
	InstanceID string `json:"instanceId,omitempty"`

	// Phase represents the current lifecycle phase of the instance.
	// +optional
	Phase InstancePhase `json:"phase,omitempty"`

	// PrivateIP is the private IP address assigned to the instance.
	// +optional
	PrivateIP string `json:"privateIp,omitempty"`

	// PublicIP is the public IP address assigned to the instance, if any.
	// +optional
	PublicIP string `json:"publicIp,omitempty"`

	// FailureReason contains the error message if the instance failed.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// ObservedGeneration is the last generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state of the OCIInstance resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="InstanceID",type="string",JSONPath=".status.instanceId"
// +kubebuilder:printcolumn:name="PrivateIP",type="string",JSONPath=".status.privateIp"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OCIInstance is the Schema for the ociinstances API
type OCIInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCIInstanceSpec   `json:"spec,omitempty"`
	Status OCIInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OCIInstanceList contains a list of OCIInstance
type OCIInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCIInstance `json:"items"`
}

// SetFailedStatus sets the phase to Failed with the given reason.
func (o *OCIInstance) SetFailedStatus(reason string) {
	o.Status.Phase = InstancePhaseFailed
	o.Status.FailureReason = reason
}

// SetObservedGeneration sets the observed generation on the status.
func (o *OCIInstance) SetObservedGeneration(generation int64) {
	o.Status.ObservedGeneration = generation
}

func init() {
	SchemeBuilder.Register(&OCIInstance{}, &OCIInstanceList{})
}
