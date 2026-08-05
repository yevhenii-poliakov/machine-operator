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
	stderrors "errors"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	machineprovider "github.com/yevhenii-poliakov/machine-operator/internal/provider"

	infrastructurev1alpha1 "github.com/yevhenii-poliakov/machine-operator/api/v1alpha1"
)

var _ = Describe("Machine Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		machine := &infrastructurev1alpha1.Machine{}

		var (
			memoryProvider       *machineprovider.MemoryProvider
			controllerReconciler *MachineReconciler
		)

		BeforeEach(func() {
			By("creating the custom resource for the Kind Machine")
			err := k8sClient.Get(ctx, typeNamespacedName, machine)
			if err != nil && apierrors.IsNotFound(err) {
				resource := &infrastructurev1alpha1.Machine{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: resourceNamespace,
					},
					Spec: infrastructurev1alpha1.MachineSpec{
						Image:  "ubuntu-24.04",
						Flavor: "medium",
						Region: "vienna",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}

			memoryProvider = machineprovider.NewMemoryProvider()
			controllerReconciler = &MachineReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Provider: memoryProvider,
			}

			By("Adding the finalizer")

			_, err = controllerReconciler.Reconcile(
				ctx,
				reconcile.Request{
					NamespacedName: typeNamespacedName,
				},
			)
			Expect(err).NotTo(HaveOccurred())

			machineWithFinalizer := &infrastructurev1alpha1.Machine{}

			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					machineWithFinalizer,
				),
			).To(Succeed())

			Expect(
				controllerutil.ContainsFinalizer(
					machineWithFinalizer,
					machineFinalizer,
				),
			).To(BeTrue())
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &infrastructurev1alpha1.Machine{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if apierrors.IsNotFound(err) {
				return
			}

			Expect(err).NotTo(HaveOccurred())

			if controllerutil.ContainsFinalizer(resource, machineFinalizer) {
				controllerutil.RemoveFinalizer(resource, machineFinalizer)
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())
			}

			err = k8sClient.Delete(ctx, resource)
			if err != nil {
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")

			result, err := controllerReconciler.Reconcile(
				ctx,
				reconcile.Request{
					NamespacedName: typeNamespacedName,
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(BeNumerically(">", 0), "Expected RequeueAfter to be greater than 0")
			reconciledMachine := &infrastructurev1alpha1.Machine{}

			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					reconciledMachine,
				),
			).To(Succeed())

			Expect(reconciledMachine.Status.ProviderID).To(
				Equal("machine-00001"),
			)
			Expect(reconciledMachine.Status.State).To(
				Equal("Running"),
			)

			firstProviderID := reconciledMachine.Status.ProviderID

			By("Reconciling the same Machine again")

			_, err = controllerReconciler.Reconcile(
				ctx,
				reconcile.Request{
					NamespacedName: typeNamespacedName,
				},
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(
				k8sClient.Get(
					ctx,
					typeNamespacedName,
					reconciledMachine,
				),
			).To(Succeed())

			Expect(reconciledMachine.Status.ProviderID).To(
				Equal(firstProviderID),
			)

			By("Deleting the Machine resource")

			Expect(
				k8sClient.Delete(ctx, reconciledMachine),
			).To(Succeed())

			By("Reconciling the deleting Machine")

			_, err = controllerReconciler.Reconcile(
				ctx,
				reconcile.Request{
					NamespacedName: typeNamespacedName,
				},
			)
			Expect(err).NotTo(HaveOccurred())

			_, err = memoryProvider.GetMachine(ctx, firstProviderID)
			Expect(
				stderrors.Is(err, machineprovider.ErrMachineNotFound),
			).To(BeTrue())

			Eventually(func() bool {
				deletedMachine := &infrastructurev1alpha1.Machine{}

				err := k8sClient.Get(
					ctx,
					typeNamespacedName,
					deletedMachine,
				)

				return apierrors.IsNotFound(err)
			}, 5*time.Second, 100*time.Millisecond).Should(BeTrue())
		})
	})
})
