// Package jobs owns a window's background work and cancels it before shutdown.
package jobs

import (
	"context"
	"sync"
)

type Group struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	closed bool
}

func New() *Group {
	ctx, cancel := context.WithCancel(context.Background())
	return &Group{ctx: ctx, cancel: cancel}
}
func (g *Group) Go(fn func(context.Context)) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return false
	}
	g.wg.Add(1)
	go func() { defer g.wg.Done(); fn(g.ctx) }()
	return true
}
func (g *Group) Close() { g.mu.Lock(); g.closed = true; g.cancel(); g.mu.Unlock(); g.wg.Wait() }
