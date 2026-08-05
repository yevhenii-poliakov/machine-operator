package provider

import (
	"context"
	"errors"
	"fmt"
	"sync"

	infrastructurev1alpha1 "github.com/yevhenii-poliakov/machine-operator/api/v1alpha1"
)

// ErrMachineNotFound is returned when the provider does not know a machine ID.
var ErrMachineNotFound = errors.New("machine not found")

// storedMachine represents the data kept internally by MemoryProvider.
type storedMachine struct {
	Spec  infrastructurev1alpha1.MachineSpec
	State MachineState
}

// MemoryProvider stores machine information in process memory.
type MemoryProvider struct {
	mu       sync.RWMutex
	machines map[string]storedMachine
	nextID   uint64
}

// Compile-time check that *MemoryProvider implements MachineProvider.
var _ MachineProvider = (*MemoryProvider)(nil)

// NewMemoryProvider creates an initialized MemoryProvider.
func NewMemoryProvider() *MemoryProvider {
	return &MemoryProvider{
		machines: make(map[string]storedMachine),
	}
}

// CreateMachine creates an in-memory machine and returns its provider ID.
func (p *MemoryProvider) CreateMachine(
	ctx context.Context,
	spec infrastructurev1alpha1.MachineSpec,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.nextID++
	providerID := fmt.Sprintf("machine-%05d", p.nextID)

	p.machines[providerID] = storedMachine{
		Spec: spec,
		State: MachineState{
			ProviderID: providerID,
			State:      "Running",
		},
	}

	return providerID, nil
}

// GetMachine returns the current state of an in-memory machine.
func (p *MemoryProvider) GetMachine(
	ctx context.Context,
	providerID string,
) (*MachineState, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	machine, exists := p.machines[providerID]
	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrMachineNotFound, providerID)
	}

	state := machine.State
	return &state, nil
}

// DeleteMachine removes an in-memory machine.
func (p *MemoryProvider) DeleteMachine(
	ctx context.Context,
	providerID string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.machines, providerID)

	return nil
}
