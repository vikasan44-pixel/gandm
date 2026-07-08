// Package payment abstracts the payment provider so the real integration
// can replace the sandbox later without touching business logic: implement
// Provider, register it in NewProvider, set PAYMENT_PROVIDER.
package payment

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

type Provider interface {
	// Name identifies the provider in stored payment rows.
	Name() string
	// Charge performs a one-time charge and returns the provider's
	// reference for the transaction.
	Charge(ctx context.Context, clientID uuid.UUID, purpose string) (ref string, err error)
}

// SandboxProvider "succeeds" every charge without moving money — the MVP
// stand-in until a real provider exists.
type SandboxProvider struct{}

func (SandboxProvider) Name() string { return "sandbox" }

func (SandboxProvider) Charge(_ context.Context, _ uuid.UUID, _ string) (string, error) {
	return "sandbox-" + uuid.NewString(), nil
}

func NewProvider(name string) (Provider, error) {
	switch name {
	case "sandbox":
		return SandboxProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown payment provider %q (supported: sandbox)", name)
	}
}
