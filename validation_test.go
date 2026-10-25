package cacheinstance

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestValidation(t *testing.T) {
	r := testRegistry(t)
	for _, err := range []error{
		r.Register(bg, "", testFactory), r.Register(bg, "other", nil),
		r.Create(bg, "object", "", json.RawMessage(`{}`)),
		r.Set(bg, "object", "id", new(testObject), MutationOptions{}, MutationOptions{}),
		r.Do(bg, "object", "id", nil),
	} {
		if !errors.Is(err, ErrInvalidArgument) {
			t.Fatal(err)
		}
	}
	for _, value := range []any{nil, (*testObject)(nil), testObject{}} {
		if err := r.Set(bg, "object", "id", value); !errors.Is(err, ErrInvalidInstance) {
			t.Fatal(err)
		}
	}
	if err := r.Create(bg, "object", "id", json.RawMessage(`bad`)); !errors.Is(err, ErrInvalidJSON) {
		t.Fatal(err)
	}
	require(t, r.UpdateKind(bg, "object", func(context.Context, json.RawMessage) (any, error) { return (*testObject)(nil), nil }))
	if err := r.Create(bg, "object", "id", json.RawMessage(`{}`)); !errors.Is(err, ErrInvalidInstance) {
		t.Fatal(err)
	}
	require(t, r.Set(bg, "object", "id", new(testObject)))
	if _, err := GetAs[*int](r, "object", "id"); !errors.Is(err, ErrTypeMismatch) {
		t.Fatal(err)
	}
}

func TestFactoryReceivesCopy(t *testing.T) {
	r := NewRegistry(CacheOptions{})
	raw := json.RawMessage(`{}`)
	require(t, r.Register(bg, "object", func(_ context.Context, config json.RawMessage) (any, error) {
		config[0] = '!'
		return new(testObject), nil
	}))
	require(t, r.Create(bg, "object", "id", raw))
	if string(raw) != `{}` {
		t.Fatal(string(raw))
	}
}

func TestDuplicateSavedIDs(t *testing.T) {
	r := NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) {
		return []InstanceConfig{{"id", json.RawMessage(`{}`)}, {" id ", json.RawMessage(`{}`)}}, nil
	}})
	if err := r.Register(bg, "object", testFactory); !errors.Is(err, ErrInstanceExists) {
		t.Fatal(err)
	}
	if len(r.Kinds()) != 0 || len(r.AllInstances()) != 0 {
		t.Fatal("partial registration")
	}
}

func TestFactoryPanicReleasesKey(t *testing.T) {
	r := NewRegistry(CacheOptions{})
	require(t, r.Register(bg, "object", func(context.Context, json.RawMessage) (any, error) { panic("factory") }))
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic hidden")
			}
		}()
		_ = r.Create(bg, "object", "id", json.RawMessage(`{}`))
	}()
	require(t, r.Set(bg, "object", "id", new(testObject)))
}
