package auth

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	utls "github.com/refraction-networking/utls"
	"github.com/rusq/slack"
	"github.com/rusq/slackdump/v3/auth"
	"golang.org/x/net/http2"
)

// SafariProvider implements auth.Provider with Safari cookie authentication
// and TLS fingerprinting to mimic real Safari browser connections.
type SafariProvider struct {
	token, ua string
	cookies   []*http.Cookie
}

func (p *SafariProvider) SlackToken() string      { return p.token }
func (p *SafariProvider) Cookies() []*http.Cookie  { return p.cookies }
func (p *SafariProvider) Validate() error {
	if p.token == "" {
		return auth.ErrNoToken
	}
	return nil
}

func (p *SafariProvider) HTTPClient() (*http.Client, error) {
	return &http.Client{
		Transport: &safariTransport{ua: p.ua, cookies: p.cookies, h2: &http2.Transport{}},
	}, nil
}

func (p *SafariProvider) Test(ctx context.Context) (*slack.AuthTestResponse, error) {
	cl, err := p.HTTPClient()
	if err != nil {
		return nil, err
	}
	return slack.New(p.token, slack.OptionHTTPClient(cl)).AuthTestContext(ctx)
}

// safariTransport uses uTLS to mimic Safari's TLS fingerprint.
type safariTransport struct {
	ua      string
	cookies []*http.Cookie
	h2      *http2.Transport
}

func (t *safariTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r := req.Clone(req.Context())
	r.Header.Set("User-Agent", t.ua)
	var cb strings.Builder
	for i, c := range t.cookies {
		if i > 0 {
			cb.WriteString("; ")
		}
		cb.WriteString(c.Name + "=" + c.Value)
	}
	r.Header.Set("Cookie", cb.String())

	addr := r.URL.Host
	if r.URL.Port() == "" {
		addr += ":443"
	}
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, err
	}
	tlsConn := utls.UClient(conn, &utls.Config{ServerName: r.URL.Hostname()}, utls.HelloSafari_Auto)
	if err := tlsConn.Handshake(); err != nil {
		conn.Close()
		return nil, err
	}
	if tlsConn.ConnectionState().NegotiatedProtocol == "h2" {
		cc, err := t.h2.NewClientConn(tlsConn)
		if err != nil {
			conn.Close()
			return nil, err
		}
		return cc.RoundTrip(r)
	}
	if err := r.Write(conn); err != nil {
		conn.Close()
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), r)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return resp, nil
}

// ReadSafariCookies reads and parses Safari's binary cookies for Slack
// and detects the Safari User-Agent, without exchanging for a token.
func ReadSafariCookies() (cookies []*http.Cookie, userAgent string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("getting home directory: %w", err)
	}

	safariCookiePath := filepath.Join(home, "Library", "Containers", "com.apple.Safari", "Data", "Library", "Cookies", "Cookies.binarycookies")
	if _, err := os.Stat(safariCookiePath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("Safari cookies not found at %s", safariCookiePath)
	}

	cookies, err = parseCookieFile(safariCookiePath)
	if err != nil {
		return nil, "", fmt.Errorf("parsing Safari cookies: %w", err)
	}

	return cookies, detectSafariUserAgent(), nil
}

// NewSafariProvider creates a new auth provider by reading Safari cookies
// and exchanging them for a Slack API token. The workspaceURL is the base
// URL of the Slack workspace (e.g., "https://myteam.slack.com").
func NewSafariProvider(ctx context.Context, workspaceURL string) (*SafariProvider, error) {
	cookies, ua, err := ReadSafariCookies()
	if err != nil {
		return nil, err
	}

	token, allCookies, err := getTokenFromCookies(workspaceURL, cookies, ua)
	if err != nil {
		return nil, fmt.Errorf("getting Slack token from cookies: %w", err)
	}

	return &SafariProvider{token: token, cookies: allCookies, ua: ua}, nil
}

