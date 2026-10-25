package cacheinstance

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func TestDeleteWaitsForConstruction(t *testing.T) {
	for _, scope := range []string{"instance", "kind", "registry"} {
		t.Run(scope, func(t *testing.T) {
			r := NewRegistry(CacheOptions{})
			entered := make(chan struct{})
			release := make(chan struct{})
			require(t, r.Register(bg, "object", func(context.Context, json.RawMessage) (any, error) {
				close(entered)
				<-release
				return new(testObject), nil
			}))
			created := make(chan error, 1)
			go func() { created <- r.Create(bg, "object", "id", json.RawMessage(`{}`)) }()
			<-entered
			deleted := make(chan error, 1)
			go func() {
				switch scope {
				case "instance":
					deleted <- r.Delete(bg, "object", "id")
				case "kind":
					deleted <- r.DeleteKind(bg, "object")
				case "registry":
					deleted <- r.Close(bg)
				}
			}()
			close(release)
			require(t, <-created)
			require(t, <-deleted)
			if _, ok := r.Lookup("object", "id"); ok {
				t.Fatal("resurrected")
			}
		})
	}
}

func TestCloseAndZeroRegistry(t *testing.T) {
	var r Registry
	require(t, r.Register(bg, "object", testFactory))
	require(t, r.Create(bg, "object", "id", json.RawMessage(`{}`)))
	require(t, r.Close(bg))
	for _, err := range []error{r.Create(bg, "object", "id", json.RawMessage(`{}`)), r.Register(bg, "new", testFactory), r.Set(bg, "object", "id", new(testObject))} {
		if !errors.Is(err, ErrRegistryClosed) {
			t.Fatal(err)
		}
	}
	if _, err := r.Get("object", "id"); !errors.Is(err, ErrRegistryClosed) {
		t.Fatal(err)
	}
	if len(r.Kinds())+len(r.AllInstances()) != 0 {
		t.Fatal("closed registry visible")
	}
}

func TestUpdateKindPreservesInstances(t *testing.T) {
	r := testRegistry(t)
	require(t, r.Create(bg, "object", "old", json.RawMessage(`{"Value":1}`)))
	require(t, r.UpdateKind(bg, "object", func(context.Context, json.RawMessage) (any, error) { return &testObject{Value: 2}, nil }))
	value, err := GetAs[*testObject](r, "object", "old")
	require(t, err)
	if value.Value != 1 {
		t.Fatal(value)
	}
	require(t, r.Create(bg, "object", "new", json.RawMessage(`{}`)))
	value, err = GetAs[*testObject](r, "object", "new")
	require(t, err)
	if value.Value != 2 {
		t.Fatal(value)
	}
	require(t, r.DeleteKind(bg, "object"))
	require(t, r.Register(bg, "object", testFactory))
	if _, err = r.Get("object", "old"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatal(err)
	}
}
