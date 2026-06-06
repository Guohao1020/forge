package nacos

import "context"

// NacosQuerier is the seam the resolver + handler depend on. The real adapter
// (client.go) implements it over the Nacos AI Registry REST API; tests and the
// cache wrapper depend only on this interface, so the deterministic core needs
// no live Nacos.
type NacosQuerier interface {
	ListMCPServers(ctx context.Context, namespace string) ([]MCPServerShape, error)
	GetMCPServer(ctx context.Context, namespace, name, ref string) (MCPServerShape, error)
	RegisterMCPServer(ctx context.Context, namespace string, s MCPServerShape) error
	SetMCPLifecycle(ctx context.Context, namespace, name, version, lifecycle string) error
}
