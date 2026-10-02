// Package nats provides the shared NATS connection used by the queue and cache JetStream drivers.
//
// # Architecture
//
// Connect wraps nats.go connection setup: it normalizes defaults (connect timeout, reconnect wait,
// drain timeout, reconnect limit), selects exactly one authentication method from Auth, optionally
// enables TLS, and installs logging handlers for disconnect, reconnect, and close events. On top of
// the raw connection it creates a JetStream context, returning both as a Client. The caller may pass
// additional nats.go options for anything the Config does not cover.
//
// One Client is intended to be shared by every NATS-based component of a transport (queue drivers,
// cache drivers, custom consumers): JetStream contexts are cheap, while each Client owns a single
// TCP connection with its own buffers and reconnect state.
//
// # Usage
//
//	client, err := nats.Connect(ctx, nats.Config{
//	    URLs: []string{"nats://localhost:4222"},
//	    Auth: nats.Auth{Username: "transport", Password: "secret"},
//	}, log)
//	if err != nil {
//	    return err
//	}
//	defer func() { _ = client.Drain(context.Background()) }()
//
//	// client.Conn is *nats.Conn, client.JetStream is a jetstream.JetStream context.
package nats
