package cacheinstance

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRegisterLoadsOnlyItsKind(t *testing.T) {
	var calls []string
	r := NewRegistry(CacheOptions{Loader: func(_ context.Context, kind string) ([]InstanceConfig, error) {
		calls = append(calls, kind)
		return []InstanceConfig{{ID: "saved", Config: json.RawMessage(`{"Value":7}`)}}, nil
	}})
	require(t, r.Register(bg, " first ", testFactory))
	a, err := GetAs[*testObject](r, "first", "saved")
	require(t, err)
	if a.Value != 7 {
		t.Fatal(a)
	}
	if !reflect.DeepEqual(calls, []string{"first"}) {
		t.Fatal(calls)
	}
	if _, err = r.Get("second", "saved"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatal(err)
	}
	require(t, r.Register(bg, "second", testFactory))
	b, err := r.Get("second", "saved")
	require(t, err)
	if a == b {
		t.Fatal("shared instance across kinds")
	}
	require(t, r.Create(bg, "first", "runtime", json.RawMessage(`{"Value":9}`)))
	require(t, r.Update(bg, "first", "runtime", json.RawMessage(`{"Value":10}`)))
	require(t, r.Delete(bg, "first", "saved"))
	if _, err = r.Get("first", "saved"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatal(calls)
	}
}

func TestRegistrationFailureIsRetryable(t *testing.T) {
	failure := errors.New("storage unavailable")
	attempt := 0
	r := NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) {
		attempt++
		if attempt == 1 {
			return nil, failure
		}
		if attempt == 2 {
			return []InstanceConfig{{"a", json.RawMessage(`{}`)}, {"b", json.RawMessage(`bad`)}}, nil
		}
		return []InstanceConfig{{"a", json.RawMessage(`{}`)}}, nil
	}})
	for _, want := range []error{failure, ErrInvalidJSON} {
		if err := r.Register(bg, "object", testFactory); !errors.Is(err, want) {
			t.Fatal(err)
		}
		if len(r.Kinds()) != 0 || len(r.AllInstances()) != 0 {
			t.Fatal("partial registration")
		}
	}
	require(t, r.Register(bg, "object", testFactory))
	if _, err := r.Get("object", "a"); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(bg, "object", testFactory); !errors.Is(err, ErrKindExists) {
		t.Fatal(err)
	}
	if attempt != 3 {
		t.Fatal(attempt)
	}
}

func TestDuplicatePreloadPreservesRuntime(t *testing.T) {
	r := NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) {
		return []InstanceConfig{{"id", json.RawMessage(`{}`)}}, nil
	}})
	old := new(testObject)
	require(t, r.Set(bg, "object", "id", old))
	if err := r.Register(bg, "object", testFactory); !errors.Is(err, ErrInstanceExists) {
		t.Fatal(err)
	}
	got, err := r.Get("object", "id")
	require(t, err)
	if got != old {
		t.Fatal("runtime overwritten")
	}
}

func TestConcurrentRegister(t *testing.T) {
	var loads, winners atomic.Int32
	r := NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) { loads.Add(1); return nil, nil }})
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := r.Register(bg, "object", testFactory)
			if err == nil {
				winners.Add(1)
			} else if !errors.Is(err, ErrKindExists) {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if loads.Load() != 1 || winners.Load() != 1 {
		t.Fatal(loads.Load(), winners.Load())
	}
}

func TestKindsInitializeIndependently(t *testing.T) {
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	r := NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) {
		entered <- struct{}{}
		<-release
		return nil, nil
	}})
	results := make(chan error, 2)
	for _, kind := range []string{"first", "second"} {
		go func(kind string) { results <- r.Register(bg, kind, testFactory) }(kind)
	}
	<-entered
	<-entered
	if len(r.Kinds()) != 0 {
		t.Fatal("unfinished kinds visible")
	}
	close(release)
	require(t, <-results)
	require(t, <-results)
}

func TestRegistrationCancellationAndPanic(t *testing.T) {
	ctx, cancel := context.WithCancel(bg)
	r := NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) {
		cancel()
		return []InstanceConfig{{"id", json.RawMessage(`{}`)}}, nil
	}})
	if err := r.Register(ctx, "object", testFactory); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(r.Kinds()) != 0 {
		t.Fatal("canceled registration published")
	}
	r = NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) {
		return []InstanceConfig{{"id", json.RawMessage(`{}`)}}, nil
	}})
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic hidden")
			}
		}()
		_ = r.Register(bg, "object", func(context.Context, json.RawMessage) (any, error) { panic("factory") })
	}()
	require(t, r.Register(bg, "object", testFactory))
}
