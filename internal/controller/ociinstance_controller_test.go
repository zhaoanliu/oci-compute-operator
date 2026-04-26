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

// newTestOCIInstance is a helper that returns a valid OCIInstance for testing
func newTestOCIInstance(name, namespace string) *computev1alpha1.OCIInstance {
	return &computev1alpha1.OCIInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: computev1alpha1.OCIInstanceSpec{
			CompartmentID:      "ocid1.compartment.oc1..test",
			DisplayName:        "test-instance",
			Shape:              "VM.Standard.E4.Flex",
			ImageID:            "ocid1.image.oc1..test",
			AvailabilityDomain: "AD-1",
			SubnetID:           "ocid1.subnet.oc1..test",
		},
	}
}

var _ = Describe("OCIInstance Controller", func() {
	const (
		resourceName      = "test-resource"
		resourceNamespace = "default"
	)

	ctx := context.Background()

	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: resourceNamespace,
	}

	// helper to build the reconciler
	newReconciler := func() *OCIInstanceReconciler {
		return &OCIInstanceReconciler{
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
	fetchInstance := func() *computev1alpha1.OCIInstance {
		instance := &computev1alpha1.OCIInstance{}
		Expect(k8sClient.Get(ctx, typeNamespacedName, instance)).To(Succeed())
		return instance
	}

	AfterEach(func() {
		// Clean up after each test
		instance := &computev1alpha1.OCIInstance{}
		err := k8sClient.Get(ctx, typeNamespacedName, instance)
		if err == nil {
			// Remove finalizer so deletion can complete in test env
			controllerutil.RemoveFinalizer(instance, ociInstanceFinalizer)
			Expect(k8sClient.Update(ctx, instance)).To(Succeed())
			Expect(k8sClient.Delete(ctx, instance)).To(Succeed())
		}
	})

	Context("When creating a new OCIInstance", func() {
		BeforeEach(func() {
			By("creating the OCIInstance resource")
			err := k8sClient.Get(ctx, typeNamespacedName, &computev1alpha1.OCIInstance{})
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, newTestOCIInstance(resourceName, resourceNamespace))).To(Succeed())
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
			instance := fetchInstance()
			Expect(controllerutil.ContainsFinalizer(instance, ociInstanceFinalizer)).To(BeTrue())
		})
	})

	Context("When validating OCIInstance spec", func() {
		It("should reject a resource with empty DisplayName", func() {
			By("creating an OCIInstance with empty DisplayName")
			invalid := newTestOCIInstance("invalid-display-name", resourceNamespace)
			invalid.Spec.DisplayName = ""
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("displayName"))
		})

		It("should reject a resource with DisplayName longer than 255 chars", func() {
			By("creating an OCIInstance with DisplayName > 255 chars")
			invalid := newTestOCIInstance("invalid-display-name-long", resourceNamespace)
			invalid.Spec.DisplayName = string(make([]byte, 256))
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("displayName"))
		})

		It("should reject a resource with empty CompartmentID", func() {
			By("creating an OCIInstance with empty CompartmentID")
			invalid := newTestOCIInstance("invalid-compartment-id", resourceNamespace)
			invalid.Spec.CompartmentID = ""
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("compartmentId"))
		})

		It("should reject a resource with CompartmentID longer than 255 chars", func() {
			By("creating an OCIInstance with CompartmentID > 255 chars")
			invalid := newTestOCIInstance("invalid-compartment-id-long", resourceNamespace)
			invalid.Spec.CompartmentID = string(make([]byte, 256))
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("compartmentId"))
		})

		It("should reject a resource with empty ImageID", func() {
			By("creating an OCIInstance with empty ImageID")
			invalid := newTestOCIInstance("invalid-image-id", resourceNamespace)
			invalid.Spec.ImageID = ""
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("imageId"))
		})

		It("should reject a resource with ImageID longer than 255 chars", func() {
			By("creating an OCIInstance with ImageID > 255 chars")
			invalid := newTestOCIInstance("invalid-image-id-long", resourceNamespace)
			invalid.Spec.ImageID = string(make([]byte, 256))
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("imageId"))
		})

		It("should reject a resource with empty SubnetID", func() {
			By("creating an OCIInstance with empty SubnetID")
			invalid := newTestOCIInstance("invalid-subnet-id", resourceNamespace)
			invalid.Spec.SubnetID = ""
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("subnetId"))
		})

		It("should reject a resource with SubnetID longer than 255 chars", func() {
			By("creating an OCIInstance with SubnetID > 255 chars")
			invalid := newTestOCIInstance("invalid-subnet-id-long", resourceNamespace)
			invalid.Spec.SubnetID = string(make([]byte, 256))
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("subnetId"))
		})

		It("should reject a resource with empty AvailabilityDomain", func() {
			By("creating an OCIInstance with empty AvailabilityDomain")
			invalid := newTestOCIInstance("invalid-ad", resourceNamespace)
			invalid.Spec.AvailabilityDomain = ""
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("availabilityDomain"))
		})

		It("should reject a resource with AvailabilityDomain longer than 255 chars", func() {
			By("creating an OCIInstance with AvailabilityDomain > 255 chars")
			invalid := newTestOCIInstance("invalid-ad-long", resourceNamespace)
			invalid.Spec.AvailabilityDomain = string(make([]byte, 256))
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("availabilityDomain"))
		})

		It("should reject a resource with empty Shape", func() {
			By("creating an OCIInstance with empty Shape")
			invalid := newTestOCIInstance("invalid-shape", resourceNamespace)
			invalid.Spec.Shape = ""
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("shape"))
		})

		It("should reject a resource with Shape longer than 255 chars", func() {
			By("creating an OCIInstance with Shape > 255 chars")
			invalid := newTestOCIInstance("invalid-shape-long", resourceNamespace)
			invalid.Spec.Shape = string(make([]byte, 256))
			err := k8sClient.Create(ctx, invalid)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("shape"))
		})

		It("should accept a resource with all required fields", func() {
			By("creating a valid OCIInstance")
			valid := newTestOCIInstance("valid-resource", resourceNamespace)
			Expect(k8sClient.Create(ctx, valid)).To(Succeed())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, valid)).To(Succeed())
		})

		It("should accept optional freeform tags", func() {
			By("creating an OCIInstance with freeform tags")
			tagged := newTestOCIInstance("tagged-resource", resourceNamespace)
			tagged.Spec.FreeformTags = map[string]string{
				"env":  "test",
				"team": "nvcne",
			}
			Expect(k8sClient.Create(ctx, tagged)).To(Succeed())

			By("cleaning up")
			Expect(k8sClient.Delete(ctx, tagged)).To(Succeed())
		})
	})

	Context("When an OCIInstance already has a finalizer", func() {
		BeforeEach(func() {
			By("creating the OCIInstance resource")
			err := k8sClient.Get(ctx, typeNamespacedName, &computev1alpha1.OCIInstance{})
			if err != nil && errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, newTestOCIInstance(resourceName, resourceNamespace))).To(Succeed())
			}

			By("running first reconcile to add finalizer")
			Expect(reconcileOnce()).To(Succeed())
		})

		It("should not add duplicate finalizers on subsequent reconciles", func() {
			By("running reconcile a second time")
			Expect(reconcileOnce()).To(Succeed())

			By("checking finalizer appears exactly once")
			instance := fetchInstance()
			count := 0
			for _, f := range instance.Finalizers {
				if f == ociInstanceFinalizer {
					count++
				}
			}
			Expect(count).To(Equal(1))
		})
	})
})
