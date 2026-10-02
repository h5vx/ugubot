package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "settings.toml")
	err := os.WriteFile(file, []byte(`
admin_jids = ["admin@example.com"]

[xmpp]
jid = "bot@example.com"

[xmpp.rooms.test]
join = true
jid = "test@conference.example.com"
nick = "bot"

[openai.prices."gpt-4.1"]
input = 0.002
output = 0.008

[openai.prompt.dan]
command = "dan"
text = "hi"

[database]
user = "u"
password = "p@ss"
database = "ugubot"

[logging.formatters.default]
format = "ignored, left from the Python version"
`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("UGUBOT_XMPP__PASSWORD", "from-env")
	t.Setenv("UGUBOT_WEBUI__PORT", "9000")
	t.Setenv("UGUBOT_WEBUI__PASSWORDS_SHA512", `["aa", "bb"]`)

	cfg, err := Load(file)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.XMPP.Password != "from-env" || cfg.WebUI.Port != 9000 {
		t.Errorf("env overrides: password %q, port %d", cfg.XMPP.Password, cfg.WebUI.Port)
	}
	if p := cfg.WebUI.PasswordsSHA512; len(p) != 2 || p[1] != "bb" {
		t.Errorf("list from env: %q", p)
	}
	if r := cfg.XMPP.Rooms["test"]; !r.Join || r.JID != "test@conference.example.com" {
		t.Errorf("room: %+v", r)
	}
	if p := cfg.OpenAI.Prices["gpt-4.1"]; p.Output != 0.008 {
		t.Errorf("price with a dot in the model name: %+v (all: %+v)", p, cfg.OpenAI.Prices)
	}
	if _, ok := cfg.OpenAI.Prices["gpt-4o-mini"]; !ok {
		t.Error("default prices lost")
	}
	if cfg.OpenAI.Prompt["dan"].Command != "dan" || cfg.OpenAI.CommandPrefix != "~" {
		t.Errorf("openai: %+v", cfg.OpenAI)
	}
	url, err := cfg.DatabaseURL()
	if err != nil || url != "postgres://u:p%40ss@localhost:5432/ugubot" {
		t.Errorf("database url: %s, %v", url, err)
	}
}
