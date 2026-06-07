package nacos

import (
	"context"
	"sync"
)

// CachedProviderQuerier wraps a ProviderQuerier with a last-known-good cache on
// GetProvider so a Nacos outage degrades gracefully (return cached shape)
// instead of failing dispatch. Writes and list pass through unchanged.
type CachedProviderQuerier struct {
	inner ProviderQuerier
	mu    sync.RWMutex
	cache map[string]ProviderShape
}

func NewCachedProviderQuerier(inner ProviderQuerier) *CachedProviderQuerier {
	return &CachedProviderQuerier{inner: inner, cache: map[string]ProviderShape{}}
}

func (c *CachedProviderQuerier) GetProvider(ctx context.Context, ns, name, ref string) (ProviderShape, error) {
	s, err := c.inner.GetProvider(ctx, ns, name, ref)
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
	return ProviderShape{}, err
}

func (c *CachedProviderQuerier) ListProviders(ctx context.Context, ns string) ([]ProviderShape, error) {
	return c.inner.ListProviders(ctx, ns)
}

func (c *CachedProviderQuerier) RegisterProvider(ctx context.Context, ns string, p ProviderShape) error {
	return c.inner.RegisterProvider(ctx, ns, p)
}

func (c *CachedProviderQuerier) SetProviderLifecycle(ctx context.Context, ns, name, ver, lc string) error {
	return c.inner.SetProviderLifecycle(ctx, ns, name, ver, lc)
}
