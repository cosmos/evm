package stream

import (
	"context"
	"sync"
)

// Cond implements conditional variable with a channel
type Cond struct {
	mu sync.Mutex // guards ch
	ch chan struct{}
}

func NewCond() *Cond {
	return &Cond{ch: make(chan struct{})}
}

func (c *Cond) Channel() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.ch
}

// Wait returns true if the condition is signaled, false if the context is canceled
func (c *Cond) Wait(ctx context.Context) bool {
	return c.WaitChannel(ctx, c.Channel())
}

func (c *Cond) WaitChannel(ctx context.Context, ch <-chan struct{}) bool {

	select {
	case <-ch:
		return true
	case <-ctx.Done():
		return false
	}
}

func (c *Cond) Broadcast() {
	c.mu.Lock()
	defer c.mu.Unlock()
	close(c.ch)
	c.ch = make(chan struct{})
}
