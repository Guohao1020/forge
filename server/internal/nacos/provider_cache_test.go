package nacos

import (
	"context"
	"errors"
	"testing"
)

type fakePQ struct {
	shape ProviderShape
	err   error
}

func (f *fakePQ) ListProviders(context.Context, string) ([]ProviderShape, error) { return nil, f.err }
func (f *fakePQ) GetProvider(context.Context, string, string, string) (ProviderShape, error) {
	return f.shape, f.err
}
func (f *fakePQ) RegisterProvider(context.Context, string, ProviderShape) error               { return f.err }
func (f *fakePQ) SetProviderLifecycle(context.Context, string, string, string, string) error { return f.err }

func TestCachedProviderQuerier_FallsBackOnError(t *testing.T) {
	f := &fakePQ{shape: ProviderShape{Name: "router", Version: "1.0.0", Protocol: "anthropic"}}
	c := NewCachedProviderQuerier(f)
	if _, err := c.GetProvider(context.Background(), "ws1", "router", "stable"); err != nil {
		t.Fatalf("warm: %v", err)
	}
	f.err = errors.New("nacos down")
	got, err := c.GetProvider(context.Background(), "ws1", "router", "stable")
	if err != nil || got.Name != "router" {
		t.Fatalf("expected cached fallback, got %+v err=%v", got, err)
	}
}
