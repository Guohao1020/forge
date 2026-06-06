package nacos

import (
	"context"
	"errors"
	"testing"
)

type fakeQ struct {
	shape MCPServerShape
	err   error
	calls int
}

func (f *fakeQ) ListMCPServers(context.Context, string) ([]MCPServerShape, error) { return nil, f.err }
func (f *fakeQ) GetMCPServer(context.Context, string, string, string) (MCPServerShape, error) {
	f.calls++
	return f.shape, f.err
}
func (f *fakeQ) RegisterMCPServer(context.Context, string, MCPServerShape) error      { return f.err }
func (f *fakeQ) SetMCPLifecycle(context.Context, string, string, string, string) error { return f.err }

func TestCachedQuerier_FallsBackOnError(t *testing.T) {
	f := &fakeQ{shape: MCPServerShape{Name: "voc", Version: "1.0.0"}}
	c := NewCachedQuerier(f)

	// 1st call: success → caches.
	if _, err := c.GetMCPServer(context.Background(), "ws1", "voc", "stable"); err != nil {
		t.Fatalf("warm: %v", err)
	}
	// 2nd call: underlying fails → must return cached, no error.
	f.err = errors.New("nacos down")
	got, err := c.GetMCPServer(context.Background(), "ws1", "voc", "stable")
	if err != nil || got.Name != "voc" {
		t.Fatalf("expected cached fallback, got %+v err=%v", got, err)
	}
}

func TestCachedQuerier_NoCacheReturnsError(t *testing.T) {
	f := &fakeQ{err: errors.New("nacos down")}
	c := NewCachedQuerier(f)
	if _, err := c.GetMCPServer(context.Background(), "ws1", "missing", "stable"); err == nil {
		t.Fatal("expected error when nothing cached")
	}
}
