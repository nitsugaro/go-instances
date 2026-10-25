package cacheinstance_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cacheinstance "github.com/nitsugaro/go-instances"
)

func loadSavedConfigs(ctx context.Context, kind string) ([]cacheinstance.InstanceConfig, error) {
	if kind != "http-client" {
		return nil, nil
	}
	return []cacheinstance.InstanceConfig{
		{ID: "payments", Config: json.RawMessage(`{"timeoutSeconds":10}`)},
		{ID: "notifications", Config: json.RawMessage(`{"timeoutSeconds":30}`)},
	}, nil
}

func buildHTTPClient(ctx context.Context, raw json.RawMessage) (any, error) {
	var config struct {
		TimeoutSeconds int `json:"timeoutSeconds"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	return &http.Client{Timeout: time.Duration(config.TimeoutSeconds) * time.Second}, nil
}

func setupHTTP(ctx context.Context, registry *cacheinstance.Registry) error {
	return registry.Register(ctx, "http-client", buildHTTPClient)
}

func ExampleRegistry_Register() {
	ctx := context.Background()
	registry := cacheinstance.NewRegistry(cacheinstance.CacheOptions{Loader: loadSavedConfigs})
	if err := setupHTTP(ctx, registry); err != nil {
		panic(err)
	}
	instance, err := registry.Get("http-client", "payments")
	if err != nil {
		panic(err)
	}
	fmt.Println(instance.(*http.Client).Timeout)
	if err := registry.Create(ctx, "http-client", "reports", json.RawMessage(`{"timeoutSeconds":5}`)); err != nil {
		panic(err)
	}
	client, err := cacheinstance.GetAs[*http.Client](registry, "http-client", "reports")
	if err != nil {
		panic(err)
	}
	fmt.Println(client.Timeout)
	// Output:
	// 10s
	// 5s
}
