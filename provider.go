package main

import (
	"context"

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
	kvMap      map[string]nats.KeyValue
}

func NewKvHandler(linkedFrom, linkedTo map[string]map[string]string) *KvHandler {
	return &KvHandler{
		linkedFrom: linkedFrom,
		linkedTo:   linkedTo,
		ncMap:      make(map[string]*nats.Conn),
		kvMap:      make(map[string]nats.KeyValue),
	}
}

func (ha *KvHandler) RegisterComponent(sourceID string, target string, config *config.Config, secrets *secrets.Secrets) error {
	url := config.NatsURL
	nc, err := pkgnats.CreateNatsConnection(target, secrets.NatsCredentials, url)
	if err != nil {
		ha.provider.Logger.Error("Failed to create NATS connection", "sourceId", sourceID, "target", target, "error", err)
		return err
	}
	js, err := nc.JetStream()
	if err != nil {
		ha.provider.Logger.Error("Failed to create jetstream context", "sourceId", sourceID, "target", target, "error", err)
		return err
	}
	ha.provider.Logger.Info("Connected to NATS", "sourceId", sourceID, "target", target)
	ha.provider.Logger.Info("Config", "config", config)
	kve, err := js.KeyValue(config.Bucket)
	if err != nil {
		ha.provider.Logger.Error("Failed to create KeyValue", "sourceId", sourceID, "target", target, "error", err)
		return err
	}
	ha.ncMap[sourceID] = nc
	ha.kvMap[sourceID] = kve
	return nil
}

func (ha *KvHandler) DeRegisterComponent(sourceID string) {
	ha.ncMap[sourceID].Close()
	delete(ha.ncMap, sourceID)
	delete(ha.kvMap, sourceID)
}

func (ha *KvHandler) DeferAllNatsConnections() {
	for _, nc := range ha.ncMap {
		nc.Close()
	}
	clear(ha.ncMap)
	clear(ha.kvMap)
}

func (ha *KvHandler) isLinkedWith(ctx context.Context) (bool, string) {
	header, ok := wrpcnats.HeaderFromContext(ctx)
	if !ok {
		ha.provider.Logger.Warn("Received request from unknown origin")
		return false, ""
	}
	sourceId := header.Get("source-id")
	// Only allow requests from a linked component
	if ha.linkedFrom[sourceId] == nil {
		ha.provider.Logger.Warn("Received request from unlinked source", "sourceId", sourceId)
		return false, ""
	}
	return true, sourceId
}

func (ha *KvHandler) Get(ctx__ context.Context, key string) (*wrpc.Result[key_value.KeyValueEntry, string], error) {
	isLinked, sourceId := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[key_value.KeyValueEntry]("Unauthorized"), nil
	}
	kve, kvGetErr := ha.kvMap[sourceId].Get(key)
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
	isLinked, sourceId := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[struct{}]("Unauthorized"), nil
	}
	_, kvPutErr := ha.kvMap[sourceId].Put(key, value)
	if kvPutErr != nil {
		return wrpc.Err[struct{}](kvPutErr.Error()), nil
	}
	return wrpc.Ok[string](struct{}{}), nil
}

func (ha *KvHandler) Purge(ctx__ context.Context, key string) (*wrpc.Result[struct{}, string], error) {
	isLinked, sourceId := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[struct{}]("Unauthorized"), nil
	}
	kvPurgeErr := ha.kvMap[sourceId].Purge(key)
	if kvPurgeErr != nil {
		return wrpc.Err[struct{}](kvPurgeErr.Error()), nil
	}
	return wrpc.Ok[string](struct{}{}), nil
}

func (ha *KvHandler) Delete(ctx__ context.Context, key string) (*wrpc.Result[struct{}, string], error) {
	isLinked, sourceId := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[struct{}]("Unauthorized"), nil
	}
	kvDeleteErr := ha.kvMap[sourceId].Delete(key)
	if kvDeleteErr != nil {
		return wrpc.Err[struct{}](kvDeleteErr.Error()), nil
	}
	return wrpc.Ok[string](struct{}{}), nil
}

func (ha *KvHandler) Create(ctx__ context.Context, key string, value []byte) (*wrpc.Result[struct{}, string], error) {
	isLinked, sourceId := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[struct{}]("Unauthorized"), nil
	}
	_, kvCreateErr := ha.kvMap[sourceId].Create(key, value)
	if kvCreateErr != nil {
		return wrpc.Err[struct{}](kvCreateErr.Error()), nil
	}
	return wrpc.Ok[string](struct{}{}), nil
}

func (ha *KvHandler) ListKeys(ctx__ context.Context) (*wrpc.Result[[]string, string], error) {
	isLinked, sourceId := ha.isLinkedWith(ctx__)
	if !isLinked {
		return wrpc.Err[[]string]("Unauthorized"), nil
	}
	keyChannel, err := ha.kvMap[sourceId].ListKeys()
	if err != nil {
		return wrpc.Err[[]string](err.Error()), err
	}
	keys := []string{}
	for key := range keyChannel.Keys() {
		keys = append(keys, key)
	}
	return wrpc.Ok[string](keys), nil
}

func (ha *KvHandler) RegisterComponentWatchAll(ctx__ context.Context, sourceId string) error {
	kvWatcherChannel, natsWatchAllErr := ha.kvMap[sourceId].WatchAll()
	if natsWatchAllErr != nil {
		ha.provider.Logger.Error("Failed to watch all", "sourceId", sourceId, "error", natsWatchAllErr)
		return natsWatchAllErr
	}
	client := ha.provider.OutgoingRpcClient(sourceId)
	go func() {
		for {
			select {
			case kvEntry := <-kvWatcherChannel.Updates():
				if kvEntry != nil {
					keyval := types.KeyValueEntry{}
					keyval.Key = kvEntry.Key()
					keyval.Value = kvEntry.Value()
					err := key_value_watcher.WatchAll(ctx__, client, &keyval)
					if err != nil {
						ha.provider.Logger.Error("Failed to send update to component", "sourceId", sourceId, "error", err)
					}
				}
			case <-ctx__.Done():
				return
			}
		}
	}()
	return nil
}
