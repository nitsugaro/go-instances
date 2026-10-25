package cacheinstance

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
)

type waitingContext struct {
	context.Context
	once    sync.Once
	waiting chan struct{}
}

func (c *waitingContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.waiting) })
	return c.Context.Done()
}

func TestCancelWhileWaitingForInstance(t *testing.T) {
	r := testRegistry(t)
	require(t, r.Create(bg, "object", "id", json.RawMessage(`{}`)))
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		finished <- r.Do(bg, "object", "id", func(any) error { close(entered); <-release; return nil })
	}()
	<-entered
	ctx, cancel := context.WithCancel(bg)
	waiting := &waitingContext{Context: ctx, waiting: make(chan struct{})}
	result := make(chan error, 1)
	go func() { result <- r.Set(waiting, "object", "id", &testObject{Value: 9}) }()
	<-waiting.waiting
	cancel()
	if err := <-result; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	require(t, <-finished)
	value, err := GetAs[*testObject](r, "object", "id")
	require(t, err)
	if value.Value != 0 {
		t.Fatal("canceled replacement published")
	}
}

func TestCancelKindDeletionWhileRegistering(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	r := NewRegistry(CacheOptions{Loader: func(context.Context, string) ([]InstanceConfig, error) {
		close(entered)
		<-release
		return []InstanceConfig{{"id", json.RawMessage(`{}`)}}, nil
	}})
	registered := make(chan error, 1)
	go func() { registered <- r.Register(bg, "object", testFactory) }()
	<-entered
	ctx, cancel := context.WithCancel(bg)
	waiting := &waitingContext{Context: ctx, waiting: make(chan struct{})}
	deleted := make(chan error, 1)
	go func() { deleted <- r.DeleteKind(waiting, "object") }()
	<-waiting.waiting
	cancel()
	if err := <-deleted; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	close(release)
	require(t, <-registered)
	_, err := r.Get("object", "id")
	require(t, err)
}
