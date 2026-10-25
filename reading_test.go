package cacheinstance

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestReadersShareLock(t *testing.T) {
	r := testRegistry(t)
	require(t, r.Create(bg, "object", "id", json.RawMessage(`{}`)))
	r.mu.RLock()
	defer r.mu.RUnlock()
	done := make(chan struct{}, 40)
	for i := 0; i < 40; i++ {
		go func() {
			defer func() { done <- struct{}{} }()
			if _, err := r.Get("object", "id"); err != nil {
				t.Error(err)
			}
			if _, ok := r.Lookup("object", "id"); !ok {
				t.Error("missing instance")
			}
			if len(r.Kinds()) != 1 {
				t.Error("missing kind")
			}
			if instances, err := r.Instances("object"); err != nil || len(instances) != 1 {
				t.Error("missing instances", err)
			}
			if len(r.AllInstances()) != 1 {
				t.Error("missing snapshot")
			}
		}()
	}
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for i := 0; i < 40; i++ {
		select {
		case <-done:
		case <-timeout.C:
			t.Fatal("readers blocked behind another reader")
		}
	}
}

func TestReadsContinueDuringCallbacks(t *testing.T) {
	for _, callback := range []string{"loader", "factory", "commit", "do"} {
		t.Run(callback, func(t *testing.T) {
			entered := make(chan struct{})
			release := make(chan struct{})
			r := NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) {
				if callback == "loader" {
					close(entered)
					<-release
				}
				return nil, nil
			}})
			require(t, r.Set(bg, "object", "id", new(testObject)))
			if callback != "loader" {
				require(t, r.Register(bg, "object", func(ctx context.Context, raw json.RawMessage) (any, error) {
					if callback == "factory" {
						close(entered)
						<-release
					}
					return testFactory(ctx, raw)
				}))
			}
			finished := make(chan error, 1)
			go func() {
				switch callback {
				case "loader":
					finished <- r.Register(bg, "object", testFactory)
				case "factory":
					finished <- r.Update(bg, "object", "id", json.RawMessage(`{}`))
				case "commit":
					finished <- r.Set(bg, "object", "id", new(testObject), MutationOptions{Commit: func(context.Context) error { close(entered); <-release; return nil }})
				case "do":
					finished <- r.Do(bg, "object", "id", func(any) error { close(entered); <-release; return nil })
				}
			}()
			<-entered
			read := make(chan error, 1)
			go func() { _, err := r.Get("object", "id"); read <- err }()
			select {
			case err := <-read:
				close(release)
				require(t, err)
			case <-time.After(5 * time.Second):
				close(release)
				t.Fatal("callback blocked Get")
			}
			require(t, <-finished)
		})
	}
}
