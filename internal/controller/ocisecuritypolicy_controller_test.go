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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	computev1alpha1 "github.com/zhaoanliu/oci-compute-operator/api/v1alpha1"
)

// int32Ptr is a helper to get a pointer to an int32 value
//
//nolint:gocritic
//go:fix inline
func int32Ptr(i int32) *int32 {
	return new(i)
}

// newTestOCISecurityPolicy is a helper that returns a valid OCISecurityPolicy for testing
func newTestOCISecurityPolicy(name, namespace string) *computev1alpha1.OCISecurityPolicy {
	return &computev1alpha1.OCISecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: computev1alpha1.OCISecurityPolicySpec{
			CompartmentID: "ocid1.compartment.oc1..test",
			VcnID:         "ocid1.vcn.oc1..test",
			DisplayName:   "test-security-policy",
			Rules: []computev1alpha1.SecurityRule{
				{
					Direction:   computev1alpha1.SecurityRuleDirectionIngress,
					Protocol:    "6",
					Source:      "0.0.0.0/0",
					MinPort:     int32Ptr(443),
					MaxPort:     int32Ptr(443),
					Description: "Allow HTTPS inbound",
				},
			},
		},
	}
}

var _ = Describe("OCISecurityPolicy Controller", func() {
	const (
		resourceName = "test-security-resource"
	)

	ctx := context.Background()

	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: testNamespaceDefault,
	}

	// helper to build the reconciler
	newReconciler := func() *OCISecurityPolicyReconciler {
		return &OCISecurityPolicyReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}
	}

	// helper to run one reconcile pass
	reconcileOnce := func() error {
		_, err := newReconciler().Reconcile(ctx, reconcile.Request{
			NamespacedName: typeNamespacedName,
		})
		return err
	}

	// helper to fetch the current state of the resource
	fetchPolicy := func() *computev1alpha1.OCISecurityPolicy {
		policy := &computev1alpha1.OCISecurityPolicy{}
		Expect(k8sClient.Get(ctx, typeNamespacedName, policy)).To(Succeed())
		return policy
	}

	AfterEach(func() {
		// Clean up after each test
		policy := &computev1alpha1.OCISecurityPolicy{}
		err := k8sClient.Get(ctx, typeNamespacedName, policy)
		if err == nil {
			// Remove finalizer so deletion can complete in test env
			controllerutil.RemoveFinalizer(policy, ociSecurityPolicyFinalizer)
			Expect(k8sClient.Update(ctx, policy)).To(Succeed())
			Expect(k8sClient.Delete(ctx, policy)).To(Succeed())
		}
	})

	Context("When creating a new OCISecurityPolicy", func() {
		BeforeEach(func() {
			By("creating the OCISecurityPolicy resource")
			err := k8sClient.Get(ctx, typeNamespacedName, &computev1alpha1.OCISecurityPolicy{})
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, newTestOCISecurityPolicy(resourceName, testNamespaceDefault))).To(Succeed())
			}
		})

		It("should not return an error on first reconcile", func() {
			By("running the first reconcile")
			Expect(reconcileOnce()).To(Succeed())
		})

		It("should add a finalizer on first reconcile", func() {
			By("running the first reconcile")
			Expect(reconcileOnce()).To(Succeed())

			By("checking the finalizer was added")
			policy := fetchPolicy()
			Expect(controllerutil.ContainsFinalizer(policy, ociSecurityPolicyFinalizer)).To(BeTrue())
		})
	})

	Context("When validating OCISecurityPolicy spec", func() {
		It("should reject a resource with empty DisplayName", func() {
			By("creating an OCISecurityPolicy with empty DisplayName")
			invalid := newTestOCISecurityPolicy("invalid-display-name", testNamespaceValidation)
			invalid.Spec.DisplayName = ""
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("displayName"))
		})

		It("should reject a resource with DisplayName longer than 255 chars", func() {
			By("creating an OCISecurityPolicy with DisplayName > 255 chars")
			invalid := newTestOCISecurityPolicy("invalid-display-name-long", testNamespaceValidation)
			invalid.Spec.DisplayName = string(make([]byte, 256))
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("displayName"))
		})

		It("should reject a resource with empty CompartmentID", func() {
			By("creating an OCISecurityPolicy with empty CompartmentID")
			invalid := newTestOCISecurityPolicy("invalid-compartment-id", testNamespaceValidation)
			invalid.Spec.CompartmentID = ""
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("compartmentId"))
		})

		It("should reject a resource with CompartmentID longer than 255 chars", func() {
			By("creating an OCISecurityPolicy with CompartmentID > 255 chars")
			invalid := newTestOCISecurityPolicy("invalid-compartment-id-long", testNamespaceValidation)
			invalid.Spec.CompartmentID = string(make([]byte, 256))
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("compartmentId"))
		})

		It("should reject a resource with empty VcnID", func() {
			By("creating an OCISecurityPolicy with empty VcnID")
			invalid := newTestOCISecurityPolicy("invalid-vcn-id", testNamespaceValidation)
			invalid.Spec.VcnID = ""
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("vcnId"))
		})

		It("should reject a resource with VcnID longer than 255 chars", func() {
			By("creating an OCISecurityPolicy with VcnID > 255 chars")
			invalid := newTestOCISecurityPolicy("invalid-vcn-id-long", testNamespaceValidation)
			invalid.Spec.VcnID = string(make([]byte, 256))
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("vcnId"))
		})

		It("should reject a resource with no rules", func() {
			By("creating an OCISecurityPolicy with empty rules list")
			invalid := newTestOCISecurityPolicy("invalid-no-rules", testNamespaceValidation)
			invalid.Spec.Rules = []computev1alpha1.SecurityRule{}
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("rules"))
		})

		It("should reject a rule with invalid direction", func() {
			By("creating an OCISecurityPolicy with invalid rule direction")
			invalid := newTestOCISecurityPolicy("invalid-direction", testNamespaceValidation)
			invalid.Spec.Rules = []computev1alpha1.SecurityRule{
				{
					Direction: "INVALID",
					Protocol:  "6",
					Source:    "0.0.0.0/0",
				},
			}
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("direction"))
		})

		It("should reject a rule with port below minimum", func() {
			By("creating an OCISecurityPolicy with port 0")
			invalid := newTestOCISecurityPolicy("invalid-port-min", testNamespaceValidation)
			invalid.Spec.Rules = []computev1alpha1.SecurityRule{
				{
					Direction: computev1alpha1.SecurityRuleDirectionIngress,
					Protocol:  "6",
					Source:    "0.0.0.0/0",
					MinPort:   int32Ptr(0),
					MaxPort:   int32Ptr(443),
				},
			}
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("minPort"))
		})

		It("should reject a rule with port above maximum", func() {
			By("creating an OCISecurityPolicy with port 65536")
			invalid := newTestOCISecurityPolicy("invalid-port-max", testNamespaceValidation)
			invalid.Spec.Rules = []computev1alpha1.SecurityRule{
				{
					Direction: computev1alpha1.SecurityRuleDirectionIngress,
					Protocol:  "6",
					Source:    "0.0.0.0/0",
					MinPort:   int32Ptr(443),
					MaxPort:   int32Ptr(65536),
				},
			}
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("maxPort"))
		})

		It("should accept a resource with all required fields", func() {
			By("creating a valid OCISecurityPolicy")
			valid := newTestOCISecurityPolicy("valid-security-policy", testNamespaceValidation)
			Expect(k8sClient.Create(ctx, valid)).To(Succeed())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, valid)).To(Succeed())
		})

		It("should accept optional freeform tags", func() {
			By("creating an OCISecurityPolicy with freeform tags")
			tagged := newTestOCISecurityPolicy("tagged-security-policy", testNamespaceTags)
			tagged.Spec.FreeformTags = map[string]string{
				"env":  "test",
				"team": "nvcne",
			}
			Expect(k8sClient.Create(ctx, tagged)).To(Succeed())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, tagged)).To(Succeed())
		})

		It("should accept both ingress and egress rules", func() {
			By("creating an OCISecurityPolicy with both rule directions")
			mixed := newTestOCISecurityPolicy("mixed-rules-policy", testNamespaceValidation)
			mixed.Spec.Rules = []computev1alpha1.SecurityRule{
				{
					Direction: computev1alpha1.SecurityRuleDirectionIngress,
					Protocol:  "6",
					Source:    "0.0.0.0/0",
					MinPort:   int32Ptr(443),
					MaxPort:   int32Ptr(443),
				},
				{
					Direction:   computev1alpha1.SecurityRuleDirectionEgress,
					Protocol:    "all",
					Destination: "0.0.0.0/0",
				},
			}
			Expect(k8sClient.Create(ctx, mixed)).To(Succeed())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, mixed)).To(Succeed())
		})
	})

	Context("When an OCISecurityPolicy already has a finalizer", func() {
		BeforeEach(func() {
			By("creating the OCISecurityPolicy resource")
			err := k8sClient.Get(ctx, typeNamespacedName, &computev1alpha1.OCISecurityPolicy{})
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, newTestOCISecurityPolicy(resourceName, testNamespaceDefault))).To(Succeed())
			}

			By("running first reconcile to add finalizer")
			Expect(reconcileOnce()).To(Succeed())
		})

		It("should not add duplicate finalizers on subsequent reconciles", func() {
			By("running reconcile a second time")
			Expect(reconcileOnce()).To(Succeed())

			By("checking finalizer appears exactly once")
			policy := fetchPolicy()
			count := 0
			for _, f := range policy.Finalizers {
				if f == ociSecurityPolicyFinalizer {
					count++
				}
			}
			Expect(count).To(Equal(1))
		})
	})
})
