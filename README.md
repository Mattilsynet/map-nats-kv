# map-nats-kv
A nats-kv provider for wasmcloud written in go, supports CRUD and listKeys

## Building

Prerequisites:

1. [Go toolchain](https://go.dev/doc/install)
1. [wit-bindgen-wrpc 0.7.0](https://github.com/bytecodealliance/wrpc), download the release binary
1. [wash](https://wasmcloud.com/docs/installation)

```bash
go generate ./...
go build .
```

Alternatively, you can generate, build and package this provider in one step:

```bash
wash build
```

You can build the included test component with `wash build -p ./component`.

## Running to test

Prerequisites:

1. [Go toolchain](https://go.dev/doc/install)
1. [nats-server](https://github.com/nats-io/nats-server)
1. [nats-cli](https://github.com/nats-io/natscli)

## Running as an application

You can deploy this provider, along with a [component](../component/) for testing, by deploying the [wadm.yaml](./wadm.yaml) application. Make sure to build the component with `wash build`.

```bash
# Build the component
cd component
wash build

# Return to the provider directory
cd ..

# Launch wasmCloud in the background
wash up -d
# Deploy the application
wash app deploy ./wadm.yaml
```

## TODO:

Add support for kv watch, wit is there but not implemented
