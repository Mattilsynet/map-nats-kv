package secrets

import (
	"encoding/base64"
	"log/slog"

	"go.wasmcloud.dev/provider"
)

type Secrets struct {
	NatsCredentials string
}

func From(secretsMap map[string]provider.SecretValue) *Secrets {
	natsCredentialsEncrypted := secretsMap["nats-credentials"]
	natsCredentialsBase64Encoded := natsCredentialsEncrypted.String.Reveal()
	slog.Info("Nats credentials base64encoded, with size: ", "size", len(natsCredentialsBase64Encoded))
	natsCredentials, err := base64.StdEncoding.DecodeString(natsCredentialsBase64Encoded)
	if err != nil {
		return nil
	}
	slog.Info("Nats credentials loaded, with size: ", "size", len(natsCredentials))
	return &Secrets{
		NatsCredentials: string(natsCredentials),
	}
}
