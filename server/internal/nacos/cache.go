package nacos

import (
	"context"
	"sync"
)

// CachedQuerier wraps a NacosQuerier with a last-known-good cache on
// GetMCPServer so a Nacos outage degrades gracefully (return cached shape)
// instead of failing dispatch. Writes and list pass through unchanged.
type CachedQuerier struct {
	inner NacosQuerier
	mu    sync.RWMutex
	cache map[string]MCPServerShape
}

func NewCachedQuerier(inner NacosQuerier) *CachedQuerier {
	return &CachedQuerier{inner: inner, cache: map[string]MCPServerShape{}}
}

func cacheKey(ns, name, ref string) string { return ns + "|" + name + "|" + ref }

func (c *CachedQuerier) GetMCPServer(ctx context.Context, ns, name, ref string) (MCPServerShape, error) {
	s, err := c.inner.GetMCPServer(ctx, ns, name, ref)
	if err == nil {
		c.mu.Lock()
		c.cache[cacheKey(ns, name, ref)] = s
		c.mu.Unlock()
		return s, nil
	}
	c.mu.RLock()
	cached, ok := c.cache[cacheKey(ns, name, ref)]
	c.mu.RUnlock()
	if ok {
		return cached, nil // degrade to last-known
	}
	return MCPServerShape{}, err
}

func (c *CachedQuerier) ListMCPServers(ctx context.Context, ns string) ([]MCPServerShape, error) {
	return c.inner.ListMCPServers(ctx, ns)
}

func (c *CachedQuerier) RegisterMCPServer(ctx context.Context, ns string, s MCPServerShape) error {
	return c.inner.RegisterMCPServer(ctx, ns, s)
}

func (c *CachedQuerier) SetMCPLifecycle(ctx context.Context, ns, name, ver, lc string) error {
	return c.inner.SetMCPLifecycle(ctx, ns, name, ver, lc)
}
