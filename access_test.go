package cacheinstance

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestGetNeverLoadsOrBuilds(t *testing.T) {
	r := NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) { t.Fatal("unexpected load"); return nil, nil }})
	if _, err := r.Get("missing", "id"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatal(err)
	}
	if _, ok := r.Lookup("missing", "id"); ok {
		t.Fatal("unexpected instance")
	}
	if len(r.AllInstances()) != 0 {
		t.Fatal("unexpected instances")
	}
	r = testRegistry(t)
	if _, err := r.Get("object", "id"); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatal(err)
	}
	if err := r.Do(bg, "object", "id", func(any) error { t.Fatal("callback called"); return nil }); !errors.Is(err, ErrInstanceNotFound) {
		t.Fatal(err)
	}
}

func TestDoSerializesAndReleasesOnPanic(t *testing.T) {
	r := testRegistry(t)
	require(t, r.Create(bg, "object", "id", json.RawMessage(`{}`)))
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := r.Do(bg, "object", "id", func(v any) error { v.(*testObject).Value++; return nil }); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	value, err := GetAs[*testObject](r, "object", "id")
	require(t, err)
	if value.Value != 100 {
		t.Fatal(value)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("panic hidden")
			}
		}()
		_ = r.Do(bg, "object", "id", func(any) error { panic("callback") })
	}()
	failure := errors.New("callback failed")
	if err = r.Do(bg, "object", "id", func(any) error { return failure }); !errors.Is(err, failure) {
		t.Fatal(err)
	}
	require(t, r.Set(bg, "object", "id", new(testObject)))
}

func TestSnapshotsAndIsolation(t *testing.T) {
	r := testRegistry(t)
	other := testRegistry(t)
	require(t, r.Create(bg, "object", "z", json.RawMessage(`{}`)))
	require(t, r.Create(bg, "object", "a", json.RawMessage(`{}`)))
	require(t, r.Set(bg, "aaa", "runtime", new(testObject)))
	want := []InstanceInfo{{"aaa", "runtime", "runtime"}, {"object", "a", "factory"}, {"object", "z", "factory"}}
	got := r.AllInstances()
	if !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	got[0].ID = "changed"
	if !reflect.DeepEqual(r.AllInstances(), want) {
		t.Fatal("snapshot alias")
	}
	if len(other.AllInstances()) != 0 {
		t.Fatal("registry leaked")
	}
	kinds := r.Kinds()
	kinds[0] = "changed"
	if !reflect.DeepEqual(r.Kinds(), []string{"aaa", "object"}) {
		t.Fatal(r.Kinds())
	}
	if _, err := r.Instances("unknown"); !errors.Is(err, ErrKindNotFound) {
		t.Fatal(err)
	}
}
