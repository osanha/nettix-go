package log

import (
	"log/slog"
	"net"

	"nettix-go/util"
)

// HexLoggingHandler logs messages in hexadecimal format at DEBUG level.
type HexLoggingHandler struct {
	logger *slog.Logger
}

// NewHexLoggingHandler creates a new HexLoggingHandler.
func NewHexLoggingHandler() *HexLoggingHandler {
	return &HexLoggingHandler{
		logger: slog.Default().With("component", "HexLoggingHandler"),
	}
}

// NewHexLoggingHandlerWithSuffix creates a new HexLoggingHandler with a suffix.
func NewHexLoggingHandlerWithSuffix(suffix string) *HexLoggingHandler {
	return &HexLoggingHandler{
		logger: slog.Default().With("component", "HexLoggingHandler."+suffix),
	}
}

// LogReceived logs a received message in hexadecimal format.
func (h *HexLoggingHandler) LogReceived(conn net.Conn, data []byte) {
	h.logger.Debug("RECEIVED",
		"remote", conn.RemoteAddr().String(),
		"hex", util.ToHexDump(data))
}

// LogSent logs a sent message in hexadecimal format.
func (h *HexLoggingHandler) LogSent(conn net.Conn, data []byte) {
	h.logger.Debug("SENT",
		"remote", conn.RemoteAddr().String(),
		"hex", util.ToHexDump(data))
}
