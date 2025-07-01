package main

import (
	"context"
	"time"

	"github.com/Mattilsynet/map-nats-kv/bindings/exports/mattilsynet/map_kv/key_value"
	"github.com/Mattilsynet/map-nats-kv/bindings/mattilsynet/map_kv/key_value_watcher"
	"github.com/Mattilsynet/map-nats-kv/bindings/mattilsynet/map_kv/types"
	"github.com/Mattilsynet/map-nats-kv/pkg/config"
	"github.com/Mattilsynet/map-nats-kv/pkg/pkgnats"
	"github.com/Mattilsynet/map-nats-kv/pkg/secrets"
	"github.com/nats-io/nats.go"
	sdk "go.wasmcloud.dev/provider"
	wrpc "wrpc.io/go"
	wrpcnats "wrpc.io/go/nats"
)

type KvHandler struct {
	// The provider instance
	provider   *sdk.WasmcloudProvider
	linkedFrom map[string]map[string]string
	linkedTo   map[string]map[string]string
	ncMap      map[string]*nats.Conn
	configs    map[string]*config.Config
}

func stuff() {
}

func NewKvHandler(linkedFrom, linkedTo map[string]map[string]string) *KvHandler {
	return &KvHandler{
		linkedFrom: linkedFrom,
		linkedTo:   linkedTo,
		configs:    make(map[string]*config.Config),
		ncMap:      make(map[string]*nats.Conn),
	}
}

func (ha *KvHandler) RegisterComponent(sourceID string, target string, config *config.Config, secrets *secrets.Secrets) error {
	url := config.NatsURL
	nc, err := pkgnats.CreateNatsConnection(target, secrets.NatsCredentials, url)
	if err != nil {
		ha.provider.Logger.Error("Failed to create (key-value) NATS connection", "sourceId", sourceID, "target", target, "error", err)
		return err
	}
	ha.ncMap[target] = nc
	ha.configs[target] = config
	return nil
}

func (ha *KvHandler) InitiateNatsWatchAll(sourceID string, target string, config *config.Config, secrets *secrets.Secrets) error {
	url := config.NatsURL
	nc, err := pkgnats.CreateNatsConnection(sourceID, secrets.NatsCredentials, url)
	if err != nil {
		ha.provider.Logger.Error("Failed to create (key-value-watcher) NATS connection", "sourceId", sourceID, "target", target, "error", err)
		return err
	}
	ha.ncMap[sourceID] = nc
	ha.configs[sourceID] = config
	return nil
}

func (ha *KvHandler) DeRegisterComponent(sourceID string) {
	ha.ncMap[sourceID].Close()
	delete(ha.ncMap, sourceID)
}

func (ha *KvHandler) DeRegisterComponentWatchAll(target string) {
	ha.ncMap[target].Close()
	delete(ha.ncMap, target)
}

func (ha *KvHandler) DeferAllNatsConnections() {
	for _, nc := range ha.ncMap {
		nc.Close()
	}
	clear(ha.ncMap)
}

func (ha *KvHandler) isLinkedWith(ctx context.Context) (bool, string) {
	header, ok := wrpcnats.HeaderFromContext(ctx)
	if !ok {
		ha.provider.Logger.Warn("Received request from unknown origin")
		return false, ""
	}
	target := header.Get("target")
	// Only allow requests from a linked component
	if ha.linkedFrom[target] == nil {
		ha.provider.Logger.Warn("Received request from unlinked target", "target", target)
		return false, ""
	}
	return true, target
}

// TODO:
// all of list-keys interface (get, purge, delete, etc, to be refactored since they share same logic)
func (ha *KvHandler) Get(ctx__ context.Context, key string) (*wrpc.Result[key_value.KeyValueEntry, string], error) {
	isLinked, target := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[key_value.KeyValueEntry]("Unauthorized"), nil
	}
	kv, err := ha.getKvByConfigAndNatsConnection(target)
	if err != nil {
		ha.provider.Logger.Error("error getting kv", "error", err)
		return nil, err
	}
	kve, kvGetErr := kv.Get(key)
	witResult := keyValErrToWit(kve, kvGetErr)
	return witResult, nil
}

func keyValErrToWit(a nats.KeyValueEntry, err error) *wrpc.Result[key_value.KeyValueEntry, string] {
	if err != nil {
		return wrpc.Err[key_value.KeyValueEntry](err.Error())
	}
	witKve := key_value.KeyValueEntry{
		Key:   a.Key(),
		Value: a.Value(),
	}
	return wrpc.Ok[string](witKve)
}

func (ha *KvHandler) Put(ctx__ context.Context, key string, value []uint8) (*wrpc.Result[struct{}, string], error) {
	isLinked, target := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[struct{}]("Unauthorized"), nil
	}
	kv, err := ha.getKvByConfigAndNatsConnection(target)
	if err != nil {
		ha.provider.Logger.Error("error getting kv", "error", err)
		return nil, err
	}
	_, kvPutErr := kv.Put(key, value)
	if kvPutErr != nil {
		return wrpc.Err[struct{}](kvPutErr.Error()), nil
	}
	return wrpc.Ok[string](struct{}{}), nil
}

