package cacheinstance

import (
	"context"
)

type entry struct {
	value   any
	source  string
	busy    bool
	changed chan struct{}
}

func signal(e *entry) {
	close(e.changed)
	e.changed = make(chan struct{})
}

func (r *Registry) release(e *entry) {
	r.mu.Lock()
	e.busy = false
	signal(e)
	r.mu.Unlock()
}

func (r *Registry) acquire(ctx context.Context, e *entry) error {
	for {
		r.mu.Lock()
		if err := ctx.Err(); err != nil {
			r.mu.Unlock()
			return err
		}
		if !e.busy {
			e.busy = true
			r.mu.Unlock()
			return nil
		}
		ch := e.changed
		r.mu.Unlock()
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
