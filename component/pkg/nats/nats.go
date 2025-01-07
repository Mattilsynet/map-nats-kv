package nats

import (
	"errors"
	"log/slog"

	"github.com/Mattilsynet/map-providers/map-nats-kv/component/gen/wasmcloud/messaging/consumer"
	"github.com/Mattilsynet/map-providers/map-nats-kv/component/gen/wasmcloud/messaging/handler"
	"github.com/Mattilsynet/map-providers/map-nats-kv/component/gen/wasmcloud/messaging/types"
	"github.com/bytecodealliance/wasm-tools-go/cm"
)

type (
	Conn struct {
		js JetStreamContext
	}
	JetStreamContext struct{}
	Msg              struct {
		Subject string
		Reply   string
		Data    []byte
		Header  map[string][]string
	}
)

type MsgHandler func(msg *Msg)

func NewConn() *Conn {
	return &Conn{}
}

func FromBrokerMessageToNatsMessage(bm types.BrokerMessage) *Msg {
	if bm.ReplyTo.None() {
		return &Msg{
			Data:    bm.Body.Slice(),
			Subject: bm.Subject,
			Reply:   "",
		}
	} else {
		return &Msg{
			Data:    bm.Body.Slice(),
			Subject: bm.Subject,
			Reply:   *bm.ReplyTo.Some(),
		}
	}
}

func ToBrokenMessageFromNatsMessage(nm *Msg) types.BrokerMessage {
	if nm.Reply == "" {
		return types.BrokerMessage{
			Subject: nm.Subject,
			Body:    cm.ToList(nm.Data),
			ReplyTo: cm.None[string](),
		}
	} else {
		return types.BrokerMessage{
			Subject: nm.Subject,
			Body:    cm.ToList(nm.Data),
			ReplyTo: cm.Some(nm.Subject),
		}
	}
}

func (conn *Conn) RequestReply(msg *Msg, timeoutInMillis uint32) (*Msg, error) {
	bm := ToBrokenMessageFromNatsMessage(msg)
	result := consumer.Request(bm.Subject, bm.Body, timeoutInMillis)
	if result.IsOK() {
		bmReceived := result.OK()
		natsMsgReceived := FromBrokerMessageToNatsMessage(*bmReceived)
		return natsMsgReceived, nil
	} else {
		return nil, errors.New(*result.Err())
	}
}

func (conn *Conn) RegisterRequestReply(fn func(*Msg) *Msg, logger *slog.Logger) {
	handler.Exports.HandleMessage = func(msg types.BrokerMessage) (result cm.Result[string, struct{}, string]) {
		logger.Info("hey hey hey")
		natsMsg := FromBrokerMessageToNatsMessage(msg)
		logger.Info("Received message on subject %s", natsMsg.Subject, " with reply %s", natsMsg.Reply, " and data %s", natsMsg.Data)
		newMsg := fn(natsMsg)
		return consumer.Publish(ToBrokenMessageFromNatsMessage(newMsg))
	}
}
