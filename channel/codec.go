package channel

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	// ErrNotEnoughData indicates more data is needed to decode a complete message.
	ErrNotEnoughData = errors.New("not enough data")
	// ErrMessageTooLarge indicates the message exceeds the maximum allowed size.
	ErrMessageTooLarge = errors.New("message too large")
)

// Decoder transforms bytes into messages.
// Multiple decoders can be chained: FrameDecoder → ProtocolDecoder.
type Decoder interface {
	// Decode reads from the buffer and returns a decoded message.
	// Returns (nil, nil) if more data is needed.
	// Returns (nil, error) on decode error.
	// Returns (msg, nil) when a complete message is decoded.
	Decode(ctx *ChannelContext, buf *Buffer) (interface{}, error)
}

// Encoder transforms messages into bytes.
// Multiple encoders can be chained: ProtocolEncoder → FrameEncoder.
type Encoder interface {
	// Encode writes the message to the buffer.
	// Returns the encoded bytes, or error on encode failure.
	Encode(ctx *ChannelContext, msg interface{}) ([]byte, error)
}

// Buffer is a simple byte buffer for decoding.
type Buffer struct {
	data      []byte
	readIndex int
}

// NewBuffer creates a new buffer with the given data.
func NewBuffer(data []byte) *Buffer {
	return &Buffer{data: data}
}

// NewEmptyBuffer creates an empty buffer.
func NewEmptyBuffer() *Buffer {
	return &Buffer{data: make([]byte, 0)}
}

// Write appends data to the buffer.
func (b *Buffer) Write(data []byte) {
	b.data = append(b.data, data...)
}

// ReadableBytes returns the number of bytes available for reading.
func (b *Buffer) ReadableBytes() int {
	return len(b.data) - b.readIndex
}

// ReadBytes reads n bytes from the buffer.
func (b *Buffer) ReadBytes(n int) ([]byte, error) {
	if b.ReadableBytes() < n {
		return nil, ErrNotEnoughData
	}
	result := make([]byte, n)
	copy(result, b.data[b.readIndex:b.readIndex+n])
	b.readIndex += n
	return result, nil
}

// PeekBytes peeks n bytes without advancing the read index.
func (b *Buffer) PeekBytes(n int) ([]byte, error) {
	if b.ReadableBytes() < n {
		return nil, ErrNotEnoughData
	}
	result := make([]byte, n)
	copy(result, b.data[b.readIndex:b.readIndex+n])
	return result, nil
}

// ReadByte reads a single byte from the buffer.
func (b *Buffer) ReadByte() (byte, error) {
	if b.ReadableBytes() < 1 {
		return 0, ErrNotEnoughData
	}
	result := b.data[b.readIndex]
	b.readIndex++
	return result, nil
}

// ReadUint16 reads a big-endian uint16.
func (b *Buffer) ReadUint16() (uint16, error) {
	if b.ReadableBytes() < 2 {
		return 0, ErrNotEnoughData
	}
	result := binary.BigEndian.Uint16(b.data[b.readIndex:])
	b.readIndex += 2
	return result, nil
}

// ReadUint32 reads a big-endian uint32.
func (b *Buffer) ReadUint32() (uint32, error) {
	if b.ReadableBytes() < 4 {
		return 0, ErrNotEnoughData
	}
	result := binary.BigEndian.Uint32(b.data[b.readIndex:])
	b.readIndex += 4
	return result, nil
}

// ReadUint64 reads a big-endian uint64.
func (b *Buffer) ReadUint64() (uint64, error) {
	if b.ReadableBytes() < 8 {
		return 0, ErrNotEnoughData
	}
	result := binary.BigEndian.Uint64(b.data[b.readIndex:])
	b.readIndex += 8
	return result, nil
}

// Skip advances the read index by n bytes.
func (b *Buffer) Skip(n int) error {
	if b.ReadableBytes() < n {
		return ErrNotEnoughData
	}
	b.readIndex += n
	return nil
}

// Reset resets the read index to 0.
func (b *Buffer) Reset() {
	b.readIndex = 0
}

// Compact discards already-read bytes and resets the read index.
func (b *Buffer) Compact() {
	if b.readIndex > 0 {
		remaining := b.data[b.readIndex:]
		b.data = make([]byte, len(remaining))
		copy(b.data, remaining)
		b.readIndex = 0
	}
}

// Data returns the underlying byte slice.
func (b *Buffer) Data() []byte {
	return b.data
}

// Bytes returns unread bytes.
func (b *Buffer) Bytes() []byte {
	return b.data[b.readIndex:]
}

// MarkReaderIndex saves the current read index.
func (b *Buffer) MarkReaderIndex() int {
	return b.readIndex
}

// ResetReaderIndex restores the read index to a previously marked position.
func (b *Buffer) ResetReaderIndex(mark int) {
	b.readIndex = mark
}

// LengthFieldDecoder decodes frames based on a length field.
// Similar to Netty's LengthFieldBasedFrameDecoder.
type LengthFieldDecoder struct {
	maxFrameLength    int
	lengthFieldOffset int
	lengthFieldLength int
	lengthAdjustment  int
	initialBytesToStrip int
}

