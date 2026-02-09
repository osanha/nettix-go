package util

import (
	"bytes"
	"compress/flate"
	"io"
	"net"
	"strings"
	"time"
)

// GetRemoteAddress returns the remote IP address from a connection.
func GetRemoteAddress(conn net.Conn) string {
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		return addr.IP.String()
	}
	return conn.RemoteAddr().String()
}

// GetLocalAddresses returns the set of local system IP addresses.
func GetLocalAddresses() ([]string, error) {
	var addresses []string

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLinkLocalUnicast() {
				continue
			}

			addresses = append(addresses, ip.String())
		}
	}

	return addresses, nil
}

// Inflate decompresses deflated data.
func Inflate(data []byte) ([]byte, error) {
	reader := flate.NewReader(bytes.NewReader(data))
	defer reader.Close()
	return io.ReadAll(reader)
}

// Deflate compresses data using deflate algorithm.
func Deflate(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := flate.NewWriter(&buf, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GetUTCTime returns the current UTC time as a formatted string.
func GetUTCTime() string {
	return GetUTCTimeFor(time.Now())
}

// GetUTCTimeFor returns a UTC time string for a given time.
func GetUTCTimeFor(t time.Time) string {
	return t.UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")
}

// LineResult represents the result of line parsing.
type LineResult struct {
	Str  string
	Next int
}

// GetLine reads a line from a string starting at a specific index.
func GetLine(str string, index int) *LineResult {
	if index >= len(str) {
		return nil
	}

	if i := strings.Index(str[index:], "\n"); i >= 0 {
		return &LineResult{
			Str:  str[index : index+i],
			Next: index + i + 1,
		}
	}

	return &LineResult{
		Str:  str[index:],
		Next: -1,
	}
}
