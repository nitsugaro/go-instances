# go-instances

An in-memory cache of shared objects, identified by type (`kind`) and name (`id`).
Each registry is independent: create it once and pass it to the components that
need to share instances. There is no global registry or provider layer.

Import the module as `cacheinstance "github.com/nitsugaro/go-instances"`. Requires Go 1.24 or later and
has no external dependencies. Cached data is lost when the process exits.

## Installation

```sh
go get github.com/nitsugaro/go-instances@v0.1.0
```

## Complete example

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    cacheinstance "github.com/nitsugaro/go-instances"
)

func loadSavedConfigs(ctx context.Context, kind string) ([]cacheinstance.InstanceConfig, error) {
    switch kind {
    case "http-client":
        return []cacheinstance.InstanceConfig{
            {ID: "payments", Config: json.RawMessage(`{"timeoutSeconds":10}`)},
            {ID: "notifications", Config: json.RawMessage(`{"timeoutSeconds":30}`)},
        }, nil
    default:
        return nil, nil
    }
}

func buildHTTPClient(ctx context.Context, raw json.RawMessage) (any, error) {
    var config struct {
        TimeoutSeconds int `json:"timeoutSeconds"`
    }
    if err := json.Unmarshal(raw, &config); err != nil {
        return nil, err
    }
    return &http.Client{
        Timeout: time.Duration(config.TimeoutSeconds) * time.Second,
    }, nil
}

func setupHTTP(ctx context.Context, registry *cacheinstance.Registry) error {
    return registry.Register(ctx, "http-client", buildHTTPClient)
}

func run(ctx context.Context) error {
    registry := cacheinstance.NewRegistry(cacheinstance.CacheOptions{
        Loader: loadSavedConfigs,
    })
    if err := setupHTTP(ctx, registry); err != nil {
        return err
    }

    instance, err := registry.Get("http-client", "payments")
    if err != nil {
        return err
    }
    fmt.Println(instance.(*http.Client).Timeout)

    if err := registry.Create(ctx, "http-client", "reports",
        json.RawMessage(`{"timeoutSeconds":5}`),
    ); err != nil {
        return err
    }

    client, err := cacheinstance.GetAs[*http.Client](registry, "http-client", "reports")
    if err != nil {
        return err
    }
    fmt.Println(client.Timeout)
    return nil
}

