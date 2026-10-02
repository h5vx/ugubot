package history

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/h5vx/ugubot/internal/bus"
	"github.com/h5vx/ugubot/internal/pg"
)

// newTestDB creates an empty database on the server from UGUBOT_TEST_DATABASE_URL.
func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv("UGUBOT_TEST_DATABASE_URL")
	if base == "" {
		t.Skip("UGUBOT_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()

	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("ugubot_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatal(err)
	}

	u, _ := url.Parse(base)
	u.Path = "/" + name
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(ctx, "DROP DATABASE "+name)
		admin.Close(ctx)
	})
	return pool
}

// Tables as created by Pony ORM in the Python version.
const legacySchema = `
CREATE TABLE chat (id serial PRIMARY KEY, jid text UNIQUE NOT NULL, name text NOT NULL, is_muc boolean NOT NULL);
CREATE TABLE message (id serial PRIMARY KEY, chat integer NOT NULL REFERENCES chat, utctime timestamp NOT NULL,
                      msg_type integer NOT NULL, nick text NOT NULL, text text NOT NULL, outgoing boolean NOT NULL);
CREATE TABLE nickcolor (id serial PRIMARY KEY, nick text UNIQUE NOT NULL, color text NOT NULL);
INSERT INTO chat (id, jid, name, is_muc) VALUES (1, 'room@conf.example.com', 'room@conf.example.com', true),
                                                (7, 'alice@example.com', 'alice', false);
INSERT INTO message (id, chat, utctime, msg_type, nick, text, outgoing) VALUES
  (1, 1, '2026-10-01 20:30:00', 1, 'alice', 'bot: hi', false),
  (2, 1, '2026-10-01 20:31:00', 1, 'bot', 'hello', true),
  (3, 1, '2026-10-02 09:00:00', 4, 'eve', '', false),
  (40, 7, '2026-10-02 09:00:00', 1, 'alice', 'private', false);
INSERT INTO nickcolor (nick, color) VALUES ('alice', '#ff0000');
`

func TestLegacyImportAndQueries(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, legacySchema); err != nil {
		t.Fatal(err)
	}
	if err := pg.Migrate(ctx, pool, pg.MustSub(migrations, "migrations"), "goose_version_chat"); err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)

	chats, err := s.Chats(ctx)
	if err != nil || len(chats) != 2 || chats[0].ID != 1 || !chats[0].IsMUC || chats[1].ID != 7 {
		t.Fatalf("chats: %+v, %v", chats, err)
	}

	// 20:30 UTC is 23:30 in Moscow (+3), still Oct 1.
	dates, err := s.Dates(ctx, 1, "Europe/Moscow")
	if err != nil || !slices.Equal(dates, []string{"2026-10-01", "2026-10-02"}) {
		t.Fatalf("moscow dates: %v, %v", dates, err)
	}
	dates, _ = s.Dates(ctx, 1, "UTC")
	if !slices.Equal(dates, []string{"2026-10-01", "2026-10-02"}) {
		t.Fatalf("utc dates: %v", dates)
	}
	dates, _ = s.Dates(ctx, 1, "Asia/Tokyo") // +9: 05:30, 05:31 and 18:00 on Oct 2
	if !slices.Equal(dates, []string{"2026-10-02"}) {
		t.Fatalf("tokyo dates: %v", dates)
	}
	if _, err := s.Dates(ctx, 1, "Mars/Olympus"); err == nil {
		t.Fatal("bad timezone accepted")
	}

	msgs, err := s.Messages(ctx, 1, "2026-10-02", "Asia/Tokyo")
	if err != nil || len(msgs) != 3 || msgs[0].Text != "bot: hi" || msgs[2].Kind != bus.KindJoin {
		t.Fatalf("tokyo messages: %+v, %v", msgs, err)
	}
	msgs, _ = s.Messages(ctx, 1, "2026-10-01", "UTC")
	if len(msgs) != 2 || !msgs[1].Outgoing || msgs[1].Time.UTC().Format(time.RFC3339) != "2026-10-01T20:31:00Z" {
		t.Fatalf("utc messages: %+v", msgs)
	}

	colors, _ := s.NickColors(ctx)
	if len(colors) != 1 || colors[0].Color != "#ff0000" {
		t.Fatalf("colors: %+v", colors)
	}

	// New rows continue after imported ids.
	msg, err := s.SaveEvent(ctx, &bus.Event{
		ID: "6f1c1e0a-9b7e-4d55-8f39-2f5a1f0d3c11", Time: time.Now(), Kind: bus.KindUser,
		Chat: bus.ChatRef{JID: "bob@example.com", Name: "bob"}, Nick: "bob", Text: "new",
	})
	if err != nil || msg.ID != 41 || msg.ChatID != 8 || msg.Chat.Name != "bob" {
		t.Fatalf("new message: %+v, %v", msg, err)
	}
}

func TestSaveEventIsIdempotent(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	if err := pg.Migrate(ctx, pool, pg.MustSub(migrations, "migrations"), "goose_version_chat"); err != nil {
		t.Fatal(err)
	}
	s := NewStore(pool)

	ev := &bus.Event{
		ID: "0b6c3a52-1c2d-4e4f-9a8b-7c6d5e4f3a2b", Time: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC),
		Kind: bus.KindTopic, Chat: bus.ChatRef{JID: "r@c.example.com", IsMUC: true}, Nick: "mod", Text: "topic",
	}
	first, err := s.SaveEvent(ctx, ev)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SaveEvent(ctx, ev)
	if err != nil || second.ID != first.ID {
		t.Fatalf("duplicate stored: %d vs %d, %v", first.ID, second.ID, err)
	}

	for i := range 5 {
		_, err := s.SaveEvent(ctx, &bus.Event{
			Time: ev.Time.Add(time.Duration(i+1) * time.Minute), Kind: bus.KindUser,
			Chat: ev.Chat, Nick: "u", Text: fmt.Sprint(i),
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	last, err := s.LastN(ctx, first.ChatID, 3, []bus.Kind{bus.KindUser})
	if err != nil || len(last) != 3 || last[0].Text != "2" || last[2].Text != "4" {
		t.Fatalf("last n: %+v, %v", last, err)
	}
}
