package auth

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	utls "github.com/refraction-networking/utls"
	"github.com/rusq/slack"
	"github.com/rusq/slackdump/v3/auth"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/net/http2"
	"golang.org/x/net/publicsuffix"
	_ "modernc.org/sqlite"
)

// Provider wraps slackdump's ValueAuth with uTLS fingerprinting
// to mimic Safari's TLS fingerprint.
type Provider struct {
	auth.ValueAuth
}

func (p *Provider) HTTPClient() (*http.Client, error) {
	jar, err := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	if err != nil {
		return nil, err
	}
	u, _ := url.Parse(auth.SlackURL)
	jar.SetCookies(u, p.Cookies())
	return &http.Client{
		Jar:       jar,
		Transport: &utlsTransport{h2: &http2.Transport{}},
	}, nil
}

func (p *Provider) Test(ctx context.Context) (*slack.AuthTestResponse, error) {
	cl, err := p.HTTPClient()
	if err != nil {
		return nil, err
	}
	return slack.New(p.SlackToken(), slack.OptionHTTPClient(cl)).AuthTestContext(ctx)
}

// utlsTransport uses uTLS to mimic Safari's TLS fingerprint.
// It caches and reuses HTTP/2 connections per host, matching real browser behavior.
type utlsTransport struct {
	h2   *http2.Transport
	mu   sync.Mutex
	h2cc map[string]*http2.ClientConn
}

func (t *utlsTransport) getOrDialH2(req *http.Request) (*http2.ClientConn, error) {
	addr := req.URL.Host
	if req.URL.Port() == "" {
		addr += ":443"
	}

	t.mu.Lock()
	if t.h2cc == nil {
		t.h2cc = make(map[string]*http2.ClientConn)
	}
	cc, ok := t.h2cc[addr]
	t.mu.Unlock()

	if ok && cc.CanTakeNewRequest() {
		return cc, nil
	}

	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, err
	}

	tlsConn := utls.UClient(conn, &utls.Config{ServerName: req.URL.Hostname()}, utls.HelloSafari_Auto)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}

	cc, err = t.h2.NewClientConn(tlsConn)
	if err != nil {
		conn.Close()
		return nil, err
	}

	t.mu.Lock()
	t.h2cc[addr] = cc
	t.mu.Unlock()

	return cc, nil
}

const safariUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.3 Safari/605.1.15"

func (t *utlsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req = req.Clone(req.Context())
		req.Header.Set("User-Agent", safariUserAgent)
	}
	cc, err := t.getOrDialH2(req)
	if err != nil {
		return nil, err
	}
	return cc.RoundTrip(req)
}

// NewProvider creates a new auth provider by reading the Slack "d" cookie
// from the Slack desktop app and exchanging it for a Slack API token.
// All connections use uTLS to mimic Safari's TLS fingerprint.
func NewProvider(ctx context.Context, workspaceURL string) (*Provider, error) {
	cookie, err := readDesktopCookie()
	if err != nil {
		return nil, fmt.Errorf("reading Slack cookie: %w", err)
	}
	if cookie == "" {
		return nil, errors.New("no Slack cookies found — sign in to Slack in the Slack desktop app")
	}

	slog.Info("trying cookie", "source", "Slack desktop app")
	token, err := exchangeCookieForToken(workspaceURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("cookie did not work for workspace: %w", err)
	}

	slog.Info("authenticated", "source", "Slack desktop app")
	va, err := auth.NewValueAuth(token, cookie)
	if err != nil {
		return nil, fmt.Errorf("creating auth: %w", err)
	}
	return &Provider{ValueAuth: va}, nil
}

var apiTokenRE = regexp.MustCompile(`"api_token":"([^"]+)"`)

