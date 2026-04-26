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

// SecurityRuleDirection defines whether a rule applies to ingress or egress traffic
// +kubebuilder:validation:Enum=INGRESS;EGRESS
type SecurityRuleDirection string

const (
	SecurityRuleDirectionIngress SecurityRuleDirection = "INGRESS"
	SecurityRuleDirectionEgress  SecurityRuleDirection = "EGRESS"
)

// SecurityRule defines a single ingress or egress rule
type SecurityRule struct {
	// Description is a human-readable description of the rule.
	// +optional
	Description string `json:"description,omitempty"`

	// Direction specifies whether this is an INGRESS or EGRESS rule.
	// +kubebuilder:validation:Required
	Direction SecurityRuleDirection `json:"direction"`

	// Protocol is the IP protocol (e.g. "6" for TCP, "17" for UDP, "all" for all protocols).
	// +kubebuilder:validation:Required
	Protocol string `json:"protocol"`

	// Source is the source CIDR for ingress rules (e.g. "0.0.0.0/0").
	// +optional
	Source string `json:"source,omitempty"`

	// Destination is the destination CIDR for egress rules (e.g. "0.0.0.0/0").
	// +optional
	Destination string `json:"destination,omitempty"`

	// MinPort is the minimum port number for the rule.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	MinPort *int32 `json:"minPort,omitempty"`

	// MaxPort is the maximum port number for the rule.
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +optional
	MaxPort *int32 `json:"maxPort,omitempty"`
}

// OCISecurityPolicySpec defines the desired state of OCISecurityPolicy
type OCISecurityPolicySpec struct {
	// CompartmentID is the OCID of the compartment containing the security list.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	CompartmentID string `json:"compartmentId"`

	// VcnID is the OCID of the VCN that owns the security list.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	VcnID string `json:"vcnId"`

	// DisplayName is the human-readable name for the security list.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=255
	DisplayName string `json:"displayName"`

	// Rules is the list of security rules to apply.
	// +kubebuilder:validation:MinItems=1
	Rules []SecurityRule `json:"rules"`

	// FreeformTags are key-value pairs you can attach to the security list.
	// +optional
	FreeformTags map[string]string `json:"freeformTags,omitempty"`
}

// SecurityPolicyPhase represents the lifecycle phase of the security policy
// +kubebuilder:validation:Enum=Pending;Creating;Active;Updating;Deleting;Deleted;Failed
type SecurityPolicyPhase string

const (
	SecurityPolicyPhasePending  SecurityPolicyPhase = "Pending"
	SecurityPolicyPhaseCreating SecurityPolicyPhase = "Creating"
	SecurityPolicyPhaseActive   SecurityPolicyPhase = "Active"
	SecurityPolicyPhaseUpdating SecurityPolicyPhase = "Updating"
	SecurityPolicyPhaseDeleting SecurityPolicyPhase = "Deleting"
	SecurityPolicyPhaseDeleted  SecurityPolicyPhase = "Deleted"
	SecurityPolicyPhaseFailed   SecurityPolicyPhase = "Failed"
)

// OCISecurityPolicyStatus defines the observed state of OCISecurityPolicy
type OCISecurityPolicyStatus struct {
	// SecurityListID is the OCID of the provisioned OCI security list.
	// +optional
	SecurityListID string `json:"securityListId,omitempty"`

	// Phase represents the current lifecycle phase of the security policy.
	// +optional
	Phase SecurityPolicyPhase `json:"phase,omitempty"`

	// FailureReason contains the error message if the policy failed.
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// ObservedGeneration is the last generation reconciled by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the current state of the OCISecurityPolicy resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="SecurityListID",type="string",JSONPath=".status.securityListId"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// OCISecurityPolicy is the Schema for the ocisecuritypolicies API
type OCISecurityPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OCISecurityPolicySpec   `json:"spec,omitempty"`
	Status OCISecurityPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OCISecurityPolicyList contains a list of OCISecurityPolicy
type OCISecurityPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OCISecurityPolicy `json:"items"`
}

// SetFailedStatus sets the phase to Failed with the given reason.
func (o *OCISecurityPolicy) SetFailedStatus(reason string) {
	o.Status.Phase = SecurityPolicyPhaseFailed
	o.Status.FailureReason = reason
}

// SetObservedGeneration sets the observed generation on the status.
func (o *OCISecurityPolicy) SetObservedGeneration(generation int64) {
	o.Status.ObservedGeneration = generation
}

func init() {
	SchemeBuilder.Register(&OCISecurityPolicy{}, &OCISecurityPolicyList{})
}
