// Package config loads ugubot settings from settings.toml, .secrets.toml and
// UGUBOT_* environment variables (nested keys are separated by "__", e.g.
// UGUBOT_XMPP__PASSWORD). The file layout is compatible with the old Python
// version of the bot.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Config struct {
	AdminJIDs []string `koanf:"admin_jids"`
	XMPP      XMPP     `koanf:"xmpp"`
	OpenAI    OpenAI   `koanf:"openai"`
	WebUI     WebUI    `koanf:"webui"`
	Database  Database `koanf:"database"`
	NATS      NATS     `koanf:"nats"`
	Log       Log      `koanf:"log"`
}

type XMPP struct {
	JID      string `koanf:"jid"`
	Password string `koanf:"password"`
	Resource string `koanf:"resource"`
	// Host overrides SRV lookup, e.g. "xmpp.example.com:5222".
	Host       string          `koanf:"host"`
	SSLVerify  bool            `koanf:"ssl_verify"`
	Subscribes Subscribes      `koanf:"subscribes"`
	IQ         IQ              `koanf:"iq"`
	Rooms      map[string]Room `koanf:"rooms"`
}

type Subscribes struct {
	AutoApprove bool `koanf:"auto_approve"`
}

type IQ struct {
	Version Version `koanf:"version"`
}

type Version struct {
	Name    string `koanf:"name"`
	Version string `koanf:"version"`
	OS      string `koanf:"os"`
}

type Room struct {
	Join     bool   `koanf:"join"`
	JID      string `koanf:"jid"`
	Nick     string `koanf:"nick"`
	Password string `koanf:"password"`
}

type OpenAI struct {
	Enabled                   bool              `koanf:"enabled"`
	APIKey                    string            `koanf:"api_key"`
	BaseURL                   string            `koanf:"base_url"`
	Model                     string            `koanf:"model"`
	ModelSecondaryCommand     string            `koanf:"model_secondary_command"`
	ModelSecondary            string            `koanf:"model_secondary"`
	UserNick                  string            `koanf:"user_nick"`
	CommandPrefix             string            `koanf:"command_prefix"`
	MaxTokens                 int               `koanf:"max_tokens"`
	TokensReservedForResponse int               `koanf:"tokens_reserved_for_response"`
	Prompt                    map[string]Prompt `koanf:"prompt"`
	// Prices in USD per 1K tokens, keyed by model name prefix ("gpt-4o", "gpt-4").
	// The longest matching prefix wins.
	Prices map[string]Price `koanf:"prices"`
}

type Prompt struct {
	Command string `koanf:"command"`
	Text    string `koanf:"text"`
}

type Price struct {
	Input  float64 `koanf:"input"`
	Output float64 `koanf:"output"`
}

type WebUI struct {
	Listen          string   `koanf:"listen"`
	Port            int      `koanf:"port"`
	Debug           bool     `koanf:"debug"`
	PasswordsSHA512 []string `koanf:"passwords_sha512"`
	SigningKey      string   `koanf:"signing_key"`
	AuthExpiration  string   `koanf:"auth_expiration"`
}

type Database struct {
	// URL is a postgres connection string. When empty it is built from the
	// fields below (the old Pony-style settings).
	URL      string `koanf:"url"`
	Provider string `koanf:"provider"`
	User     string `koanf:"user"`
	Password string `koanf:"password"`
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Database string `koanf:"database"`
}

type NATS struct {
	// URL of the NATS server. "embedded" starts an in-process server
	// (only for `ugubot all`).
	URL string `koanf:"url"`
	// StoreDir is where the embedded server keeps JetStream data.
	StoreDir string `koanf:"store_dir"`
}

type Log struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"` // text | json
}

func defaults() Config {
	return Config{
		XMPP: XMPP{
			Resource: "ugubot",
			IQ:       IQ{Version: Version{Name: "ugubot", Version: "2.0"}},
		},
		OpenAI: OpenAI{
			Model:                     "gpt-4o-mini",
			UserNick:                  "bot",
			CommandPrefix:             "~",
			MaxTokens:                 16384,
			TokensReservedForResponse: 1024,
			Prices: map[string]Price{
				"gpt-3.5":     {Input: 0.0015, Output: 0.002},
				"gpt-4":       {Input: 0.03, Output: 0.06},
				"gpt-4o":      {Input: 0.0025, Output: 0.01},
				"gpt-4o-mini": {Input: 0.00015, Output: 0.0006},
			},
		},
		WebUI: WebUI{Listen: "localhost", Port: 8000, AuthExpiration: "1w"},
		NATS:  NATS{URL: "nats://localhost:4222", StoreDir: "./nats-data"},
		Log:   Log{Level: "info", Format: "text"},
	}
}

const keyDelim = "::"

// Load reads configuration files (missing files are skipped) and environment.
func Load(files ...string) (*Config, error) {
	if len(files) == 0 {
		files = []string{"settings.toml", ".secrets.toml"}
	}

	// Keys are joined with "::" instead of "." so model names like
	// "gpt-4.1" can be used as keys.
	k := koanf.New(keyDelim)

	for _, f := range files {
		if _, err := os.Stat(f); errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err := k.Load(file.Provider(f), toml.Parser()); err != nil {
			return nil, fmt.Errorf("load %s: %w", f, err)
		}
	}

	envProvider := env.Provider(keyDelim, env.Opt{
		Prefix: "UGUBOT_",
		TransformFunc: func(k, v string) (string, any) {
			k = strings.ToLower(strings.TrimPrefix(k, "UGUBOT_"))
			return strings.ReplaceAll(k, "__", keyDelim), v
		},
	})
	if err := k.Load(envProvider, nil); err != nil {
		return nil, fmt.Errorf("load env: %w", err)
	}

	cfg := defaults()
	defaultPrices := cfg.OpenAI.Prices
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf"}); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	for model, price := range defaultPrices {
		if _, ok := cfg.OpenAI.Prices[model]; !ok {
			cfg.OpenAI.Prices[model] = price
		}
	}

	return &cfg, nil
}

// DatabaseURL returns the postgres connection string.
func (c *Config) DatabaseURL() (string, error) {
	d := c.Database
	if d.URL != "" {
		return d.URL, nil
	}
	if d.Provider != "" && d.Provider != "postgres" {
		return "", fmt.Errorf("unsupported database provider %q: only postgres is supported", d.Provider)
	}
	if d.Database == "" {
		return "", errors.New("database is not configured: set database.url")
	}

	host := d.Host
	if host == "" {
		host = "localhost"
	}
	port := d.Port
	if port == 0 {
		port = 5432
	}

	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(d.User, d.Password),
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   d.Database,
	}
	return u.String(), nil
}