// NewLengthFieldDecoder creates a new length field decoder.
// Parameters:
//   - maxFrameLength: maximum allowed frame size
//   - lengthFieldOffset: offset of the length field from the beginning
//   - lengthFieldLength: number of bytes in the length field (1, 2, 4, or 8)
//   - lengthAdjustment: added to the length field value to get actual data length
//   - initialBytesToStrip: bytes to strip from the decoded frame (usually header length)
func NewLengthFieldDecoder(maxFrameLength, lengthFieldOffset, lengthFieldLength, lengthAdjustment, initialBytesToStrip int) *LengthFieldDecoder {
	return &LengthFieldDecoder{
		maxFrameLength:      maxFrameLength,
		lengthFieldOffset:   lengthFieldOffset,
		lengthFieldLength:   lengthFieldLength,
		lengthAdjustment:    lengthAdjustment,
		initialBytesToStrip: initialBytesToStrip,
	}
}

// Decode implements Decoder.
func (d *LengthFieldDecoder) Decode(ctx *ChannelContext, buf *Buffer) (interface{}, error) {
	headerLen := d.lengthFieldOffset + d.lengthFieldLength
	if buf.ReadableBytes() < headerLen {
		return nil, nil // Need more data
	}

	// Mark position in case we need to reset
	mark := buf.MarkReaderIndex()

	// Skip to length field
	if d.lengthFieldOffset > 0 {
		if err := buf.Skip(d.lengthFieldOffset); err != nil {
			buf.ResetReaderIndex(mark)
			return nil, nil
		}
	}

	// Read length field
	var frameLength int
	switch d.lengthFieldLength {
	case 1:
		b, err := buf.ReadByte()
		if err != nil {
			buf.ResetReaderIndex(mark)
			return nil, nil
		}
		frameLength = int(b)
	case 2:
		v, err := buf.ReadUint16()
		if err != nil {
			buf.ResetReaderIndex(mark)
			return nil, nil
		}
		frameLength = int(v)
	case 4:
		v, err := buf.ReadUint32()
		if err != nil {
			buf.ResetReaderIndex(mark)
			return nil, nil
		}
		frameLength = int(v)
	case 8:
		v, err := buf.ReadUint64()
		if err != nil {
			buf.ResetReaderIndex(mark)
			return nil, nil
		}
		frameLength = int(v)
	default:
		return nil, errors.New("invalid length field length")
	}

	frameLength += d.lengthAdjustment

	if frameLength > d.maxFrameLength {
		return nil, ErrMessageTooLarge
	}

	// Reset to beginning
	buf.ResetReaderIndex(mark)

	// Check if we have enough data
	totalLength := d.lengthFieldOffset + d.lengthFieldLength + frameLength
	if buf.ReadableBytes() < totalLength {
		return nil, nil // Need more data
	}

	// Strip initial bytes
	if d.initialBytesToStrip > 0 {
		if err := buf.Skip(d.initialBytesToStrip); err != nil {
			buf.ResetReaderIndex(mark)
			return nil, nil
		}
	}

	// Read the frame
	frameLen := totalLength - d.initialBytesToStrip
	frame, err := buf.ReadBytes(frameLen)
	if err != nil {
		buf.ResetReaderIndex(mark)
		return nil, nil
	}

	return frame, nil
}

// LengthFieldEncoder prepends a length field to outgoing messages.
// Similar to Netty's LengthFieldPrepender.
type LengthFieldEncoder struct {
	lengthFieldLength   int
	lengthIncludesHeader bool
}

// NewLengthFieldEncoder creates a new length field encoder.
// Parameters:
//   - lengthFieldLength: number of bytes for the length field (1, 2, 4, or 8)
//   - lengthIncludesHeader: if true, length includes the header itself
func NewLengthFieldEncoder(lengthFieldLength int, lengthIncludesHeader bool) *LengthFieldEncoder {
	return &LengthFieldEncoder{
		lengthFieldLength:    lengthFieldLength,
		lengthIncludesHeader: lengthIncludesHeader,
	}
}

// Encode implements Encoder.
func (e *LengthFieldEncoder) Encode(ctx *ChannelContext, msg interface{}) ([]byte, error) {
	data, ok := msg.([]byte)
	if !ok {
		return nil, errors.New("expected []byte message")
	}

	length := len(data)
	if e.lengthIncludesHeader {
		length += e.lengthFieldLength
	}

	result := make([]byte, e.lengthFieldLength+len(data))

	switch e.lengthFieldLength {
	case 1:
		result[0] = byte(length)
	case 2:
		binary.BigEndian.PutUint16(result, uint16(length))
	case 4:
		binary.BigEndian.PutUint32(result, uint32(length))
	case 8:
		binary.BigEndian.PutUint64(result, uint64(length))
	default:
		return nil, errors.New("invalid length field length")
	}

	copy(result[e.lengthFieldLength:], data)
	return result, nil
}

// LineDecoder decodes line-based text (terminated by \n or \r\n).
type LineDecoder struct {
	maxLength int
}

