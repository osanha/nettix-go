package util

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"

	"github.com/google/uuid"
)

const (
	// Newline is the platform-specific newline character.
	Newline = "\n"
)

var (
	byte2Hex     [256]string
	hexPadding   [16]string
	bytePadding  [16]string
	byte2Char    [256]byte
)

func init() {
	// Generate the lookup table for byte-to-hex conversion
	for i := 0; i < 256; i++ {
		byte2Hex[i] = fmt.Sprintf(" %02x", i)
	}

	// Generate lookup table for hex dump paddings
	for i := 0; i < 16; i++ {
		padding := 16 - i
		byte2Char[i] = '.'
		hexPadding[i] = strings.Repeat("   ", padding)
		bytePadding[i] = strings.Repeat(" ", padding)
	}

	// Generate lookup table for byte-to-char conversion
	for i := 0; i < 256; i++ {
		if i <= 0x1f || i >= 0x7f {
			byte2Char[i] = '.'
		} else {
			byte2Char[i] = byte(i)
		}
	}
}

// ToHexString converts a byte slice to a hex string.
func ToHexString(data []byte) string {
	return hex.EncodeToString(data)
}

// ToHexDump returns a formatted hex dump of the byte slice.
func ToHexDump(data []byte) string {
	length := len(data)
	rows := length/16 + 1
	if length%16 == 0 && length > 0 {
		rows = length / 16
	}

	var sb strings.Builder
	sb.Grow(rows*80 + 200)

	sb.WriteString(Newline)
	sb.WriteString("         +-------------------------------------------------+")
	sb.WriteString(Newline)
	sb.WriteString("         |  0  1  2  3  4  5  6  7  8  9  a  b  c  d  e  f |")
	sb.WriteString(Newline)
	sb.WriteString("+--------+-------------------------------------------------+----------------+")

	for i := 0; i < length; i++ {
		relIdx := i
		relIdxMod16 := relIdx & 15

		if relIdxMod16 == 0 {
			sb.WriteString(Newline)
			sb.WriteString(fmt.Sprintf("|%08x|", relIdx))
		}

		sb.WriteString(byte2Hex[data[i]])

		if relIdxMod16 == 15 {
			sb.WriteString(" |")
			for j := i - 15; j <= i; j++ {
				sb.WriteByte(byte2Char[data[j]])
			}
			sb.WriteByte('|')
		}
	}

	if length&15 != 0 {
		remainder := length & 15
		sb.WriteString(hexPadding[remainder])
		sb.WriteString(" |")
		for j := length - remainder; j < length; j++ {
			sb.WriteByte(byte2Char[data[j]])
		}
		sb.WriteString(bytePadding[remainder])
		sb.WriteByte('|')
	}

	sb.WriteString(Newline)
	sb.WriteString("+--------+-------------------------------------------------+----------------+")

	return sb.String()
}

// ReadHexString converts a hex string to a byte slice.
func ReadHexString(s string) ([]byte, error) {
	// Remove whitespace
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	return hex.DecodeString(s)
}

// RandomUUID generates a random UUID encoded in URL-safe Base64.
func RandomUUID(length int) string {
	data := make([]byte, length)
	for i := 0; i < length; i += 16 {
		id := uuid.New()
		copy(data[i:], id[:])
	}
	return base64.URLEncoding.EncodeToString(data[:length])
}

// TrimTail trims a string at the first occurrence of a specific character.
func TrimTail(s string, ch rune) string {
	if i := strings.IndexRune(s, ch); i >= 0 {
		return s[:i]
	}
	return s
}

// Concat concatenates strings efficiently.
func Concat(parts ...string) string {
	var sb strings.Builder
	for _, p := range parts {
		sb.WriteString(p)
	}
	return sb.String()
}

// ToDigestHexString generates a message digest as a hex string.
func ToDigestHexString(s string, algorithm string) (string, error) {
	var h hash.Hash
	switch strings.ToUpper(algorithm) {
	case "MD5":
		h = md5.New()
	case "SHA-1", "SHA1":
		h = sha1.New()
	case "SHA-256", "SHA256":
		h = sha256.New()
	default:
		return "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}
	io.WriteString(h, s)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Split splits a string by a delimiter character.
func Split(src string, c rune) []string {
	return strings.Split(src, string(c))
}
