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
		slog.Debug("pkgnats: Credentials file provided")
	} else {
		slog.Warn("pkgnats: No credentials file provided, if this is production then this setup might not work")
	}
	slog.Debug("pkgnats: Creating nats connection with url: " + natsUrl + " and client name: " + clientName)
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
	slog.Debug("pkgnats: reconnect wait option set to " + reconnectDelay.String())
	opts = append(opts, nats.MaxReconnects(int(totalWait/reconnectDelay)))
	slog.Debug("pkgnats: max reconnects option set to " + fmt.Sprintf("%v", int(totalWait/reconnectDelay)))
	opts = append(opts, nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
		slog.Debug("nats disconnected: Will kill application")
		panic("nats disconnected")
	}))
	opts = append(opts, nats.ReconnectHandler(func(nc *nats.Conn) {
		slog.Debug(fmt.Sprintf("Reconnected [%s]", nc.ConnectedUrl()))
	}))
	opts = append(opts, nats.ClosedHandler(func(nc *nats.Conn) {
		slog.Warn(fmt.Sprintf("Exiting: %v", nc.LastError()))
	}))
	return opts
}