// exchangeCookieForToken exchanges a Slack "d" cookie for an API token
// by hitting the workspace's /ssb/redirect endpoint through uTLS.
func exchangeCookieForToken(workspaceURL, cookie string) (string, error) {
	req, err := http.NewRequest("GET", workspaceURL+"/ssb/redirect", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Cookie", "d="+cookie)
	req.Header.Set("User-Agent", safariUserAgent)

	client := &http.Client{
		Transport: &utlsTransport{h2: &http2.Transport{}},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	matches := apiTokenRE.FindSubmatch(body)
	if len(matches) < 2 {
		return "", errors.New("api token not found in response")
	}

	return string(matches[1]), nil
}

// ReadCookie reads the Slack "d" cookie from the Slack desktop app's cookie database.
func ReadCookie() (string, error) {
	cookie, err := readDesktopCookie()
	if err != nil {
		return "", err
	}
	slog.Info("using Slack desktop cookie")
	return cookie, nil
}

// readDesktopCookie reads and decrypts the Slack "d" cookie from the
// Slack desktop app's local cookie database.
func readDesktopCookie() (string, error) {
	dbPath, err := slackCookieDBPath()
	if err != nil {
		return "", err
	}

	slog.Info("reading Slack cookie", "path", dbPath)

	db, err := sql.Open("sqlite", dbPath+"?mode=ro")
	if err != nil {
		return "", fmt.Errorf("opening cookie database: %w", err)
	}
	defer db.Close()

	var cookie string
	var encryptedValue []byte
	err = db.QueryRow(`SELECT value, encrypted_value FROM cookies WHERE host_key=".slack.com" AND name="d"`).Scan(&cookie, &encryptedValue)
	if err != nil {
		return "", fmt.Errorf("querying cookie: %w", err)
	}

	if cookie != "" {
		return cookie, nil
	}

	if len(encryptedValue) < 4 {
		return "", errors.New("encrypted cookie value too short")
	}

	// Remove version prefix (e.g. "v11" = 3 bytes)
	encryptedValue = encryptedValue[3:]

	key, err := cookiePassword()
	if err != nil {
		return "", fmt.Errorf("getting cookie password: %w", err)
	}

	decrypted, err := decryptCookie(encryptedValue, key)
	if err != nil {
		return "", fmt.Errorf("decrypting cookie: %w", err)
	}

	decrypted = removeDomainHashPrefix(decrypted)

	return string(decrypted), nil
}

// decryptCookie decrypts a Chromium-encrypted cookie value using PBKDF2 + AES-CBC.
func decryptCookie(value, key []byte) ([]byte, error) {
	dk := pbkdf2.Key(key, []byte("saltysalt"), 1003, 16, sha1.New)

	block, err := aes.NewCipher(dk)
	if err != nil {
		return nil, err
	}

	iv := make([]byte, 16)
	for i := range iv {
		iv[i] = ' '
	}

	decrypted := make([]byte, len(value))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(decrypted, value)

	// Remove PKCS7 padding
	if len(decrypted) > 0 {
		pad := int(decrypted[len(decrypted)-1])
		if pad > 0 && pad <= aes.BlockSize && pad <= len(decrypted) {
			decrypted = decrypted[:len(decrypted)-pad]
		}
	}

	return decrypted, nil
}

// Chromium prefixes encrypted cookie values with a SHA256 hash of the domain.
// See https://chromium-review.googlesource.com/c/chromium/src/+/5792044
var domainHashPrefixes = [][]byte{
	// slack.com
	{3, 202, 236, 172, 132, 247, 212, 240, 217, 211, 68, 226, 103, 153, 245, 64, 85, 68, 2, 183, 83, 182, 186, 218, 14, 102, 237, 62, 231, 241, 231, 142},
	// .slack.com
	{145, 28, 115, 68, 173, 92, 42, 78, 104, 243, 5, 63, 24, 206, 51, 190, 31, 169, 160, 244, 247, 106, 147, 228, 60, 68, 92, 134, 105, 199, 162, 120},
}

func removeDomainHashPrefix(value []byte) []byte {
	for _, prefix := range domainHashPrefixes {
		if bytes.HasPrefix(value, prefix) {
			return value[len(prefix):]
		}
	}
	return value
}

// slackCookieDBPath returns the path to the Slack desktop app's cookie database.
func slackCookieDBPath() (string, error) {
	dir, err := slackConfigDir()
	if err != nil {
		return "", err
	}

	cookieFile := filepath.Join(dir, "Cookies")

	if _, err := os.Stat(cookieFile); err != nil {
		return "", fmt.Errorf("Slack cookie database not found at %s — is the Slack desktop app installed and signed in?", cookieFile)
	}

	return cookieFile, nil
}

// slackConfigDir returns the Slack desktop app's configuration directory.
func slackConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	first := filepath.Join(home, "Library", "Application Support", "Slack")
	second := filepath.Join(home, "Library", "Containers", "com.tinyspeck.slackmacgap", "Data", "Library", "Application Support", "Slack")
	if _, err := os.Stat(first); err == nil {
		return first, nil
	}
	return second, nil
}

