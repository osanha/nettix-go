package channel

// HeartbeatFactory generates messages to be sent at regular intervals.
type HeartbeatFactory[T any] interface {
	// CreateHeartbeat creates a message to be sent.
	CreateHeartbeat() T
}

// EnquireLinkFactory extends HeartbeatFactory with the ability to track
// futures for sent messages.
type EnquireLinkFactory[T any] interface {
	HeartbeatFactory[T]

	// PutFuture stores the result future for a sent message.
	PutFuture(heartbeat T, future *Future)
}
