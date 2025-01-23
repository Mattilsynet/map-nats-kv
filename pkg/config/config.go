package config

type Config struct {
	NatsURL        string
	Bucket         string
	ProviderConfig map[string]string
}

func From(config map[string]string) *Config {
	return &Config{
		NatsURL:        config["url"],
		Bucket:         config["bucket"],
		ProviderConfig: config,
	}
}
