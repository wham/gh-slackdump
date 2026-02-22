package auth

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"net/http"
	"runtime"
	"testing"

	"golang.org/x/crypto/pbkdf2"
)

func TestDesktopProviderValidate(t *testing.T) {
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
			p := &DesktopProvider{token: tt.token}
			err := p.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDesktopProviderAccessors(t *testing.T) {
	cookies := []*http.Cookie{{Name: "d", Value: "abc"}}
	p := &DesktopProvider{token: "xoxc-test", cookies: cookies}

	if got := p.SlackToken(); got != "xoxc-test" {
		t.Errorf("SlackToken() = %q, want %q", got, "xoxc-test")
	}
	if got := p.Cookies(); len(got) != 1 || got[0].Name != "d" {
		t.Errorf("Cookies() unexpected result: %v", got)
	}
}

func TestDesktopProviderHTTPClient(t *testing.T) {
	p := &DesktopProvider{token: "xoxc-test", cookies: []*http.Cookie{{Name: "d", Value: "abc"}}}
	client, err := p.HTTPClient()
	if err != nil {
		t.Fatalf("HTTPClient() error: %v", err)
	}
	if client == nil {
		t.Fatal("HTTPClient() returned nil")
	}
	if client.Transport == nil {
		t.Fatal("HTTPClient().Transport is nil, expected cookieTransport")
	}
}

func TestDecryptCookie(t *testing.T) {
	// Encrypt a known value with known key to test decryption
	plaintext := []byte("test-cookie-value")
	key := []byte("test-password")

	// Use the same PBKDF2 rounds that decryptCookie uses on this platform
	rounds := 1003 // macOS
	if runtime.GOOS == "linux" {
		rounds = 1
	}
	dk := pbkdf2.Key(key, []byte("saltysalt"), rounds, 16, sha1.New)

	block, err := aes.NewCipher(dk)
	if err != nil {
		t.Fatalf("NewCipher error: %v", err)
	}

	// Add PKCS7 padding
	padLen := aes.BlockSize - len(plaintext)%aes.BlockSize
	padded := make([]byte, len(plaintext)+padLen)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padLen)
	}

	// Encrypt with the same IV (all spaces)
	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = ' '
	}
	mode := cipher.NewCBCEncrypter(block, iv)
	encrypted := make([]byte, len(padded))
	mode.CryptBlocks(encrypted, padded)

	// Now test decryption
	decrypted, err := decryptCookie(encrypted, key)
	if err != nil {
		t.Fatalf("decryptCookie error: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("decryptCookie() = %q, want %q", decrypted, plaintext)
	}
}

func TestRemoveDomainHashPrefix(t *testing.T) {
	// slack.com prefix
	slackPrefix := []byte{3, 202, 236, 172, 132, 247, 212, 240, 217, 211, 68, 226, 103, 153, 245, 64, 85, 68, 2, 183, 83, 182, 186, 218, 14, 102, 237, 62, 231, 241, 231, 142}
	// .slack.com prefix
	dotSlackPrefix := []byte{145, 28, 115, 68, 173, 92, 42, 78, 104, 243, 5, 63, 24, 206, 51, 190, 31, 169, 160, 244, 247, 106, 147, 228, 60, 68, 92, 134, 105, 199, 162, 120}

	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{
			name:  "with slack.com prefix",
			input: append(slackPrefix, []byte("cookie-value")...),
			want:  "cookie-value",
		},
		{
			name:  "with .slack.com prefix",
			input: append(dotSlackPrefix, []byte("cookie-value")...),
			want:  "cookie-value",
		},
		{
			name:  "without prefix",
			input: []byte("cookie-value"),
			want:  "cookie-value",
		},
		{
			name:  "empty input",
			input: []byte{},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := removeDomainHashPrefix(tt.input)
			if !bytes.Equal(got, []byte(tt.want)) {
				t.Errorf("removeDomainHashPrefix() = %q, want %q", got, tt.want)
			}
		})
	}
}
