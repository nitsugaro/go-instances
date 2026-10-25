package cacheinstance

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

func TestReplacementKeepsPreviousReadable(t *testing.T) {
	r := testRegistry(t)
	require(t, r.Create(bg, "object", "id", json.RawMessage(`{"Value":1}`)))
	old, _ := r.Get("object", "id")
	entered := make(chan struct{})
	release := make(chan struct{})
	result := make(chan error, 1)
	require(t, r.UpdateKind(bg, "object", func(context.Context, json.RawMessage) (any, error) {
		close(entered)
		<-release
		return &testObject{Value: 2}, nil
	}))
	go func() { result <- r.Update(bg, "object", "id", json.RawMessage(`{}`)) }()
	<-entered
	got, err := r.Get("object", "id")
	require(t, err)
	if got != old {
		t.Fatal("previous instance unavailable")
	}
	got, ok := r.Lookup("object", "id")
	if !ok || got != old {
		t.Fatal("lookup blocked or replaced")
	}
	require(t, r.Set(bg, "object", "other", new(testObject)))
	seen := make(chan int, 1)
	callbackDone := make(chan error, 1)
	go func() {
		callbackDone <- r.Do(bg, "object", "id", func(v any) error { seen <- v.(*testObject).Value; return nil })
	}()
	close(release)
	require(t, <-result)
	require(t, <-callbackDone)
	if value := <-seen; value != 2 {
		t.Fatal(value)
	}
}

func TestDifferentKeysBuildInParallel(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	r := NewRegistry(CacheOptions{})
	require(t, r.Register(bg, "object", func(context.Context, json.RawMessage) (any, error) {
		entered <- struct{}{}
		<-release
		return new(testObject), nil
	}))
	results := make(chan error, 2)
	for _, id := range []string{"a", "b"} {
		go func(id string) { results <- r.Create(bg, "object", id, json.RawMessage(`{}`)) }(id)
	}
	<-entered
	<-entered
	close(release)
	require(t, <-results)
	require(t, <-results)
}

func TestCancellationDoesNotPublish(t *testing.T) {
	r := testRegistry(t)
	ctx, cancel := context.WithCancel(bg)
	require(t, r.UpdateKind(bg, "object", func(context.Context, json.RawMessage) (any, error) { cancel(); return new(testObject), nil }))
	if err := r.Create(ctx, "object", "id", json.RawMessage(`{}`)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := r.Get("object", "id"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatal(err)
	}
	require(t, r.UpdateKind(bg, "object", testFactory))
	require(t, r.Create(bg, "object", "id", json.RawMessage(`{}`)))
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- r.Do(bg, "object", "id", func(any) error { close(entered); <-release; return nil }) }()
	<-entered
	ctx, cancel = context.WithCancel(bg)
	cancel()
	if err := r.Set(ctx, "object", "id", new(testObject)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := r.DeleteKind(ctx, "object"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	require(t, <-done)
}

func TestConcurrentMutations(t *testing.T) {
	r := testRegistry(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				switch i % 5 {
				case 0:
					_ = r.Create(bg, "object", "id", json.RawMessage(`{}`))
				case 1:
					_ = r.Set(bg, "object", "id", new(testObject))
				case 2:
					_ = r.Delete(bg, "object", "id")
				case 3:
					_ = r.Do(bg, "object", "id", func(any) error { return nil })
				case 4:
					_ = r.Update(bg, "object", "id", json.RawMessage(`{}`))
				}
				_, _ = r.Get("object", "id")
				_ = r.AllInstances()
			}
		}(i)
	}
	wg.Wait()
}
