package main

import (
	"context"
	"sync"
)

type SafeContext struct {
	mu sync.Mutex
	v  map[string]int
}

type KVStorage interface {
	Get(ctx context.Context, key string) (interface{}, error)
	Put(ctx context.Context, key string, val interface{}) error
	Delete(ctx context.Context, key string) error
}

func (c *SafeContext) Get(ctx context.Context, key string) (value interface{}, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx != nil {
		value = ctx.Value(key)
	}
	return
}
func (c *SafeContext) Put(ctx context.Context, key string, val interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx != nil {
		ctx.WithValue(ctx, key, val)
	}
}

func (c *SafeContext) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
}
