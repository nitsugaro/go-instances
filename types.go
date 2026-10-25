package cacheinstance

import (
	"context"
	"encoding/json"
)

type Factory func(context.Context, json.RawMessage) (any, error)
type ConfigLoader func(context.Context, string) ([]InstanceConfig, error)
type CacheOptions struct{ Loader ConfigLoader }
type InstanceConfig struct {
	ID     string
	Config json.RawMessage
}
type MutationOptions struct{ Commit func(context.Context) error }
type InstanceInfo struct{ Kind, ID, Source string }
