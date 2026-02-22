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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/rusq/slack"
	"github.com/rusq/slackdump/v3/auth"
	"golang.org/x/crypto/pbkdf2"
	_ "modernc.org/sqlite"
)

// DesktopProvider implements auth.Provider by reading authentication
// from the Slack desktop app's local cookie database. This approach
// is based on how gh-slack (github.com/rneatherway/gh-slack) handles
// authentication.
type DesktopProvider struct {
	token   string
	cookies []*http.Cookie
}

func (p *DesktopProvider) SlackToken() string     { return p.token }
func (p *DesktopProvider) Cookies() []*http.Cookie { return p.cookies }
func (p *DesktopProvider) Validate() error {
	if p.token == "" {
		return auth.ErrNoToken
	}
	return nil
}

func (p *DesktopProvider) HTTPClient() (*http.Client, error) {
	return &http.Client{
		Transport: &cookieTransport{cookies: p.cookies},
	}, nil
}

func (p *DesktopProvider) Test(ctx context.Context) (*slack.AuthTestResponse, error) {
	cl, err := p.HTTPClient()
	if err != nil {
		return nil, err
	}
	return slack.New(p.token, slack.OptionHTTPClient(cl)).AuthTestContext(ctx)
}

// cookieTransport adds cookies to every outgoing request.
type cookieTransport struct {
	cookies []*http.Cookie
}

func (t *cookieTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	for _, c := range t.cookies {
		r.AddCookie(c)
	}
	return http.DefaultTransport.RoundTrip(r)
}

// NewDesktopProvider creates a new auth provider by reading the Slack desktop
// app's cookie and exchanging it for a Slack API token.
func NewDesktopProvider(ctx context.Context, workspaceURL string) (*DesktopProvider, error) {
	cookie, err := ReadDesktopCookie()
	if err != nil {
		return nil, fmt.Errorf("reading Slack desktop cookie: %w", err)
	}

	token, cookies, err := exchangeCookieForToken(workspaceURL, cookie)
	if err != nil {
		return nil, fmt.Errorf("exchanging cookie for token: %w", err)
	}

	return &DesktopProvider{token: token, cookies: cookies}, nil
}

// ReadDesktopCookie reads and decrypts the Slack "d" cookie from the
// Slack desktop app's local cookie database.
func ReadDesktopCookie() (string, error) {
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

var apiTokenRE = regexp.MustCompile(`"api_token":"([^"]+)"`)

func exchangeCookieForToken(workspaceURL, cookie string) (string, []*http.Cookie, error) {
	req, err := http.NewRequest("GET", workspaceURL, nil)
	if err != nil {
		return "", nil, err
	}
	req.AddCookie(&http.Cookie{Name: "d", Value: cookie})

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, err
	}

	matches := apiTokenRE.FindSubmatch(body)
	if len(matches) < 2 {
		return "", nil, errors.New("api token not found in response")
	}

	token := string(matches[1])
	cookies := []*http.Cookie{{Name: "d", Value: cookie}}

	// Include any additional cookies from the response
	for _, c := range resp.Cookies() {
		if c.Name != "d" {
			cookies = append(cookies, c)
		}
	}

	return token, cookies, nil
}

// decryptCookie decrypts a Chromium-encrypted cookie value using PBKDF2 + AES-CBC.
// This works for macOS (1003 PBKDF2 rounds) and Linux (1 round).
func decryptCookie(value, key []byte) ([]byte, error) {
	rounds := 1003 // macOS default
	if runtime.GOOS == "linux" {
		rounds = 1
	}

	dk := pbkdf2.Key(key, []byte("saltysalt"), rounds, 16, sha1.New)

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
	if runtime.GOOS == "windows" {
		cookieFile = filepath.Join(dir, "Network", "Cookies")
	}

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

	switch runtime.GOOS {
	case "darwin":
		first := filepath.Join(home, "Library", "Application Support", "Slack")
		second := filepath.Join(home, "Library", "Containers", "com.tinyspeck.slackmacgap", "Data", "Library", "Application Support", "Slack")
		if _, err := os.Stat(first); err == nil {
			return first, nil
		}
		return second, nil
	case "linux":
		if xdgConfigHome, found := os.LookupEnv("XDG_CONFIG_HOME"); found {
			return filepath.Join(xdgConfigHome, "Slack"), nil
		}
		return filepath.Join(home, ".config", "Slack"), nil
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Slack"), nil
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

// cookiePassword retrieves the encryption key for Slack's cookie database
// from the system's secret storage.
func cookiePassword() ([]byte, error) {
	switch runtime.GOOS {
	case "darwin":
		return cookiePasswordDarwin()
	case "linux":
		return cookiePasswordLinux()
	default:
		return nil, fmt.Errorf("cookie decryption not supported on %s", runtime.GOOS)
	}
}

func cookiePasswordDarwin() ([]byte, error) {
	accountNames := []string{"Slack Key", "Slack", "Slack App Store Key"}
	for _, name := range accountNames {
		out, err := exec.Command("security", "find-generic-password",
			"-s", "Slack Safe Storage",
			"-a", name,
			"-w",
		).Output()
		if err == nil {
			return []byte(strings.TrimSpace(string(out))), nil
		}
	}
	return nil, fmt.Errorf("could not find Slack cookie password in Keychain")
}

func cookiePasswordLinux() ([]byte, error) {
	out, err := exec.Command("secret-tool", "lookup",
		"xdg:schema", "chrome_libsecret_os_crypt_password_v2",
		"application", "Slack",
	).Output()
	if err != nil {
		return nil, fmt.Errorf("could not get Slack cookie password from secret service: %w", err)
	}
	return []byte(strings.TrimSpace(string(out))), nil
}