func getTokenFromCookies(workspaceURL string, cookies []*http.Cookie, userAgent string) (string, []*http.Cookie, error) {
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	}
	req, err := http.NewRequest("GET", workspaceURL+"/ssb/redirect", nil)
	if err != nil {
		return "", nil, fmt.Errorf("creating request: %w", err)
	}
	// Build raw Cookie header to avoid Go's cookie value sanitization
	var cb strings.Builder
	for i, c := range cookies {
		if i > 0 {
			cb.WriteString("; ")
		}
		cb.WriteString(c.Name + "=" + c.Value)
	}
	req.Header.Set("Cookie", cb.String())
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"macOS"`)
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("received status: %s", resp.Status)
	}
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("reading body: %w", err)
	}
	re := regexp.MustCompile(`"api_token":"([^"]+)"`)
	matches := re.FindSubmatch(responseBody)
	if len(matches) < 2 {
		return "", nil, fmt.Errorf("no Slack token found in response")
	}
	token := string(matches[1])

	// Merge input cookies with response cookies (response takes precedence)
	cm := make(map[string]*http.Cookie, len(cookies))
	for _, c := range cookies {
		cm[c.Name] = c
	}
	for _, c := range resp.Cookies() {
		cm[c.Name] = c
	}
	merged := make([]*http.Cookie, 0, len(cm))
	for _, c := range cm {
		merged = append(merged, c)
	}
	return token, merged, nil
}

func parseCookieFile(path string) ([]*http.Cookie, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseSafariBinaryCookies(data)
}

// parseSafariBinaryCookies parses Apple's Cookies.binarycookies format.
// Format: "cook" magic, big-endian page count + sizes, then pages with little-endian cookie records.
func parseSafariBinaryCookies(data []byte) ([]*http.Cookie, error) {
	r := bytes.NewReader(data[4:]) // skip "cook" magic
	var numPages int32
	if err := binary.Read(r, binary.BigEndian, &numPages); err != nil {
		return nil, fmt.Errorf("reading page count: %w", err)
	}
	pageSizes := make([]int32, numPages)
	for i := range pageSizes {
		if err := binary.Read(r, binary.BigEndian, &pageSizes[i]); err != nil {
			return nil, fmt.Errorf("reading page size: %w", err)
		}
	}
	var cookies []*http.Cookie
	for _, ps := range pageSizes {
		pageData := make([]byte, ps)
		if _, err := io.ReadFull(r, pageData); err != nil {
			return nil, fmt.Errorf("reading page: %w", err)
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
			flags := binary.LittleEndian.Uint32(cd[4:8])
			urlOff := int(binary.LittleEndian.Uint32(cd[12:16])) - 4
			nameOff := int(binary.LittleEndian.Uint32(cd[16:20])) - 4
			pathOff := int(binary.LittleEndian.Uint32(cd[20:24])) - 4
			valueOff := int(binary.LittleEndian.Uint32(cd[24:28])) - 4
			expiryMac := math.Float64frombits(binary.LittleEndian.Uint64(cd[36:44]))

			readStr := func(off int) string {
				if off < 0 || off >= len(cd) {
					return ""
				}
				end := bytes.IndexByte(cd[off:], 0)
				if end < 0 {
					return string(cd[off:])
				}
				return string(cd[off : off+end])
			}
			domain := readStr(urlOff)
			if !strings.Contains(domain, "slack.com") {
				continue
			}
			val := strings.ReplaceAll(readStr(valueOff), `"`, "")
			c := &http.Cookie{
				Domain:   domain,
				Name:     readStr(nameOff),
				Path:     readStr(pathOff),
				Value:    val,
				Secure:   flags&1 != 0,
				HttpOnly: flags&4 != 0,
			}
			if expiryMac > 0 {
				c.Expires = time.Unix(int64(expiryMac)+978307200, 0)
			}
			cookies = append(cookies, c)
		}
	}
	return cookies, nil
}

func detectSafariUserAgent() string {
	safariVer, err := exec.Command("defaults", "read", "/Applications/Safari.app/Contents/Info", "CFBundleShortVersionString").Output()
	if err != nil {
		return ""
	}
	macVer, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return ""
	}
	sv := strings.TrimSpace(string(safariVer))
	mv := strings.ReplaceAll(strings.TrimSpace(string(macVer)), ".", "_")
	return fmt.Sprintf("Mozilla/5.0 (Macintosh; Intel Mac OS X %s) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%s Safari/605.1.15", mv, sv)
}