func main() {
    if err := run(context.Background()); err != nil {
        panic(err)
    }
}
```

Prints `10s` and `5s`. An [executable test example](example_test.go) is also available.

## Registration and configuration loading

Each component registers its types during initialization, as `setupHTTP` does.
It does not need to wait for other components to register theirs.

`Register(ctx, kind, factory)` calls the configured loader for **that type**. The
loader receives `ctx` and `kind` and returns a list of `InstanceConfig` values, each
with an `ID` and a `Config` (`json.RawMessage`). Your application chooses where to
read these configurations: files, a database, or another source. The example returns
fixed data.

The factory converts each JSON configuration into an object. If every construction
succeeds, the type and its instances are published. `Register` waits until loading
finishes. There is no separate `Preload` call.

If loading or construction fails, neither the factory nor any new instances are
published; existing instances are preserved. Fix the error and retry. Duplicate IDs
in the loaded configurations or collisions with cached instances produce
`ErrInstanceExists`. Registering a type that already has a factory produces
`ErrKindExists` without calling the loader again.

Without saved configurations, use
`cacheinstance.NewRegistry(cacheinstance.CacheOptions{})`: `Register` only registers
the factory. The loader may run concurrently for different types and must support
that usage.

## Creating, retrieving, and modifying instances

`Create` takes JSON directly and uses the registered factory. It rejects an existing
key, does not call the loader, and does not persist anything. `Get` only reads memory:
it does not construct objects, wait for their construction to finish, or call the
loader. If an instance is missing or has not been published yet, it returns
`ErrInstanceNotFound`.

| Registry method | Behavior |
| --- | --- |
| `Register(ctx, kind, factory)` | Registers the type and loads its saved configurations. |
| `Create(ctx, kind, id, config)` | Constructs and adds a new instance. |
| `Get(kind, id)` | Returns `(any, error)` containing the published pointer. |
| `Lookup(kind, id)` | Returns `(any, bool)` without constructing anything. |
| `Update(ctx, kind, id, config, options...)` | Rebuilds an existing instance from the supplied JSON. |
| `UpdateJSON(ctx, kind, id, config, options...)` | Constructs from JSON and adds or replaces an instance. |
| `Set(ctx, kind, id, value, options...)` | Adds or replaces an existing pointer; allows types without a factory. |
| `Delete(ctx, kind, id, options...)` | Removes an instance from memory. |
| `Do(ctx, kind, id, fn)` | Runs `func(any) error` on an existing instance with exclusive access for that key. |
| `Kinds()` | Returns sorted visible types, including those created with `Set`. |
| `Instances(kind)` | Returns `([]InstanceInfo, error)` for the specified type. |
| `AllInstances()` | Returns `[]InstanceInfo` for the entire cache. |
| `UpdateKind(ctx, kind, factory)` | Replaces an existing factory for future constructions. |
| `DeleteKind(ctx, kind)` | Removes the factory and every instance of that type. |
| `Close(ctx)` | Empties and closes the registry; does not close the objects. |

`options...` accepts zero or one `cacheinstance.MutationOptions` value. `Create`
does not accept these options. `Update`, `UpdateJSON`, and `UpdateKind` never call
the loader. Deleting an instance does not trigger reloading on the next `Get`.

`cacheinstance.GetAs[T](registry, kind, id)` retrieves an instance as a concrete type;
it returns `ErrTypeMismatch` if the object does not match that type.

Listings include only published instances, sorted by type and ID. `InstanceInfo`
contains `Kind`, `ID`, and `Source`: `"factory"` for objects built by a factory, or
`"runtime"` for objects added with `Set`. Calling `Create` at runtime also produces
source `"factory"`. Listings expose neither configurations nor objects, and a later
lookup may observe changes made since the listing was obtained.

## Validation and errors

Types and IDs are trimmed with `TrimSpace`, must not be empty, and are case-sensitive.
Factories and `Do` callbacks must not be nil. Factories and `Set` must supply non-nil
pointers. JSON is validated and copied before being passed to the factory.

Errors support `errors.Is`. A missing key in `Get` produces `ErrInstanceNotFound`,
even if its type is also missing. For operations requiring a registered type, an
unknown type produces `ErrKindNotFound`; a type created only with `Set` may produce
`ErrFactoryNotFound` when attempting to construct from JSON.

After `Close`, calls to `Get` with valid arguments return `ErrRegistryClosed`,
`Lookup` returns `false`, and `Kinds` and `AllInstances` are empty. You cannot
register types or add instances to that registry again.

## External persistence

The loader reads configurations only during `Register`. Creating, modifying, or
deleting instances does not automatically save changes to their source.

`MutationOptions.Commit` is a `func(context.Context) error` implemented by your
application. It runs after the candidate is constructed and validated, before
publication; for `Delete`, it runs before removal. For example, `UpdateJSON` can
receive JSON with resolved secrets while the commit saves the original with
placeholders.

If construction fails, the commit does not run. If the commit fails, the previous
instance is preserved. If it returns nil, the change is published even if the
context was canceled during the commit. Your storage must ensure that a failed
commit leaves no partial changes: the library does not roll them back or provide
a transaction spanning storage and memory in the event of a process crash.

## Concurrency and resource ownership

`Get`, `Lookup`, and listings allow concurrent readers. They are not lock-free:
an active or pending write to the registry's internal state can make new reads
wait, even for another key. There is no guaranteed maximum read latency.

The loader, factories, and commits run without holding the lock protecting those
reads. During replacement, `Get` and `Lookup` can continue returning the previous
object; once the change is published, subsequent lookups return the new pointer.

Using an object after `Get` is not protected by the registry. For objects that are
not safe for concurrent use, all consumers must access them through `Do`. `Do`
coordinates with other `Do` calls, creations, replacements, and deletions for the
same key; **it does not block `Get` or `Lookup`**. Different keys can operate in
parallel, although changing or deleting a type coordinates operations for that
type, and `Close` coordinates the entire registry. Callbacks must not reenter
the same registry.

Operations accepting a context can be canceled while waiting or constructing.
Loaders and factories must respect the context: they cannot be forcibly interrupted
if they ignore it.

Objects are never closed automatically because consumers may still hold their
pointers. Your application owns the resources of replaced or deleted objects and
candidates that were never published, including those constructed during a failed
preload.

## Verification

```sh
go test ./...
go test -race ./...
```

## License

[MIT](LICENSE). Copyright (c) 2026 nitsugaro.
