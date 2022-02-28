package main

import (
	"context"
	"errors"
	"sync"
)

type SafeContext struct {
	mu sync.Mutex
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
		err = nil
	} else {
		err = errors.New("Empty Context")
	}
	return
}
func (c *SafeContext) Put(ctx context.Context, key string, val interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx != nil {
		context.WithValue(ctx, key, val)
		return nil
	} else {
		return errors.New("Empty Context")
	}
}

func (c *SafeContext) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ctx != nil {
		context.Delete(ctx, key)
		return nil
	} else {
		return errors.New("Empty Context")
	}
}
