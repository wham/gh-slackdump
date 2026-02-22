package auth

import (
	"bytes"
	"encoding/binary"
	"math"
	"net/http"
	"testing"
	"time"
)

func TestSafariProviderValidate(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{name: "valid token", token: "xoxc-abc123", wantErr: false},
		{name: "empty token", token: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &SafariProvider{token: tt.token}
			err := p.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSafariProviderAccessors(t *testing.T) {
	cookies := []*http.Cookie{{Name: "d", Value: "abc"}}
	p := &SafariProvider{token: "xoxc-test", ua: "TestAgent", cookies: cookies}

	if got := p.SlackToken(); got != "xoxc-test" {
		t.Errorf("SlackToken() = %q, want %q", got, "xoxc-test")
	}
	if got := p.Cookies(); len(got) != 1 || got[0].Name != "d" {
		t.Errorf("Cookies() unexpected result: %v", got)
	}
}

// buildBinaryCookies constructs a minimal Cookies.binarycookies file for testing.
func buildBinaryCookies(cookies []testCookie) []byte {
	var page bytes.Buffer

	// Page header: 0x00000100 magic, numCookies, offsets
	binary.Write(&page, binary.LittleEndian, uint32(0x00000100))
	binary.Write(&page, binary.LittleEndian, uint32(len(cookies)))

	// We'll compute cookie data first, then backfill offsets
	headerSize := 8 + len(cookies)*4
	var cookieBlobs [][]byte
	offsets := make([]uint32, len(cookies))
	currentOffset := uint32(headerSize)

	for i, c := range cookies {
		blob := encodeCookie(c)
		cookieBlobs = append(cookieBlobs, blob)
		offsets[i] = currentOffset
		currentOffset += uint32(len(blob))
	}

	// Write offsets
	for _, off := range offsets {
		binary.Write(&page, binary.LittleEndian, off)
	}
	// Write cookie blobs
	for _, blob := range cookieBlobs {
		page.Write(blob)
	}

	pageBytes := page.Bytes()

	// Build full file
	var file bytes.Buffer
	file.WriteString("cook")
	binary.Write(&file, binary.BigEndian, int32(1)) // 1 page
	binary.Write(&file, binary.BigEndian, int32(len(pageBytes)))
	file.Write(pageBytes)

	return file.Bytes()
}

type testCookie struct {
	domain, name, path, value string
	flags                     uint32
	expiry                    time.Time
}

func encodeCookie(c testCookie) []byte {
	// Build strings: url, name, path, value (null-terminated)
	urlBytes := append([]byte(c.domain), 0)
	nameBytes := append([]byte(c.name), 0)
	pathBytes := append([]byte(c.path), 0)
	valueBytes := append([]byte(c.value), 0)

	// Cookie record layout (after the 4-byte size prefix):
	// 0-3: ignored (we set to 0)
	// 4-7: flags
	// 8-11: ignored
	// 12-15: url offset (relative, +4 to account for size prefix)
	// 16-19: name offset
	// 20-23: path offset
	// 24-27: value offset
	// 28-35: ignored
	// 36-43: expiry (float64, Mac epoch)
	// 44+: string data

	stringsStart := uint32(44)
	urlOff := stringsStart + 4
	nameOff := urlOff + uint32(len(urlBytes))
	pathOff := nameOff + uint32(len(nameBytes))
	valueOff := pathOff + uint32(len(pathBytes))

	totalSize := int(valueOff) + len(valueBytes) - 4 // subtract the 4-byte size prefix

	var buf bytes.Buffer
	// Size prefix (written before the cookie data, but parseSafariBinaryCookies reads it at offset)
	binary.Write(&buf, binary.LittleEndian, uint32(totalSize))
	// cd[0:4] - ignored
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	// cd[4:8] - flags
	binary.Write(&buf, binary.LittleEndian, c.flags)
	// cd[8:12] - ignored
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	// cd[12:16] - url offset
	binary.Write(&buf, binary.LittleEndian, urlOff)
	// cd[16:20] - name offset
	binary.Write(&buf, binary.LittleEndian, nameOff)
	// cd[20:24] - path offset
	binary.Write(&buf, binary.LittleEndian, pathOff)
	// cd[24:28] - value offset
	binary.Write(&buf, binary.LittleEndian, valueOff)
	// cd[28:36] - ignored
	binary.Write(&buf, binary.LittleEndian, uint64(0))
	// cd[36:44] - expiry as float64 (Mac epoch = Unix - 978307200)
	var expiryMac float64
	if !c.expiry.IsZero() {
		expiryMac = float64(c.expiry.Unix() - 978307200)
	}
	binary.Write(&buf, binary.LittleEndian, math.Float64bits(expiryMac))
	// String data
	buf.Write(urlBytes)
	buf.Write(nameBytes)
	buf.Write(pathBytes)
	buf.Write(valueBytes)

	return buf.Bytes()
}

func TestParseSafariBinaryCookies(t *testing.T) {
	expiry := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	data := buildBinaryCookies([]testCookie{
		{domain: ".slack.com", name: "d", path: "/", value: "abc123", flags: 0x05, expiry: expiry},
		{domain: ".example.com", name: "session", path: "/", value: "xyz", flags: 0, expiry: expiry},
		{domain: ".enterprise.slack.com", name: "d-s", path: "/", value: "ent456", flags: 0x01, expiry: expiry},
	})

	cookies, err := parseSafariBinaryCookies(data)
	if err != nil {
		t.Fatalf("parseSafariBinaryCookies() error: %v", err)
	}

	// Should only have slack.com cookies (2), not example.com
	if len(cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %d", len(cookies))
	}

	// Check first cookie
	if cookies[0].Domain != ".slack.com" {
		t.Errorf("cookie[0].Domain = %q, want %q", cookies[0].Domain, ".slack.com")
	}
	if cookies[0].Name != "d" {
		t.Errorf("cookie[0].Name = %q, want %q", cookies[0].Name, "d")
	}
	if cookies[0].Value != "abc123" {
		t.Errorf("cookie[0].Value = %q, want %q", cookies[0].Value, "abc123")
	}
	if !cookies[0].Secure {
		t.Error("cookie[0].Secure should be true (flag 0x01)")
	}
	if !cookies[0].HttpOnly {
		t.Error("cookie[0].HttpOnly should be true (flag 0x04)")
	}

	// Check second cookie (enterprise)
	if cookies[1].Domain != ".enterprise.slack.com" {
		t.Errorf("cookie[1].Domain = %q, want %q", cookies[1].Domain, ".enterprise.slack.com")
	}
}

func TestParseSafariBinaryCookiesEmpty(t *testing.T) {
	data := buildBinaryCookies(nil)
	cookies, err := parseSafariBinaryCookies(data)
	if err != nil {
		t.Fatalf("parseSafariBinaryCookies() error: %v", err)
	}
	if len(cookies) != 0 {
		t.Errorf("expected 0 cookies, got %d", len(cookies))
	}
}
