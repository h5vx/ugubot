package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Signer issues and checks "timestamp:hmac" session tokens. The format is the
// same as in the Python version, so existing sessions stay valid.
type Signer struct {
	key        []byte
	expiration time.Duration // 0 = never expires
	now        func() time.Time
}

func NewSigner(key, expiration string) (*Signer, error) {
	if key == "" {
		return nil, fmt.Errorf("webui.signing_key is not set")
	}
	exp, err := parseExpiration(expiration)
	if err != nil {
		return nil, err
	}
	return &Signer{key: []byte(key), expiration: exp, now: time.Now}, nil
}

// parseExpiration parses "<number><s|m|h|d|w>"; empty means no expiration.
func parseExpiration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	units := map[byte]time.Duration{'s': time.Second, 'm': time.Minute, 'h': time.Hour, 'd': 24 * time.Hour, 'w': 7 * 24 * time.Hour}
	unit, ok := units[s[len(s)-1]]
	n, err := strconv.Atoi(s[:len(s)-1])
	if !ok || err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid webui.auth_expiration %q: expected <number><s|m|h|d|w>", s)
	}
	return time.Duration(n) * unit, nil
}

func (s *Signer) sign(value string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *Signer) Token() string {
	value := strconv.FormatInt(s.now().Unix(), 10)
	return value + ":" + s.sign(value)
}

func (s *Signer) Check(token string) bool {
	value, signature, ok := strings.Cut(token, ":")
	if !ok || value == "" {
		return false
	}
	if !hmac.Equal([]byte(signature), []byte(s.sign(value))) {
		return false
	}
	if s.expiration == 0 {
		return true
	}
	issued, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return false
	}
	return s.now().Before(time.Unix(issued, 0).Add(s.expiration))
}

// checkPassword compares the password against configured sha512 hex digests.
func checkPassword(password string, hashes []string) bool {
	sum := sha512.Sum512([]byte(password))
	given := []byte(hex.EncodeToString(sum[:]))
	ok := false
	for _, h := range hashes {
		if subtle.ConstantTimeCompare(given, []byte(strings.ToLower(strings.TrimSpace(h)))) == 1 {
			ok = true
		}
	}
	return ok
}
