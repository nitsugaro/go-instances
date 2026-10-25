package cacheinstance

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestConcurrentCreateBuildsOnce(t *testing.T) {
	r := NewRegistry(CacheOptions{})
	var calls, winners atomic.Int32
	require(t, r.Register(bg, "object", func(ctx context.Context, raw json.RawMessage) (any, error) {
		calls.Add(1)
		return testFactory(ctx, raw)
	}))
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := r.Create(bg, "object", "id", json.RawMessage(`{}`))
			if err == nil {
				winners.Add(1)
			} else if !errors.Is(err, ErrInstanceExists) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 || winners.Load() != 1 {
		t.Fatal(calls.Load(), winners.Load())
	}
	a, err := r.Get("object", "id")
	require(t, err)
	b, err := r.Get("object", "id")
	require(t, err)
	if a != b {
		t.Fatal("identity lost")
	}
}

func TestMutationRollbackAndCommit(t *testing.T) {
	r := testRegistry(t)
	require(t, r.Create(bg, "object", "id", json.RawMessage(`{"Value":1}`)))
	old, _ := r.Get("object", "id")
	failure := errors.New("commit failed")
	options := MutationOptions{Commit: func(context.Context) error { return failure }}
	if err := r.Update(bg, "object", "id", json.RawMessage(`{"Value":2}`), options); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	got, _ := r.Get("object", "id")
	if got != old {
		t.Fatal("failed update published")
	}
	if err := r.Delete(bg, "object", "id", options); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	got, _ = r.Get("object", "id")
	if got != old {
		t.Fatal("failed deletion published")
	}
	called := false
	if err := r.UpdateJSON(bg, "object", "id", json.RawMessage(`bad`), MutationOptions{Commit: func(context.Context) error { called = true; return nil }}); !errors.Is(err, ErrInvalidJSON) || called {
		t.Fatal(err, called)
	}
	ctx, cancel := context.WithCancel(bg)
	require(t, r.Update(bg, "object", "id", json.RawMessage(`{"Value":3}`), MutationOptions{Commit: func(context.Context) error { cancel(); return nil }}))
	got, _ = r.Get("object", "id")
	if got.(*testObject).Value != 3 {
		t.Fatal(got)
	}
	ctx, cancel = context.WithCancel(bg)
	require(t, r.Delete(ctx, "object", "id", MutationOptions{Commit: func(context.Context) error { cancel(); return nil }}))
	if _, err := r.Get("object", "id"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatal(err)
	}
	if err := r.Update(bg, "object", "id", json.RawMessage(`{}`)); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatal(err)
	}
}

func TestResolvedJSONAndExternalStorage(t *testing.T) {
	r := testRegistry(t)
	original := json.RawMessage(`{"Value":"${VALUE}"}`)
	var stored json.RawMessage
	require(t, r.UpdateJSON(bg, "object", "id", json.RawMessage(`{"Value":4}`), MutationOptions{Commit: func(context.Context) error { stored = append(json.RawMessage(nil), original...); return nil }}))
	value, err := GetAs[*testObject](r, "object", "id")
	require(t, err)
	if value.Value != 4 || string(stored) != string(original) {
		t.Fatal(value, string(stored))
	}
	require(t, r.Set(bg, "object", "id", new(testObject), MutationOptions{Commit: func(context.Context) error { stored = nil; return nil }}))
	if stored != nil {
		t.Fatal("external cleanup skipped")
	}
}

func TestFactoryFailurePreservesInstance(t *testing.T) {
	r := testRegistry(t)
	require(t, r.Create(bg, "object", "id", json.RawMessage(`{}`)))
	old, _ := r.Get("object", "id")
	failure := errors.New("factory failed")
	require(t, r.UpdateKind(bg, "object", func(context.Context, json.RawMessage) (any, error) { return nil, failure }))
	if err := r.Update(bg, "object", "id", json.RawMessage(`{}`)); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	got, _ := r.Get("object", "id")
	if got != old {
		t.Fatal("failed factory published")
	}
	require(t, r.UpdateKind(bg, "object", testFactory))
	require(t, r.Update(bg, "object", "id", json.RawMessage(`{}`)))
}
