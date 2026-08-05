package provider

import (
	"context"

	infrastructurev1alpha1 "github.com/yevhenii-poliakov/machine-operator/api/v1alpha1"
)

// MachineState represents the state returned by an infrastructure provider.
type MachineState struct {
	ProviderID string
	State      string
}

// MachineProvider defines the operations required by the Machine controller.
type MachineProvider interface {
	CreateMachine(
		ctx context.Context,
		spec infrastructurev1alpha1.MachineSpec,
	) (string, error)

	GetMachine(
		ctx context.Context,
		providerID string,
	) (*MachineState, error)

	DeleteMachine(
		ctx context.Context,
		providerID string,
	) error
}
