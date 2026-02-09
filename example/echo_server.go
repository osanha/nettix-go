// Package example demonstrates how to use nettix-go to build servers and clients.
// This example shows how to create an echo server similar to nettix-smpp's SmppServer
// using the pipeline-based approach.
package example

import (
	"log/slog"

	"nettix-go/channel"
)

// EchoServer demonstrates a simple echo server using nettix-go.
// Similar pattern to SmppServer in nettix-smpp:
//
//	server := NewEchoServer("Echo", 9000)
//	server.BasePipeline().AddLast("FRAME_DECODER", channel.NewLengthFieldDecoder(1024, 0, 4, 0, 4))
//	server.BasePipeline().AddLast("FRAME_ENCODER", channel.NewLengthFieldEncoder(4, false))
//	server.BasePipeline().AddLast("ECHO_HANDLER", &EchoHandler{})
//	server.Start()
type EchoServer struct {
	*channel.ServerChannelManager
	logger *slog.Logger
}

// NewEchoServer creates a new echo server.
func NewEchoServer(name string, port int) *EchoServer {
	server := &EchoServer{
		ServerChannelManager: channel.NewServerChannelManager(name, port),
		logger:               slog.Default().With("component", "EchoServer."+name),
	}

	// Enable channel group for connection tracking
	server.UseChannelGroup(true)

	// Enable built-in pipeline-based read loop
	server.UsePipeline(true)

	// Add default pipeline handlers
	// Users can add more handlers using server.BasePipeline().AddLast(...)
	server.BasePipeline().AddLast("FRAME_DECODER", channel.NewLengthFieldDecoder(65536, 0, 4, 0, 4))
	server.BasePipeline().AddLast("FRAME_ENCODER", channel.NewLengthFieldEncoder(4, false))
	server.BasePipeline().AddLast("ECHO_HANDLER", &EchoHandler{logger: server.logger})

	return server
}

// EchoHandler echoes received messages back to the client.
type EchoHandler struct {
	channel.BaseChannelHandler
	logger *slog.Logger
}

// OnConnected logs when a client connects.
func (h *EchoHandler) OnConnected(ctx *channel.ChannelContext) error {
	h.logger.Info("Client connected", "remote", ctx.RemoteAddr())
	return nil
}

// OnDisconnected logs when a client disconnects.
func (h *EchoHandler) OnDisconnected(ctx *channel.ChannelContext) error {
	h.logger.Info("Client disconnected", "remote", ctx.RemoteAddr())
	return nil
}

// OnMessage echoes the message back to the client.
func (h *EchoHandler) OnMessage(ctx *channel.ChannelContext, msg interface{}) (interface{}, error) {
	h.logger.Debug("Received message", "remote", ctx.RemoteAddr(), "msg", msg)

	// Echo the message back
	future := ctx.Write(msg)
	future.AddListener(func(f *channel.Future) {
		if !f.IsSuccess() {
			h.logger.Error("Failed to send echo", "error", f.Cause())
		}
	})

	return nil, nil // Message consumed
}

// OnError handles errors.
func (h *EchoHandler) OnError(ctx *channel.ChannelContext, err error) {
	h.logger.Error("Error occurred", "remote", ctx.RemoteAddr(), "error", err)
}
