# nettix-go

> **Work In Progress, not fully tested**  
> This project is currently being ported from the original Java implementation. APIs may change.

Go port of [nettix](https://github.com/osanha/nettix), a high-level network application framework originally built on Java Netty.

## Background

**nettix** is a high-level network application framework built on Java Netty. It abstracts away Netty's complexity, allowing developers to focus on protocol implementation and business logic.

**nettix-go** brings this philosophy to Go:

| Concept | Java (Netty/nettix) | Go (nettix-go) |
|---------|---------------------|----------------|
| Pipeline | ChannelPipeline | Pipeline |
| Context | ChannelHandlerContext | ChannelContext |
| Handler | ChannelHandler | LifecycleHandler, InboundHandler, ... |
| Frame Decoder | LengthFieldBasedFrameDecoder | LengthFieldDecoder |
| Async Result | ChannelFuture | Future, CallableFuture |
| Request Correlation | - | TimeoutableMap |
| Lifecycle | LifeCycle | AbstractStartable |

### Design Philosophy

1. **Pipeline Pattern**: Netty style Chain of Responsibility for message processing
2. **Named Handlers**: Dynamically add/remove/replace handlers at runtime
3. **Codec Chain**: Layered decoding with FrameDecoder → ProtocolDecoder → Handler
4. **Simplified API**: Reduce Netty boilerplate for rapid protocol implementation
5. **Protocol Agnostic**: Support for HTTP, WebSocket custom binary protocols, and more

### Use Cases

nettix/nettix-go is well-suited for implementing:

- **Binary Protocols**: Length-field based framing
- **Line-based Protocols**: Text protocols like SMTP, POP3
- **Custom Protocols**: Design and implement your own protocols

## Features

- **Pipeline Architecture**: Chain of Responsibility pattern for modular message processing
- **Named Handlers**: Dynamic pipeline manipulation with AddFirst/AddLast/AddBefore/AddAfter/Replace/Remove
- **Codec Chain**: Layered encoding/decoding (Frame → Protocol → Business Logic)
- **Lifecycle Management**: Automatic Start/Stop with state transitions
- **TLS Support**: Built-in TLS configuration and handshake handling
- **Connection Tracking**: Optional ChannelGroup for managing active connections
- **Async I/O**: Future-based async operations with listeners
- **Request-Response Correlation**: TimeoutableMap for matching requests to responses
 
## Related Projects

- [**nettix-go-mq**](https:?/github.com/osanha/nettix-go-mq): gRPC-based message queue using nettix-go utilities
- [**nettix-go-smpp**](https://github.com/osanha/nettix-go-smpp): SMPP 3.4 protocol implementation using nettix-go


## Installation

```bash
go get github.com/yourusername/nettix-go
```

## Quick Start

### Echo Server

```go
package main

import (
    "log/slog"
    "nettix-go/channel"
)

func main() {
    // Create server
    server := channel.NewServerChannelManager("Echo", 9000)
    server.UseChannelGroup(true)
    server.UsePipeline(true)

    // Configure pipeline
    server.BasePipeline().AddLast("FRAME_DECODER",
        channel.NewLengthFieldDecoder(65536, 0, 4, 0, 4))
    server.BasePipeline().AddLast("FRAME_ENCODER",
        channel.NewLengthFieldEncoder(4, false))
    server.BasePipeline().AddLast("ECHO_HANDLER", &EchoHandler{})

    // Start server
    if err := server.Start(); err != nil {
        slog.Error("Failed to start server", "error", err)
        return
    }
    defer server.Stop()

    // Wait for signal...
    select {}
}

type EchoHandler struct {
    channel.BaseChannelHandler
}

func (h *EchoHandler) OnConnected(ctx *channel.ChannelContext) error {
    slog.Info("Client connected", "remote", ctx.RemoteAddr())
    return nil
}

func (h *EchoHandler) OnMessage(ctx *channel.ChannelContext, msg interface{}) (interface{}, error) {
    ctx.Write(msg) // Echo back
    return nil, nil
}
```

### Echo Client

```go
package main

import (
    "log/slog"
    "nettix-go/channel"
)

func main() {
    client := channel.NewClientChannelManager("EchoClient")
    client.UsePipeline(true)

    client.BasePipeline().AddLast("FRAME_DECODER",
        channel.NewLengthFieldDecoder(65536, 0, 4, 0, 4))
    client.BasePipeline().AddLast("FRAME_ENCODER",
        channel.NewLengthFieldEncoder(4, false))
    client.BasePipeline().AddLast("CLIENT_HANDLER", &ClientHandler{})

    if err := client.Start(); err != nil {
        slog.Error("Failed to start client", "error", err)
        return
    }
    defer client.Stop()

    ctx, err := client.ConnectSync("localhost", 9000)
    if err != nil {
        slog.Error("Failed to connect", "error", err)
        return
    }

    // Send message
    ctx.WriteSync([]byte("Hello, World!"))
}
```

## Architecture

### Pipeline

The pipeline is the core of nettix-go, processing messages through a chain of handlers:

```
Inbound:  Socket → Decoder → Decoder → InboundHandler → InboundHandler → App
Outbound: App → OutboundHandler → Encoder → Encoder → Socket
```

```go
pipeline := channel.NewPipeline()

// Add handlers
pipeline.AddLast("FRAME_DECODER", channel.NewLengthFieldDecoder(...))
pipeline.AddLast("PROTOCOL_DECODER", &MyProtocolDecoder{})
pipeline.AddLast("BUSINESS_HANDLER", &MyHandler{})

// Dynamic manipulation
pipeline.AddBefore("BUSINESS_HANDLER", "LOGGING", &LoggingHandler{})
pipeline.Replace("PROTOCOL_DECODER", "NEW_DECODER", &NewDecoder{})
pipeline.Remove("LOGGING")
```

### Handler Interfaces

```go
// Lifecycle events
type LifecycleHandler interface {
    OnConnected(ctx *ChannelContext) error
    OnDisconnected(ctx *ChannelContext) error
}

// Inbound message processing
type InboundHandler interface {
    OnMessage(ctx *ChannelContext, msg interface{}) (interface{}, error)
}

// Outbound message processing
type OutboundHandler interface {
    OnWrite(ctx *ChannelContext, msg interface{}) (interface{}, error)
}

// Error handling
type ErrorHandler interface {
    OnError(ctx *ChannelContext, err error)
}
```

Use `BaseChannelHandler` for default implementations:

```go
type MyHandler struct {
    channel.BaseChannelHandler
}

func (h *MyHandler) OnMessage(ctx *channel.ChannelContext, msg interface{}) (interface{}, error) {
    // Handle message
    return msg, nil // Pass to next handler
    // return nil, nil // Consume message
}
```

### Codecs

#### Built-in Decoders

```go
// Length-prefixed frames (like Netty's LengthFieldBasedFrameDecoder)
channel.NewLengthFieldDecoder(maxFrameLength, lengthFieldOffset, lengthFieldLength, lengthAdjustment, initialBytesToStrip)

// Line-based text (\n or \r\n)
channel.NewLineDecoder(maxLength)

// Delimiter-based
channel.NewDelimiterDecoder([]byte{0x00}, maxLength)
```

#### Built-in Encoders

```go
// Length-prefixed encoding
channel.NewLengthFieldEncoder(lengthFieldLength, lengthIncludesHeader)

// Line encoding (appends \r\n)
channel.NewLineEncoder()
```

#### Custom Codec

```go
type MyDecoder struct{}

func (d *MyDecoder) Decode(ctx *channel.ChannelContext, buf *channel.Buffer) (interface{}, error) {
    if buf.ReadableBytes() < 4 {
        return nil, nil // Need more data
    }

    length, _ := buf.ReadUint32()
    if buf.ReadableBytes() < int(length) {
        buf.ResetReaderIndex(buf.MarkReaderIndex())
        return nil, nil // Need more data
    }

    data, _ := buf.ReadBytes(int(length))
    return &MyMessage{Data: data}, nil
}

type MyEncoder struct{}

func (e *MyEncoder) Encode(ctx *channel.ChannelContext, msg interface{}) ([]byte, error) {
    m := msg.(*MyMessage)
    buf := make([]byte, 4+len(m.Data))
    binary.BigEndian.PutUint32(buf, uint32(len(m.Data)))
    copy(buf[4:], m.Data)
    return buf, nil
}
```

### ChannelContext

Per-connection context providing:

```go
ctx.Conn()           // Underlying net.Conn
ctx.Pipeline()       // Connection's pipeline
ctx.RemoteAddr()     // Remote address
ctx.LocalAddr()      // Local address
ctx.Write(msg)       // Async write (returns Future)
ctx.WriteSync(msg)   // Sync write
ctx.Close()          // Close connection
ctx.SetAttr(k, v)    // Store connection-scoped data
ctx.GetAttr(k)       // Retrieve connection-scoped data
```

### Future

Async operation result:

```go
future := ctx.Write(msg)

// Option 1: Add listener
future.AddListener(func(f *channel.Future) {
    if f.IsSuccess() {
        slog.Info("Message sent")
    } else {
        slog.Error("Send failed", "error", f.Cause())
    }
})

// Option 2: Block and wait
future.Await()
if !future.IsSuccess() {
    return future.Cause()
}

// Option 3: Wait with timeout
if !future.AwaitTimeout(5 * time.Second) {
    return errors.New("timeout")
}
```

### CallableFuture

Future with return value for request-response patterns:

```go
future := channel.NewCallableFuture[*Response]()

// Store for correlation
requestMap.Put(sequenceNumber, future)

// In response handler
future.SetSuccessWithValue(response)

// Wait for response
response := future.Get()
// or with timeout
response, ok := future.GetTimeout(10 * time.Second)
```

## Utilities

### TimeoutableMap

Map with automatic timeout handling for request-response correlation:

```go
responseMap := util.NewTimeoutableMapWithTimeout[int32, *channel.CallableFuture[Response]](
    "responses",
    10 * time.Second,
)

// Store pending request
future := channel.NewCallableFuture[Response]()
responseMap.Put(sequenceNumber, future)

// On response received
if f := responseMap.Remove(sequenceNumber); f != nil {
    (*f).SetSuccessWithValue(response)
}

// On timeout, automatically removed and logged
```

### Lifecycle Management

All managers inherit from `AbstractStartable`:

```go
server := channel.NewServerChannelManager("MyServer", 9000)

server.Start()  // Transitions: READY → RUNNING
server.Stop()   // Transitions: RUNNING → TERMINATED
server.State()  // Returns current state
server.Name()   // Returns component name
```

## TLS Support

```go
import "nettix-go/ssl"

// Server with TLS
server.SetTLSFactory(ssl.NewServerTLSConfigFactory("cert.pem", "key.pem"))

// Client with TLS
client.SetTLSFactory(ssl.NewClientTLSConfigFactory(true)) // skipVerify

// Custom TLS config
client.SetTLSFactory(func() *tls.Config {
    return &tls.Config{
        ServerName: "example.com",
        MinVersion: tls.VersionTLS12,
    }
})
```

## Socket Options

```go
options := channel.DefaultServerOptions()
options.SocketOptions.KeepAlive = true
options.SocketOptions.NoDelay = true
options.SocketOptions.ReadBuffer = 64 * 1024
options.SocketOptions.WriteBuffer = 64 * 1024

server.SetOptions(options)
```

## Package Structure

```
nettix-go/
├── channel/
│   ├── pipeline.go      # Named handler pipeline
│   ├── context.go       # ChannelContext and handler interfaces
│   ├── codec.go         # Decoder/Encoder interfaces and implementations
│   ├── future.go        # Async operation futures
│   ├── server.go        # ServerChannelManager
│   ├── client.go        # ClientChannelManager
│   ├── manager.go       # AbstractChannelManager base
│   └── handler/         # Built-in handlers (ChannelGroup, Heartbeat)
├── util/
│   ├── startable.go     # Lifecycle management
│   ├── timeoutable.go   # TimeoutableMap
│   ├── scheduled.go     # Timer and scheduling
│   └── ...              # Other utilities
├── ssl/
│   ├── factory.go       # TLS configuration factories
│   ├── manager.go       # Certificate management
│   └── handshaker.go    # TLS handshake handling
├── log/
│   └── ...              # Logging utilities
└── example/
    └── echo_server.go   # Example implementation
```

## See Also

- [nettix](https://github.com/osanha/nettix) - Original Java implementation on Netty
- [Netty](https://netty.io/) - The underlying Java network framework

## License

MIT License
