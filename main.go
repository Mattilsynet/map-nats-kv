//go:generate wit-bindgen-wrpc go --out-dir bindings --package github.com/Mattilsynet/map-nats-kv/bindings wit

package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	server "github.com/Mattilsynet/map-nats-kv/bindings"
	"github.com/Mattilsynet/map-nats-kv/pkg/config"
	"github.com/Mattilsynet/map-nats-kv/pkg/secrets"
	"go.wasmcloud.dev/provider"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Initialize the provider with callbacks to track linked components
	providerHandler := NewKvHandler(
		make(map[string]map[string]string),
		make(map[string]map[string]string),
	)

	p, err := provider.New(
		provider.SourceLinkPut(func(link provider.InterfaceLinkDefinition) error {
			return handleNewSourceLink(providerHandler, link)
		}),
		provider.TargetLinkPut(func(link provider.InterfaceLinkDefinition) error {
			return handleNewTargetLink(providerHandler, link)
		}),
		provider.SourceLinkDel(func(link provider.InterfaceLinkDefinition) error {
			return handleDelSourceLink(providerHandler, link)
		}),
		provider.TargetLinkDel(func(link provider.InterfaceLinkDefinition) error {
			return handleDelTargetLink(providerHandler, link)
		}),
		provider.HealthCheck(func() string {
			return handleHealthCheck(providerHandler)
		}),
		provider.Shutdown(func() error {
			return handleShutdown(providerHandler)
		}),
	)
	if err != nil {
		return err
	}

	// Store the provider for use in the handlers
	providerHandler.provider = p

	// Setup two channels to await RPC and control interface operations
	providerCh := make(chan error, 1)
	signalCh := make(chan os.Signal, 1)

	// Handle RPC operations
	stopFunc, err := server.Serve(p.RPCClient, providerHandler)
	if err != nil {
		p.Shutdown()
		return err
	}

	// Handle control interface operations
	go func() {
		err := p.Start()
		providerCh <- err
	}()

	// Shutdown on SIGINT
	signal.Notify(signalCh, syscall.SIGINT)

	// Run provider until either a shutdown is requested or a SIGINT is received
	select {
	case err = <-providerCh:
		stopFunc()
		return err
	case <-signalCh:
		p.Shutdown()
		stopFunc()
	}

	return nil
}

// TODO: handle nats-kv-watcher-interface
func handleNewSourceLink(handler *KvHandler, link provider.InterfaceLinkDefinition) error {
	handler.provider.Logger.Info("Handling new source link", "link", link)
	handler.linkedTo[link.Target] = link.SourceConfig
	return nil
}

func handleNewTargetLink(handler *KvHandler, link provider.InterfaceLinkDefinition) error {
	handler.provider.Logger.Info("Handling new target link", "link", link)
	handler.linkedFrom[link.SourceID] = link.TargetConfig
	kvConfig := config.From(link.TargetConfig)
	secrets := secrets.From(link.TargetSecrets)
	handler.RegisterComponent(link.SourceID, link.Target, kvConfig, secrets)
	return nil
}

func handleDelSourceLink(handler *KvHandler, link provider.InterfaceLinkDefinition) error {
	handler.provider.Logger.Info("Handling del source link", "link", link)
	delete(handler.linkedTo, link.SourceID)
	return nil
}

func handleDelTargetLink(handler *KvHandler, link provider.InterfaceLinkDefinition) error {
	handler.provider.Logger.Info("Handling del target link", "link", link)
	handler.DeRegisterComponent(link.Target)
	delete(handler.linkedFrom, link.Target)
	return nil
}

func handleHealthCheck(_ *KvHandler) string {
	return "provider healthy"
}

func handleShutdown(handler *KvHandler) error {
	handler.provider.Logger.Info("Handling shutdown")
	handler.DeRegisterAll()
	clear(handler.linkedFrom)
	clear(handler.linkedTo)
	return nil
}
