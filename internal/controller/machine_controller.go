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
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"time"

	infrastructurev1alpha1 "github.com/yevhenii-poliakov/machine-operator/api/v1alpha1"
	machineprovider "github.com/yevhenii-poliakov/machine-operator/internal/provider"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	machineFinalizer     = "infrastructure.example.com/machine-finalizer"
	providerPollInterval = 30 * time.Second
)

// MachineReconciler reconciles a Machine object
type MachineReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Provider machineprovider.MachineProvider
}

// +kubebuilder:rbac:groups=infrastructure.example.com,resources=machines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.example.com,resources=machines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.example.com,resources=machines/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Machine object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.24.1/pkg/reconcile
func (r *MachineReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx).WithValues(
		"machine",
		req.NamespacedName,
	)

	var machine infrastructurev1alpha1.Machine

	if err := r.Get(ctx, req.NamespacedName, &machine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf(
			"get Machine %s/%s: %w",
			req.Namespace,
			req.Name,
			err,
		)
	}

	if !machine.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(&machine, machineFinalizer) {
			return ctrl.Result{}, nil
		}

		if machine.Status.ProviderID != "" {
			if r.Provider == nil {
				return ctrl.Result{}, errors.New(
					"machine provider is not configured",
				)
			}

			if err := r.Provider.DeleteMachine(
				ctx,
				machine.Status.ProviderID,
			); err != nil {
				return ctrl.Result{}, fmt.Errorf(
					"delete provider machine %q: %w",
					machine.Status.ProviderID,
					err,
				)
			}

			logger.Info(
				"Deleted machine from infrastructure provider",
				"providerID",
				machine.Status.ProviderID,
			)
		}

		controllerutil.RemoveFinalizer(&machine, machineFinalizer)

		if err := r.Update(ctx, &machine); err != nil {
			return ctrl.Result{}, fmt.Errorf(
				"remove finalizer from Machine %s/%s: %w",
				req.Namespace,
				req.Name,
				err,
			)
		}

		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&machine, machineFinalizer) {
		controllerutil.AddFinalizer(&machine, machineFinalizer)

		if err := r.Update(ctx, &machine); err != nil {
			return ctrl.Result{}, fmt.Errorf(
				"add finalizer to Machine %s/%s: %w",
				req.Namespace,
				req.Name,
				err,
			)
		}

		logger.Info("Added finalizer to Machine")

		return ctrl.Result{}, nil
	}

	if r.Provider == nil {
		return ctrl.Result{}, errors.New(
			"machine provider is not configured",
		)
	}

	providerID := machine.Status.ProviderID

	if providerID == "" {
		createdProviderID, err := r.Provider.CreateMachine(
			ctx,
			machine.Spec,
		)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf(
				"create provider machine for %s/%s: %w",
				req.Namespace,
				req.Name,
				err,
			)
		}

		if createdProviderID == "" {
			return ctrl.Result{}, errors.New(
				"provider returned an empty machine ID",
			)
		}

		providerID = createdProviderID

		logger.Info(
			"Created machine in infrastructure provider",
			"providerID",
			providerID,
		)
	}

	providerState, err := r.Provider.GetMachine(ctx, providerID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf(
			"get provider machine %q: %w",
			providerID,
			err,
		)
	}

	if providerState == nil {
		return ctrl.Result{}, fmt.Errorf(
			"provider returned nil state for machine %q",
			providerID,
		)
	}

	if providerState.ProviderID != providerID {
		return ctrl.Result{}, fmt.Errorf(
			"provider returned machine ID %q for requested ID %q",
			providerState.ProviderID,
			providerID,
		)
	}

	statusUnchanged :=
		machine.Status.ProviderID == providerState.ProviderID &&
			machine.Status.State == providerState.State

	if statusUnchanged {
		return ctrl.Result{
			RequeueAfter: providerPollInterval,
		}, nil
	}

	machine.Status.ProviderID = providerState.ProviderID
	machine.Status.State = providerState.State

	if err := r.Status().Update(ctx, &machine); err != nil {
		return ctrl.Result{}, fmt.Errorf(
			"update status of Machine %s/%s: %w",
			req.Namespace,
			req.Name,
			err,
		)
	}

	logger.Info(
		"Updated Machine status",
		"providerID",
		machine.Status.ProviderID,
		"state",
		machine.Status.State,
	)

	return ctrl.Result{
		RequeueAfter: providerPollInterval,
	}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *MachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha1.Machine{}).
		Named("machine").
		Complete(r)
}
