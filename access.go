package cacheinstance

import (
	"context"
	"fmt"
)

func (r *Registry) Get(kind, id string) (any, error) {
	kind, err := clean(kind)
	if err != nil {
		return nil, err
	}
	id, err = clean(id)
	if err != nil {
		return nil, err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.closed {
		return nil, ErrRegistryClosed
	}
	k := r.kinds[kind]
	if k == nil {
		return nil, ErrInstanceNotFound
	}
	e := k.entries[id]
	if e == nil || e.value == nil {
		return nil, ErrInstanceNotFound
	}
	return e.value, nil
}

func (r *Registry) Do(ctx context.Context, kind, id string, fn func(any) error) error {
	_, e, done, err := r.prepare(ctx, kind, id, false)
	if err != nil {
		return err
	}
	defer done()
	if fn == nil {
		return ErrInvalidArgument
	}
	if err = r.acquire(ctx, e); err != nil {
		return err
	}
	defer r.release(e)
	r.mu.RLock()
	value := e.value
	r.mu.RUnlock()
	if value == nil {
		return ErrInstanceNotFound
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = fn(value); err != nil {
		return fmt.Errorf("callback %s/%s: %w", kind, id, err)
	}
	return nil
}

func GetAs[T any](r *Registry, kind, id string) (T, error) {
	var zero T
	if r == nil {
		return zero, ErrInvalidArgument
	}
	value, err := r.Get(kind, id)
	if err != nil {
		return zero, err
	}
	typed, ok := value.(T)
	if !ok {
		return zero, fmt.Errorf("%s/%s: %w", kind, id, ErrTypeMismatch)
	}
	return typed, nil
}
