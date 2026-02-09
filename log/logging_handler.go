package log

import (
	"log/slog"
	"net"

	"github.com/osanha/nettix-go/util"
)

// LoggingHandler logs I/O and exception events.
type LoggingHandler struct {
	logger            *slog.Logger
	excludes          map[string]bool
	allEventsLogging  bool
	attachmentLogging bool
}

// NewLoggingHandler creates a new LoggingHandler.
func NewLoggingHandler() *LoggingHandler {
	return &LoggingHandler{
		logger: slog.Default().With("component", "LoggingHandler"),
	}
}

// NewLoggingHandlerWithSuffix creates a new LoggingHandler with a logger name suffix.
func NewLoggingHandlerWithSuffix(suffix string) *LoggingHandler {
	return &LoggingHandler{
		logger: slog.Default().With("component", "LoggingHandler."+suffix),
	}
}

// SetAllEventsLogging enables or disables logging of all I/O events.
func (h *LoggingHandler) SetAllEventsLogging(enabled bool) {
	h.allEventsLogging = enabled
}

// SetAttachmentLogging enables or disables logging of attached objects.
func (h *LoggingHandler) SetAttachmentLogging(enabled bool) {
	h.attachmentLogging = enabled
}

// SetLogExcludes sets the set of IP addresses to exclude from logging.
func (h *LoggingHandler) SetLogExcludes(excludes []string) {
	h.excludes = make(map[string]bool)
	for _, ip := range excludes {
		h.excludes[ip] = true
	}
}

// IsExcluded checks if an address should be excluded from logging.
func (h *LoggingHandler) IsExcluded(addr string) bool {
	if h.excludes == nil {
		return false
	}
	return h.excludes[addr]
}

// LogConnect logs a connection event.
func (h *LoggingHandler) LogConnect(conn net.Conn) {
	if h.IsExcluded(util.GetRemoteAddress(conn)) {
		return
	}
	h.logger.Info("Connected", "remote", conn.RemoteAddr().String())
}

// LogDisconnect logs a disconnection event.
func (h *LoggingHandler) LogDisconnect(conn net.Conn) {
	if h.IsExcluded(util.GetRemoteAddress(conn)) {
		return
	}
	h.logger.Info("Disconnected", "remote", conn.RemoteAddr().String())
}

// LogClose logs a close event.
func (h *LoggingHandler) LogClose(conn net.Conn) {
	if h.IsExcluded(util.GetRemoteAddress(conn)) {
		return
	}
	h.logger.Info("Closed", "remote", conn.RemoteAddr().String())
}

// LogMessage logs a message event (only at DEBUG level).
func (h *LoggingHandler) LogMessage(conn net.Conn, direction string, data []byte) {
	if h.IsExcluded(util.GetRemoteAddress(conn)) {
		return
	}
	if h.allEventsLogging {
		h.logger.Debug(direction,
			"remote", conn.RemoteAddr().String(),
			"data", util.ToHexDump(data))
	}
}

// LogError logs an error event.
func (h *LoggingHandler) LogError(conn net.Conn, err error) {
	if conn != nil && h.IsExcluded(util.GetRemoteAddress(conn)) {
		return
	}
	h.logger.Error("Exception",
		"remote", conn.RemoteAddr().String(),
		"error", err)
}
