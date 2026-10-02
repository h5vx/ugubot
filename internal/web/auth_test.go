package web

import (
	"crypto/sha512"
	"encoding/hex"
	"testing"
	"time"
)

func TestSigner(t *testing.T) {
	s, err := NewSigner("secret", "1w")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_700_000_000, 0)
	s.now = func() time.Time { return now }

	token := s.Token()
	if !s.Check(token) {
		t.Fatal("fresh token rejected")
	}
	if s.Check(token + "0") {
		t.Fatal("tampered token accepted")
	}
	if s.Check("1700000000:") || s.Check("garbage") {
		t.Fatal("malformed token accepted")
	}

	now = now.Add(8 * 24 * time.Hour)
	if s.Check(token) {
		t.Fatal("expired token accepted")
	}
}

// Token produced by util/signer.py of the Python version with key "secret".
func TestSignerCompatibleWithPython(t *testing.T) {
	s, _ := NewSigner("secret", "")
	if !s.Check("1700000000:4b227f8831b3763d066901751ad4c583ed08832bf1924a4ec50c2e871b1e8586") {
		t.Fatal("python token rejected")
	}
}

func TestCheckPassword(t *testing.T) {
	sum := sha512.Sum512([]byte("hunter2"))
	hash := hex.EncodeToString(sum[:])

	if !checkPassword("hunter2", []string{"other", hash}) {
		t.Fatal("correct password rejected")
	}
	if checkPassword("hunter3", []string{hash}) {
		t.Fatal("wrong password accepted")
	}
}

func TestParseExpiration(t *testing.T) {
	for in, want := range map[string]time.Duration{"": 0, "30s": 30 * time.Second, "2d": 48 * time.Hour, "1w": 7 * 24 * time.Hour} {
		got, err := parseExpiration(in)
		if err != nil || got != want {
			t.Errorf("parseExpiration(%q) = %v, %v", in, got, err)
		}
	}
	if _, err := parseExpiration("5y"); err == nil {
		t.Error("invalid unit accepted")
	}
}
