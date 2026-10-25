package cacheinstance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func (r *Registry) mutate(ctx context.Context, kind, id, op string, value any, config json.RawMessage, options []MutationOptions) error {
	kind = strings.TrimSpace(kind)
	id = strings.TrimSpace(id)
	k, e, done, err := r.prepare(ctx, kind, id, op == "set")
	if err != nil {
		return err
	}
	defer done()
	if len(options) > 1 {
		return ErrInvalidArgument
	}
	if err = r.acquire(ctx, e); err != nil {
		return err
	}
	defer r.release(e)
	r.mu.RLock()
	old := e.value
	r.mu.RUnlock()
	if (op == "update" || op == "delete") && old == nil {
		return ErrInstanceNotFound
	}
	if op == "create" && old != nil {
		return ErrInstanceExists
	}
	source := "factory"
	switch op {
	case "create", "update", "json":
		value, err = r.build(ctx, k, kind, id, config)
		if err != nil {
			return err
		}
	case "set":
		if !validInstance(value) {
			return ErrInvalidInstance
		}
		source = "runtime"
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	committed := len(options) == 1 && options[0].Commit != nil
	if committed {
		if err = options[0].Commit(ctx); err != nil {
			return fmt.Errorf("commit %s/%s: %w", kind, id, err)
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if !committed {
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	if op == "delete" {
		e.value = nil
		e.source = ""
	} else {
		e.value = value
		e.source = source
	}
	return nil
}

func (r *Registry) Update(ctx context.Context, kind, id string, config json.RawMessage, options ...MutationOptions) error {
	return r.mutate(ctx, kind, id, "update", nil, config, options)
}

func (r *Registry) UpdateJSON(ctx context.Context, kind, id string, config json.RawMessage, options ...MutationOptions) error {
	return r.mutate(ctx, kind, id, "json", nil, config, options)
}

func (r *Registry) Set(ctx context.Context, kind, id string, value any, options ...MutationOptions) error {
	return r.mutate(ctx, kind, id, "set", value, nil, options)
}

func (r *Registry) Delete(ctx context.Context, kind, id string, options ...MutationOptions) error {
	return r.mutate(ctx, kind, id, "delete", nil, nil, options)
}

func (r *Registry) Create(ctx context.Context, kind, id string, config json.RawMessage) error {
	return r.mutate(ctx, kind, id, "create", nil, config, nil)
}
