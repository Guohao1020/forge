package nacos

import "context"

// ProviderQuerier is the seam the providerresolve package + handler depend on.
// The real adapter (provider_client.go) implements it over the Nacos config
// center; tests use a fake.
type ProviderQuerier interface {
	ListProviders(ctx context.Context, namespace string) ([]ProviderShape, error)
	GetProvider(ctx context.Context, namespace, name, ref string) (ProviderShape, error)
	RegisterProvider(ctx context.Context, namespace string, p ProviderShape) error
	SetProviderLifecycle(ctx context.Context, namespace, name, version, lifecycle string) error
}
