package auth

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// readSafariCookie reads the Slack "d" cookie from Safari's binary cookie file.
// Returns empty string if Safari cookies are not available or the "d" cookie is not found.
func readSafariCookie() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(home, "Library", "Containers", "com.apple.Safari", "Data", "Library", "Cookies", "Cookies.binarycookies")
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("Safari cookies not found at %s", path)
	}

	slog.Info("reading Safari cookies", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return parseSafariDCookie(data)
}

// parseSafariDCookie parses Apple's Cookies.binarycookies format and returns
// the value of the Slack "d" cookie. Returns empty string if not found.
// Format: "cook" magic, big-endian page count + sizes, then pages with little-endian cookie records.
func parseSafariDCookie(data []byte) (string, error) {
	if len(data) < 4 {
		return "", fmt.Errorf("cookie file too short")
	}

	r := bytes.NewReader(data[4:]) // skip "cook" magic
	var numPages int32
	if err := binary.Read(r, binary.BigEndian, &numPages); err != nil {
		return "", fmt.Errorf("reading page count: %w", err)
	}
	pageSizes := make([]int32, numPages)
	for i := range pageSizes {
		if err := binary.Read(r, binary.BigEndian, &pageSizes[i]); err != nil {
			return "", fmt.Errorf("reading page size: %w", err)
		}
	}
	for _, ps := range pageSizes {
		pageData := make([]byte, ps)
		if _, err := io.ReadFull(r, pageData); err != nil {
			return "", fmt.Errorf("reading page: %w", err)
		}
		if len(pageData) < 8 {
			continue
		}
		numCookies := binary.LittleEndian.Uint32(pageData[4:8])
		if len(pageData) < int(8+numCookies*4) {
			continue
		}
		for i := uint32(0); i < numCookies; i++ {
			offset := binary.LittleEndian.Uint32(pageData[8+i*4:])
			if int(offset+4) > len(pageData) {
				continue
			}
			cookieSize := int(binary.LittleEndian.Uint32(pageData[offset:]))
			start := int(offset) + 4
			if start+cookieSize > len(pageData) || cookieSize < 44 {
				continue
			}
			cd := pageData[start : start+cookieSize]

			readStr := func(fieldOffset int) string {
				off := int(binary.LittleEndian.Uint32(cd[fieldOffset:fieldOffset+4])) - 4
				if off < 0 || off >= len(cd) {
					return ""
				}
				end := bytes.IndexByte(cd[off:], 0)
				if end < 0 {
					return string(cd[off:])
				}
				return string(cd[off : off+end])
			}

			domain := readStr(12) // url offset field
			if !strings.Contains(domain, "slack.com") {
				continue
			}
			name := readStr(16) // name offset field
			if name != "d" {
				continue
			}
			value := readStr(24) // value offset field
			return strings.ReplaceAll(value, `"`, ""), nil
		}
	}
	return "", nil
}
