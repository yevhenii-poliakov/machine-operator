package provider

import (
	"context"
	"errors"
	"testing"

	infrastructurev1alpha1 "github.com/yevhenii-poliakov/machine-operator/api/v1alpha1"
)

func TestMemoryProviderCreateAndGetMachine(t *testing.T) {
	ctx := context.Background()
	machineProvider := NewMemoryProvider()

	spec := infrastructurev1alpha1.MachineSpec{
		Image:  "ubuntu-24.04",
		Flavor: "medium",
		Region: "vienna",
	}

	providerID, err := machineProvider.CreateMachine(ctx, spec)
	if err != nil {
		t.Fatalf("CreateMachine() returned an error: %v", err)
	}

	if providerID != "machine-00001" {
		t.Fatalf(
			"CreateMachine() provider ID = %q, want %q",
			providerID,
			"machine-00001",
		)
	}

	state, err := machineProvider.GetMachine(ctx, providerID)
	if err != nil {
		t.Fatalf("GetMachine() returned an error: %v", err)
	}

	if state.ProviderID != providerID {
		t.Errorf(
			"GetMachine() provider ID = %q, want %q",
			state.ProviderID,
			providerID,
		)
	}

	if state.State != "Running" {
		t.Errorf(
			"GetMachine() state = %q, want %q",
			state.State,
			"Running",
		)
	}
}

func TestMemoryProviderGetMissingMachine(t *testing.T) {
	machineProvider := NewMemoryProvider()

	_, err := machineProvider.GetMachine(
		context.Background(),
		"machine-does-not-exist",
	)

	if !errors.Is(err, ErrMachineNotFound) {
		t.Fatalf(
			"GetMachine() error = %v, want ErrMachineNotFound",
			err,
		)
	}
}

func TestMemoryProviderDeleteMachine(t *testing.T) {
	ctx := context.Background()
	machineProvider := NewMemoryProvider()

	providerID, err := machineProvider.CreateMachine(
		ctx,
		infrastructurev1alpha1.MachineSpec{
			Image:  "ubuntu-24.04",
			Flavor: "medium",
			Region: "vienna",
		},
	)
	if err != nil {
		t.Fatalf("CreateMachine() returned an error: %v", err)
	}

	if err := machineProvider.DeleteMachine(ctx, providerID); err != nil {
		t.Fatalf("DeleteMachine() returned an error: %v", err)
	}

	// Deleting an already absent machine should remain successful.
	if err := machineProvider.DeleteMachine(ctx, providerID); err != nil {
		t.Fatalf("second DeleteMachine() returned an error: %v", err)
	}

	_, err = machineProvider.GetMachine(ctx, providerID)
	if !errors.Is(err, ErrMachineNotFound) {
		t.Fatalf(
			"GetMachine() after deletion error = %v, want ErrMachineNotFound",
			err,
		)
	}
}
