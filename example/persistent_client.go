// Package example demonstrates client patterns in nettix-go.
// This file shows a persistent client similar to nettix-smpp's SmppClient
// or nettix-mq's MessageSubscriber.
package example

import (
	"log/slog"
	"time"

	"nettix-go/channel"
	"nettix-go/util"
)

// MessageListener handles received messages.
type MessageListener interface {
	OnMessage(msg *Message)
}

// PersistentClient demonstrates a persistent client with auto-reconnection.
// Similar pattern to SmppClient in nettix-smpp or MessageSubscriber in nettix-mq.
type PersistentClient struct {
	*channel.PersistentClientChannelManager

	logger     *slog.Logger
	sequencer  *util.RoundRobinInteger
	futureMap  *util.TimeoutableMap[int32, *channel.CallableFuture[*Message]]
	resTimeout time.Duration
	listener   MessageListener
}

// NewPersistentClient creates a new persistent client.
func NewPersistentClient(name string, resTimeout time.Duration) *PersistentClient {
	client := &PersistentClient{
		PersistentClientChannelManager: channel.NewPersistentClientChannelManager(name),
		logger:                         slog.Default().With("component", "PersistentClient."+name),
		sequencer:                      util.NewRoundRobinIntegerRange(1, 0x7FFFFFFF),
		resTimeout:                     resTimeout,
	}

	// Create future map with timeout
	client.futureMap = util.NewTimeoutableMapWithTimeout[int32, *channel.CallableFuture[*Message]](
		name+" responses",
		resTimeout,
	)

	// Set timeout handler
	client.futureMap.SetTimeoutHandler(func(seq int32, future *channel.CallableFuture[*Message]) {
		future.SetFailure(channel.ErrConnectTimeout)
	})

	// Configure reconnection
	client.SetReconnDelay(1 * time.Second)
	client.SetConnTimeout(30 * time.Second)

	// Enable pipeline
	client.UsePipeline(true)

	// Setup pipeline - same as server
	client.BasePipeline().AddLast("FRAME_DECODER", channel.NewLengthFieldDecoder(65536, 0, 4, 0, 4))
	client.BasePipeline().AddLast("MSG_DECODER", &MessageDecoder{})
	client.BasePipeline().AddLast("FRAME_ENCODER", channel.NewLengthFieldEncoder(4, false))
	client.BasePipeline().AddLast("MSG_ENCODER", &MessageEncoder{})
	client.BasePipeline().AddLast("CLIENT_HANDLER", &ClientHandler{client: client})

	return client
}

// SetListener sets the message listener.
func (c *PersistentClient) SetListener(listener MessageListener) {
	c.listener = listener
}

// Connect connects to the server.
func (c *PersistentClient) ConnectToServer(host string, port int) *channel.Future {
	return c.Connect(host, port)
}

// Request sends a request and waits for a response.
func (c *PersistentClient) Request(ctx *channel.ChannelContext, msgType MessageType, value []byte) *channel.CallableFuture[*Message] {
	seq := c.sequencer.Next()

	msg := &Message{
		Seq:   seq,
		Type:  int32(msgType),
		Value: value,
	}

	future := channel.NewCallableFutureWithConn[*Message](ctx.Conn())

	// Store in future map
	c.futureMap.Put(seq, future)

	// Send message
	sendFuture := ctx.Write(msg)
	sendFuture.AddListener(func(f *channel.Future) {
		if !f.IsSuccess() {
			c.futureMap.Remove(seq)
			future.SetFailure(f.Cause())
		}
	})

	return future
}

// Send sends a message without waiting for a response.
func (c *PersistentClient) Send(ctx *channel.ChannelContext, msgType MessageType, value []byte) *channel.Future {
	msg := &Message{
		Seq:   0,
		Type:  int32(msgType),
		Value: value,
	}

	return ctx.Write(msg)
}

// ClientHandler handles messages on the client side.
type ClientHandler struct {
	channel.BaseChannelHandler
	client *PersistentClient
}

// OnConnected logs connection.
func (h *ClientHandler) OnConnected(ctx *channel.ChannelContext) error {
	h.client.logger.Info("Connected to server", "remote", ctx.RemoteAddr())
	return nil
}

// OnDisconnected logs disconnection.
func (h *ClientHandler) OnDisconnected(ctx *channel.ChannelContext) error {
	h.client.logger.Info("Disconnected from server", "remote", ctx.RemoteAddr())
	return nil
}

// OnMessage handles incoming messages.
func (h *ClientHandler) OnMessage(ctx *channel.ChannelContext, msg interface{}) (interface{}, error) {
	m, ok := msg.(*Message)
	if !ok {
		return msg, nil
	}

	// Check if this is a response
	if m.Seq > 0 {
		if future := h.client.futureMap.Remove(m.Seq); future != nil {
			(*future).SetSuccessWithValue(m)
			return nil, nil
		}
	}

	// Dispatch to listener
	if h.client.listener != nil {
		h.client.listener.OnMessage(m)
	}

	return nil, nil // Message consumed
}

// OnError handles errors.
func (h *ClientHandler) OnError(ctx *channel.ChannelContext, err error) {
	h.client.logger.Error("Error occurred", "error", err)
}

// Example usage:
//
//	func main() {
//		client := NewPersistentClient("MyClient", 10*time.Second)
//		client.SetListener(&MyListener{})
//
//		if err := client.Start(); err != nil {
//			log.Fatal(err)
//		}
//		defer client.Stop()
//
//		// Connect to server
//		future := client.ConnectToServer("localhost", 9000)
//		future.Await()
//
//		if !future.IsSuccess() {
//			log.Fatal(future.Cause())
//		}
//
//		// Connection established, get context for sending messages
//		ctx := client.NewChannelContext(future.Conn())
//
//		// Send request and wait for response
//		respFuture := client.Request(ctx, MessageTypeData, []byte("hello"))
//		response := respFuture.Get()
//		fmt.Printf("Response: %v\n", response)
//
//		select {}
//	}
//
//	type MyListener struct{}
//
//	func (l *MyListener) OnMessage(msg *Message) {
//		fmt.Printf("Received: %v\n", msg)
//	}
