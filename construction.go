package cacheinstance

import (
	"context"
	"encoding/json"
	"fmt"
)

func build(ctx context.Context, factory Factory, kind, id string, config json.RawMessage) (any, error) {
	if factory == nil {
		return nil, ErrFactoryNotFound
	}
	config = append(json.RawMessage(nil), config...)
	if !json.Valid(config) {
		return nil, fmt.Errorf("%s/%s: %w", kind, id, ErrInvalidJSON)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	value, err := factory(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("factory %s/%s: %w", kind, id, err)
	}
	if !validInstance(value) {
		return nil, fmt.Errorf("%s/%s: %w", kind, id, ErrInvalidInstance)
	}
	return value, nil
}

func (r *Registry) build(ctx context.Context, k *kindState, kind, id string, config json.RawMessage) (any, error) {
	r.mu.RLock()
	factory := k.factory
	r.mu.RUnlock()
	return build(ctx, factory, kind, id, config)
}
