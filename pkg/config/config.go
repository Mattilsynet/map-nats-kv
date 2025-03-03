package config

import "strconv"

type Config struct {
	NatsURL string
	Bucket  string
	// INFO: we need to wait a little for the component to startup such that we don't aggregate the kv watchall data to the component before it's deployment time, if we do this we're in a stall and nothing happens during watchall
	ComponentEstimatedStartupTime int
	ProviderConfig                map[string]string
}

func From(config map[string]string) *Config {
	componentEstimatedStartupTime, err := strconv.Atoi(config["startup_time"])
	if err != nil {
		componentEstimatedStartupTime = 30
	}
	return &Config{
		NatsURL:                       config["url"],
		Bucket:                        config["bucket"],
		ComponentEstimatedStartupTime: componentEstimatedStartupTime,
		ProviderConfig:                config,
	}
}
