package pkgnats

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/nats-io/nats.go"
)

func CreateNatsConnection(clientName, credentialsFileContent, natsUrl string) (*nats.Conn, error) {
	tmpCredsFile, err := os.CreateTemp("", "creds-*.json")
	if err != nil {
		return nil, err
	}
	_, err = io.WriteString(tmpCredsFile, credentialsFileContent)
	if err != nil {
		return nil, err
	}
	opts := []nats.Option{nats.Name(clientName)}
	opts = setupNatsConnectionOpts(opts)
	if len(tmpCredsFile.Name()) > 0 {
		opts = append(opts, nats.UserCredentials(tmpCredsFile.Name()))
		slog.Info("pkgnats: Credentials file provided")
	} else {
		slog.Info("pkgnats: No credentials file provided")
	}
	slog.Info("pkgnats: Creating nats connection with url: " + natsUrl + " and client name: " + clientName)
	nc, err := nats.Connect(natsUrl, opts...)
	if err != nil {
		return nil, err
	}
	tmpCredsFile.Close()
	os.Remove(tmpCredsFile.Name())
	return nc, nil
}

func setupNatsConnectionOpts(opts []nats.Option) []nats.Option {
	totalWait := 10 * time.Minute
	reconnectDelay := time.Second

	opts = append(opts, nats.ReconnectWait(reconnectDelay))
	slog.Info("pkgnats: reconnect wait option set to " + reconnectDelay.String())
	opts = append(opts, nats.MaxReconnects(int(totalWait/reconnectDelay)))
	slog.Info("pkgnats: max reconnects option set to " + fmt.Sprintf("%v", int(totalWait/reconnectDelay)))
	opts = append(opts, nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
		slog.Info(fmt.Sprintf("Disconnected: will attempt reconnects for %.0fm", totalWait.Minutes()))
	}))
	opts = append(opts, nats.ReconnectHandler(func(nc *nats.Conn) {
		slog.Info(fmt.Sprintf("Reconnected [%s]", nc.ConnectedUrl()))
	}))
	opts = append(opts, nats.ClosedHandler(func(nc *nats.Conn) {
		slog.Info(fmt.Sprintf("Exiting: %v", nc.LastError()))
	}))
	return opts
}
