package cacheinstance

import (
	"context"
)

func (r *Registry) enter(ctx context.Context) error {
	if err := r.lifecycle.lock(ctx, false); err != nil {
		return err
	}
	r.mu.RLock()
	closed := r.closed
	r.mu.RUnlock()
	if closed {
		r.lifecycle.unlock(false)
		return ErrRegistryClosed
	}
	return nil
}

func (r *Registry) prepare(ctx context.Context, kind, id string, create bool) (*kindState, *entry, func(), error) {
	if err := r.enter(ctx); err != nil {
		return nil, nil, nil, err
	}
	fail := func() { r.lifecycle.unlock(false) }
	kind, err := clean(kind)
	if err != nil {
		fail()
		return nil, nil, nil, err
	}
	id, err = clean(id)
	if err != nil {
		fail()
		return nil, nil, nil, err
	}
	k, err := r.kindEnter(ctx, kind, false, create)
	if err != nil {
		fail()
		return nil, nil, nil, err
	}
	done := func() { k.gate.unlock(false); fail() }
	r.mu.Lock()
	e := k.entries[id]
	if e == nil {
		e = &entry{changed: make(chan struct{})}
		k.entries[id] = e
	}
	r.mu.Unlock()
	return k, e, done, nil
}
