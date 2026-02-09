// Package example demonstrates advanced patterns in nettix-go.
// This file shows request-response messaging similar to nettix-mq's MessagePublisher.
package example

import (
	"encoding/binary"
	"log/slog"
	"time"

	"nettix-go/channel"
	"nettix-go/util"
)

// Message represents a request-response message.
// Similar to nettix-mq's Message class.
type Message struct {
	Seq   int32
	Type  int32
	Value []byte
}

// MessageType defines message types.
type MessageType int32

const (
	MessageTypePing MessageType = iota
	MessageTypePong
	MessageTypeData
	MessageTypeDataAck
)

// MessageEncoder encodes Message structs to bytes.
type MessageEncoder struct{}

func (e *MessageEncoder) Encode(ctx *channel.ChannelContext, msg interface{}) ([]byte, error) {
	m, ok := msg.(*Message)
	if !ok {
		return nil, channel.ErrNotEnoughData
	}

	// 4 bytes seq + 4 bytes type + N bytes value
	buf := make([]byte, 8+len(m.Value))
	binary.BigEndian.PutUint32(buf[0:4], uint32(m.Seq))
	binary.BigEndian.PutUint32(buf[4:8], uint32(m.Type))
	copy(buf[8:], m.Value)

	return buf, nil
}

// MessageDecoder decodes bytes to Message structs.
type MessageDecoder struct{}

func (d *MessageDecoder) Decode(ctx *channel.ChannelContext, buf *channel.Buffer) (interface{}, error) {
	if buf.ReadableBytes() < 8 {
		return nil, nil // Need more data
	}

	seq, err := buf.ReadUint32()
	if err != nil {
		return nil, nil
	}

	msgType, err := buf.ReadUint32()
	if err != nil {
		return nil, nil
	}

	value := make([]byte, buf.ReadableBytes())
	copy(value, buf.Bytes())
	buf.Skip(len(value))

	return &Message{
		Seq:   int32(seq),
		Type:  int32(msgType),
		Value: value,
	}, nil
}

// RequestResponseServer demonstrates request-response messaging.
// Similar pattern to MessagePublisher in nettix-mq.
type RequestResponseServer struct {
	*channel.ServerChannelManager

	logger     *slog.Logger
	sequencer  *util.RoundRobinInteger
	futureMap  *util.TimeoutableMap[int32, *channel.CallableFuture[*Message]]
	resTimeout time.Duration
}

// NewRequestResponseServer creates a new request-response server.
func NewRequestResponseServer(name string, port int, resTimeout time.Duration) *RequestResponseServer {
	server := &RequestResponseServer{
		ServerChannelManager: channel.NewServerChannelManager(name, port),
		logger:               slog.Default().With("component", "RequestResponseServer."+name),
		sequencer:            util.NewRoundRobinIntegerRange(1, 0x7FFFFFFF),
		resTimeout:           resTimeout,
	}

	// Create future map with timeout
	server.futureMap = util.NewTimeoutableMapWithTimeout[int32, *channel.CallableFuture[*Message]](
		name+" responses",
		resTimeout,
	)

	// Set timeout handler
	server.futureMap.SetTimeoutHandler(func(seq int32, future *channel.CallableFuture[*Message]) {
		future.SetFailure(channel.ErrConnectTimeout)
	})

	// Enable channel group
	server.UseChannelGroup(true)

	// Enable pipeline
	server.UsePipeline(true)

	// Setup pipeline
	server.BasePipeline().AddLast("FRAME_DECODER", channel.NewLengthFieldDecoder(65536, 0, 4, 0, 4))
	server.BasePipeline().AddLast("MSG_DECODER", &MessageDecoder{})
	server.BasePipeline().AddLast("FRAME_ENCODER", channel.NewLengthFieldEncoder(4, false))
	server.BasePipeline().AddLast("MSG_ENCODER", &MessageEncoder{})
	server.BasePipeline().AddLast("RESPONSE_HANDLER", &ResponseHandler{server: server})

	return server
}

// Request sends a request and waits for a response.
// Similar to MessagePublisher.request() in nettix-mq.
func (s *RequestResponseServer) Request(ctx *channel.ChannelContext, msgType MessageType, value []byte) *channel.CallableFuture[*Message] {
	seq := s.sequencer.Next()

	msg := &Message{
		Seq:   seq,
		Type:  int32(msgType),
		Value: value,
	}

	future := channel.NewCallableFutureWithConn[*Message](ctx.Conn())

	// Store in future map
	s.futureMap.Put(seq, future)

	// Send message
	sendFuture := ctx.Write(msg)
	sendFuture.AddListener(func(f *channel.Future) {
		if !f.IsSuccess() {
			s.futureMap.Remove(seq)
			future.SetFailure(f.Cause())
		}
	})

	return future
}

// Publish sends a message without waiting for a response (fire-and-forget).
func (s *RequestResponseServer) Publish(ctx *channel.ChannelContext, msgType MessageType, value []byte) *channel.Future {
	msg := &Message{
		Seq:   0, // No sequence for fire-and-forget
		Type:  int32(msgType),
		Value: value,
	}

	return ctx.Write(msg)
}

// ResponseHandler handles response messages for request-response pattern.
type ResponseHandler struct {
	channel.BaseChannelHandler
	server *RequestResponseServer
}

// OnMessage handles incoming messages.
func (h *ResponseHandler) OnMessage(ctx *channel.ChannelContext, msg interface{}) (interface{}, error) {
	m, ok := msg.(*Message)
	if !ok {
		return msg, nil // Pass through
	}

	// Check if this is a response to a pending request
	if m.Seq > 0 {
		if future := h.server.futureMap.Remove(m.Seq); future != nil {
			(*future).SetSuccessWithValue(m)
			return nil, nil // Message consumed
		}
	}

	// Otherwise, it's a new request - pass to next handler
	return msg, nil
}

// Example usage:
//
//	func main() {
//		server := NewRequestResponseServer("MyServer", 9000, 10*time.Second)
//		if err := server.Start(); err != nil {
//			log.Fatal(err)
//		}
//		defer server.Stop()
//
//		// Server is now running and handling connections via pipeline
//		// To send a request to a connected client:
//		// server.Connections().ForEach(func(conn net.Conn) bool {
//		//     ctx := server.NewChannelContext(conn)
//		//     future := server.Request(ctx, MessageTypeData, []byte("hello"))
//		//     response := future.Get()
//		//     ...
//		// })
//
//		select {}
//	}