// NewLineDecoder creates a new line decoder.
func NewLineDecoder(maxLength int) *LineDecoder {
	return &LineDecoder{maxLength: maxLength}
}

// Decode implements Decoder.
func (d *LineDecoder) Decode(ctx *ChannelContext, buf *Buffer) (interface{}, error) {
	data := buf.Bytes()
	for i := 0; i < len(data); i++ {
		if i > d.maxLength {
			return nil, ErrMessageTooLarge
		}
		if data[i] == '\n' {
			line := make([]byte, i)
			copy(line, data[:i])
			// Handle \r\n
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			buf.Skip(i + 1)
			return string(line), nil
		}
	}
	return nil, nil // Need more data
}

// LineEncoder encodes strings to line-based format (appending \r\n).
type LineEncoder struct{}

// NewLineEncoder creates a new line encoder.
func NewLineEncoder() *LineEncoder {
	return &LineEncoder{}
}

// Encode implements Encoder.
func (e *LineEncoder) Encode(ctx *ChannelContext, msg interface{}) ([]byte, error) {
	var data []byte
	switch v := msg.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		return nil, errors.New("expected string or []byte message")
	}
	result := make([]byte, len(data)+2)
	copy(result, data)
	result[len(data)] = '\r'
	result[len(data)+1] = '\n'
	return result, nil
}

// DelimiterDecoder decodes messages delimited by a specific byte sequence.
type DelimiterDecoder struct {
	delimiter []byte
	maxLength int
}

// NewDelimiterDecoder creates a new delimiter decoder.
func NewDelimiterDecoder(delimiter []byte, maxLength int) *DelimiterDecoder {
	return &DelimiterDecoder{
		delimiter: delimiter,
		maxLength: maxLength,
	}
}

// Decode implements Decoder.
func (d *DelimiterDecoder) Decode(ctx *ChannelContext, buf *Buffer) (interface{}, error) {
	data := buf.Bytes()
	delimLen := len(d.delimiter)

	for i := 0; i <= len(data)-delimLen; i++ {
		if i > d.maxLength {
			return nil, ErrMessageTooLarge
		}
		match := true
		for j := 0; j < delimLen; j++ {
			if data[i+j] != d.delimiter[j] {
				match = false
				break
			}
		}
		if match {
			msg := make([]byte, i)
			copy(msg, data[:i])
			buf.Skip(i + delimLen)
			return msg, nil
		}
	}
	return nil, nil // Need more data
}

// ReplayingDecoder is a base for decoders that may need to "replay" decoding.
// It wraps the decode process and handles ErrNotEnoughData transparently.
type ReplayingDecoder struct {
	state    int
	checkpoint int
}

// NewReplayingDecoder creates a new replaying decoder.
func NewReplayingDecoder() *ReplayingDecoder {
	return &ReplayingDecoder{}
}

// State returns the current decoder state.
func (d *ReplayingDecoder) State() int {
	return d.state
}

// SetState sets the decoder state.
func (d *ReplayingDecoder) SetState(state int) {
	d.state = state
}

// Checkpoint sets a checkpoint for replaying.
func (d *ReplayingDecoder) Checkpoint(buf *Buffer) {
	d.checkpoint = buf.MarkReaderIndex()
}

// Replay resets to the checkpoint.
func (d *ReplayingDecoder) Replay(buf *Buffer) {
	buf.ResetReaderIndex(d.checkpoint)
}

// ByteToMessageDecoder wraps a decode function for simpler decoder implementation.
type ByteToMessageDecoder struct {
	decodeFn func(ctx *ChannelContext, buf *Buffer) (interface{}, error)
}

// NewByteToMessageDecoder creates a decoder from a function.
func NewByteToMessageDecoder(fn func(ctx *ChannelContext, buf *Buffer) (interface{}, error)) *ByteToMessageDecoder {
	return &ByteToMessageDecoder{decodeFn: fn}
}

// Decode implements Decoder.
func (d *ByteToMessageDecoder) Decode(ctx *ChannelContext, buf *Buffer) (interface{}, error) {
	return d.decodeFn(ctx, buf)
}

// MessageToByteEncoder wraps an encode function for simpler encoder implementation.
type MessageToByteEncoder struct {
	encodeFn func(ctx *ChannelContext, msg interface{}) ([]byte, error)
}

// NewMessageToByteEncoder creates an encoder from a function.
func NewMessageToByteEncoder(fn func(ctx *ChannelContext, msg interface{}) ([]byte, error)) *MessageToByteEncoder {
	return &MessageToByteEncoder{encodeFn: fn}
}

// Encode implements Encoder.
func (e *MessageToByteEncoder) Encode(ctx *ChannelContext, msg interface{}) ([]byte, error) {
	return e.encodeFn(ctx, msg)
}

// ReaderFunc adapts an io.Reader to read into a Buffer.
func ReaderFunc(r io.Reader, buf *Buffer, n int) (int, error) {
	tmp := make([]byte, n)
	read, err := r.Read(tmp)
	if read > 0 {
		buf.Write(tmp[:read])
	}
	return read, err
}