func (ha *KvHandler) Purge(ctx__ context.Context, key string) (*wrpc.Result[struct{}, string], error) {
	isLinked, target := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[struct{}]("Unauthorized"), nil
	}
	kv, err := ha.getKvByConfigAndNatsConnection(target)
	if err != nil {
		ha.provider.Logger.Error("error getting kv", "error", err)
		return nil, err
	}
	kvPurgeErr := kv.Purge(key)
	if kvPurgeErr != nil {
		return wrpc.Err[struct{}](kvPurgeErr.Error()), nil
	}
	return wrpc.Ok[string](struct{}{}), nil
}

func (ha *KvHandler) Delete(ctx__ context.Context, key string) (*wrpc.Result[struct{}, string], error) {
	isLinked, target := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[struct{}]("Unauthorized"), nil
	}
	kv, err := ha.getKvByConfigAndNatsConnection(target)
	if err != nil {
		ha.provider.Logger.Error("error getting kv", "error", err)
		return nil, err
	}
	err = kv.Delete(key)
	if err != nil {
		ha.provider.Logger.Error("error deleting key", "key", key, "error", err)
		return wrpc.Err[struct{}](err.Error()), nil
	}
	return wrpc.Ok[string](struct{}{}), nil
}

func (ha *KvHandler) Create(ctx__ context.Context, key string, value []byte) (*wrpc.Result[struct{}, string], error) {
	isLinked, target := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[struct{}]("Unauthorized"), nil
	}
	kv, err := ha.getKvByConfigAndNatsConnection(target)
	if err != nil {
		ha.provider.Logger.Error("error getting kv", "error", err)
		return nil, err
	}
	_, kvCreateErr := kv.Create(key, value)
	if kvCreateErr != nil {
		return wrpc.Err[struct{}](kvCreateErr.Error()), nil
	}
	return wrpc.Ok[string](struct{}{}), nil
}

func (ha *KvHandler) ListKeys(ctx__ context.Context) (*wrpc.Result[[]string, string], error) {
	isLinked, target := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[[]string]("Unauthorized"), nil
	}
	ha.provider.Logger.Info("Get request", "target", target)
	kv, err := ha.getKvByConfigAndNatsConnection(target)
	if err != nil {
		ha.provider.Logger.Error("error getting kv", "error", err)
		return nil, err
	}
	keyChannel, err := kv.ListKeys()
	if err != nil {
		ha.provider.Logger.Error("error listing keys", "error", err)
		return wrpc.Err[[]string](err.Error()), err
	}
	keys := []string{}
	for key := range keyChannel.Keys() {
		keys = append(keys, key)
	}
	return wrpc.Ok[string](keys), nil
}

func (ha *KvHandler) RegisterComponentWatchAll(ctx__ context.Context, sourceId, target string) error {
	kv, err := ha.getKvByConfigAndNatsConnection(sourceId)
	config := ha.configs[sourceId]
	if err != nil {
		ha.provider.Logger.Warn("Failed to getkv", "error", err)
		return err
	}

	kvWatcherChannel, natsWatchAllErr := kv.WatchAll(nats.Context(ctx__))
	if natsWatchAllErr != nil {
		ha.provider.Logger.Error("Failed to watch all", "sourceId", sourceId, "error", natsWatchAllErr)
		return natsWatchAllErr
	}
	client := ha.provider.OutgoingRpcClient(sourceId)
	// INFO: A little delay for the provider to wait for the component to be ready
	time.Sleep(time.Duration(config.ComponentEstimatedStartupTime) * time.Second)
	go func() {
		for {
			select {
			case kvEntry := <-kvWatcherChannel.Updates():
				if kvEntry != nil {
					keyval := types.KeyValueEntry{}

					keyval.Key = kvEntry.Key()
					keyval.Value = kvEntry.Value()
					keyval.Op = kvEntry.Operation().String()
					ha.provider.Logger.Info("provider", "pre component, key found", string(keyval.Key))
					response, err := key_value_watcher.WatchAll(ctx__, client, &keyval)
					if err != nil {
						ha.provider.Logger.Error("Failed to watch all", "sourceId", sourceId, "error", err)
					}
					if response != nil {
						if response.Err != nil {
							ha.provider.Logger.Error("Failed to watch all", "sourceId", sourceId, "error", response.Err)
						}
					}
				}
			case <-ctx__.Done():
				ha.provider.Logger.Warn("Context done", "sourceId", sourceId)
				return
			}
		}
	}()
	return nil
}

func (ha *KvHandler) getKvByConfigAndNatsConnection(name string) (nats.KeyValue, error) {
	nc := ha.ncMap[name]
	js, err := nc.JetStream()
	if err != nil {
		ha.provider.Logger.Warn("Failed to create JetStream context", "sourceId/target", name, "error", err)
		return nil, err
	}
	config := ha.configs[name]
	kv, err := js.KeyValue(config.Bucket)
	return kv, nil
}
