//go:generate go run github.com/bytecodealliance/wasm-tools-go/cmd/wit-bindgen-go generate --world component --out gen ./wit
package main

import (
	"log/slog"
	"strings"

	keyvalue "github.com/Mattilsynet/map-nats-kv/component/gen/mattilsynet/map-kv/key-value"
	keyvaluewatcher "github.com/Mattilsynet/map-nats-kv/component/gen/mattilsynet/map-kv/key-value-watcher"
	"github.com/Mattilsynet/map-nats-kv/component/gen/wasmcloud/messaging/consumer"
	"github.com/Mattilsynet/map-nats-kv/component/gen/wasmcloud/messaging/handler"
	"github.com/Mattilsynet/map-nats-kv/component/gen/wasmcloud/messaging/types"
	"github.com/Mattilsynet/map-nats-kv/component/pkg/nats"
	"github.com/bytecodealliance/wasm-tools-go/cm"
	"go.wasmcloud.dev/component/log/wasilog"
)

var (
	conn   *nats.Conn
	logger *slog.Logger
)

func init() {
	handler.Exports.HandleMessage = msgHandlerv2
	keyvaluewatcher.Exports.WatchAll = watchAllHandler
}

func watchAllHandler(kv keyvalue.KeyValueEntry) cm.Result[string, struct{}, string] {
	logger = wasilog.ContextLogger("NATS-KV-Component-watch-all")
	logger.Info("Got", "key", kv.Key, "value", string(kv.Value.Slice()))
	return cm.OK[cm.Result[string, struct{}, string]](struct{}{})
}

func msgHandlerv2(msg types.BrokerMessage) (result cm.Result[string, struct{}, string]) {
	logger = wasilog.ContextLogger("NATS-KV-Component-request-reply")
	replyMsg := types.BrokerMessage{
		Subject: *msg.ReplyTo.Some(),
		Body:    cm.ToList([]byte("hey back")),
	}
	crud := msg.Body.Slice()
	crudAsString := string(crud)
	switch crudAsString {
	case "create":
		res := keyvalue.Create("stuff", cm.ToList([]byte("hello first world")))
		if res.IsErr() {
			logger.Error("Error creating key", "error", res.Err())
		}
	case "put":
		res := keyvalue.Put("stuff", cm.ToList([]byte("hello other world")))
		if res.IsErr() {
			logger.Error("Error putting key", "error", res.Err())
		}
	case "delete":
		res := keyvalue.Delete("stuff")
		if res.IsErr() {
			logger.Error("Error deleting key", "error", res.Err())
		}
	case "get":
		res := keyvalue.Get("stuff")
		if res.IsErr() {
			logger.Error("Error getting key", "error", res.Err())
		}
	case "list":
		listOfKeys := keyvalue.ListKeys()
		logger.Info("List of keys", "keys", strings.Join(listOfKeys.OK().Slice(), ", "))
		if listOfKeys.IsErr() {
			logger.Error("Error listing keys", "error", listOfKeys.Err())
		}
	default:
		if msg.ReplyTo.None() {
			logger.Info("no reply to, we just got a publish and not a request")
			return cm.Err[cm.Result[string, struct{}, string]]("pub/sub not allowed")
		}
		logger.Info("unknown command", "command", crudAsString)
	}
	return consumer.Publish(replyMsg)
}

func MsgHandler(msg *nats.Msg) *nats.Msg {
	replyMsg := &nats.Msg{
		Subject: msg.Reply,
		Data:    []byte("hey back"),
	}
	return replyMsg
}

func main() {}
