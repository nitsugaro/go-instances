package cacheinstance

import (
	"context"
	"sort"
)

type kindState struct {
	gate    gate
	factory Factory
	entries map[string]*entry
}

func (r *Registry) kindEnter(ctx context.Context, kind string, write, create bool) (*kindState, error) {
	r.mu.Lock()
	if r.kinds == nil {
		r.kinds = make(map[string]*kindState)
	}
	k := r.kinds[kind]
	if k == nil && create {
		k = &kindState{entries: make(map[string]*entry)}
		r.kinds[kind] = k
	}
	r.mu.Unlock()
	if k == nil {
		return nil, ErrKindNotFound
	}
	if err := k.gate.lock(ctx, write); err != nil {
		return nil, err
	}
	r.mu.RLock()
	same := r.kinds[kind] == k
	r.mu.RUnlock()
	if !same {
		k.gate.unlock(write)
		return nil, ErrKindNotFound
	}
	return k, nil
}

func (r *Registry) changeKind(ctx context.Context, kind string, f Factory, remove bool) error {
	if err := r.enter(ctx); err != nil {
		return err
	}
	defer r.lifecycle.unlock(false)
	kind, err := clean(kind)
	if err != nil || (!remove && f == nil) {
		return ErrInvalidArgument
	}
	k, err := r.kindEnter(ctx, kind, true, false)
	if err != nil {
		return err
	}
	defer k.gate.unlock(true)
	r.mu.Lock()
	defer r.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return err
	}
	if remove {
		delete(r.kinds, kind)
	} else {
		if k.factory == nil {
			return ErrFactoryNotFound
		}
		k.factory = f
	}
	return nil
}

func (r *Registry) UpdateKind(ctx context.Context, kind string, f Factory) error {
	return r.changeKind(ctx, kind, f, false)
}

func (r *Registry) DeleteKind(ctx context.Context, kind string) error {
	return r.changeKind(ctx, kind, nil, true)
}

func (r *Registry) Kinds() []string {
	r.mu.RLock()
	out := []string{}
	if !r.closed {
		for name, k := range r.kinds {
			if k.factory == nil && !hasInstances(k) {
				continue
			}
			out = append(out, name)
		}
	}
	r.mu.RUnlock()
	sort.Strings(out)
	return out
}

func hasInstances(k *kindState) bool {
	for _, e := range k.entries {
		if e.value != nil {
			return true
		}
	}
	return false
}
