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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	computev1alpha1 "github.com/zhaoanliu/oci-compute-operator/api/v1alpha1"
)

var _ = Describe("OCISecurityPolicy Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		ocisecuritypolicy := &computev1alpha1.OCISecurityPolicy{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind OCISecurityPolicy")
			err := k8sClient.Get(ctx, typeNamespacedName, ocisecuritypolicy)
			if err != nil && errors.IsNotFound(err) {
				resource := &computev1alpha1.OCISecurityPolicy{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
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
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &computev1alpha1.OCISecurityPolicy{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())
			By("Cleanup the specific resource instance OCISecurityPolicy")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &OCISecurityPolicyReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

// int32Ptr is a helper to get a pointer to an int32 value
func int32Ptr(i int32) *int32 {
	return &i
}
