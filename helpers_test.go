package cacheinstance

import (
	"context"
	"encoding/json"
	"testing"
)

var bg = context.Background()

type testObject struct{ Value int }

func require(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func testFactory(_ context.Context, raw json.RawMessage) (any, error) {
	value := new(testObject)
	return value, json.Unmarshal(raw, value)
}
func testRegistry(t *testing.T) *Registry {
	t.Helper()
	r := NewRegistry(CacheOptions{})
	require(t, r.Register(bg, "object", testFactory))
	return r
}
