package history

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"time"
	_ "time/tzdata" // checkTimezone must work in minimal containers

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/h5vx/ugubot/internal/bus"
)

//go:embed migrations/*.sql
var migrations embed.FS

var ErrNotFound = errors.New("not found")

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// SaveEvent stores an event as a message. Saving the same event twice returns
// the message created the first time.
func (s *Store) SaveEvent(ctx context.Context, ev *bus.Event) (*bus.Message, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var chatID int64
	err = tx.QueryRow(ctx, `
		INSERT INTO chat.chats (jid, name, is_muc) VALUES ($1, $2, $3)
		ON CONFLICT (jid) DO UPDATE SET jid = excluded.jid
		RETURNING id`,
		ev.Chat.JID, chatName(ev.Chat), ev.Chat.IsMUC,
	).Scan(&chatID)
	if err != nil {
		return nil, fmt.Errorf("upsert chat: %w", err)
	}

	msg := &bus.Message{
		ChatID:   chatID,
		Time:     ev.Time,
		Kind:     ev.Kind,
		Nick:     ev.Nick,
		Text:     ev.Text,
		Outgoing: ev.Outgoing,
		Meta:     ev.Meta,
	}

	var eventID any
	if ev.ID != "" {
		eventID = ev.ID
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO chat.messages (chat_id, ts, kind, nick, text, outgoing, event_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (event_id) DO NOTHING
		RETURNING id`,
		chatID, ev.Time, ev.Kind.Code(), ev.Nick, ev.Text, ev.Outgoing, eventID,
	).Scan(&msg.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		// Redelivered event: return what was stored before.
		err = tx.QueryRow(ctx, `SELECT id, ts FROM chat.messages WHERE event_id = $1`, eventID).Scan(&msg.ID, &msg.Time)
	}
	if err != nil {
		return nil, fmt.Errorf("insert message: %w", err)
	}

	if err := tx.QueryRow(ctx, `SELECT jid, name, is_muc FROM chat.chats WHERE id = $1`, chatID).
		Scan(&msg.Chat.JID, &msg.Chat.Name, &msg.Chat.IsMUC); err != nil {
		return nil, err
	}

	return msg, tx.Commit(ctx)
}

func chatName(c bus.ChatRef) string {
	if c.Name != "" {
		return c.Name
	}
	return c.JID
}

func (s *Store) Chats(ctx context.Context) ([]bus.Chat, error) {
	rows, err := s.db.Query(ctx, `SELECT id, jid, name, is_muc FROM chat.chats ORDER BY is_muc DESC, lower(name)`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (bus.Chat, error) {
		var c bus.Chat
		err := row.Scan(&c.ID, &c.JID, &c.Name, &c.IsMUC)
		return c, err
	})
}

func (s *Store) Chat(ctx context.Context, id int64) (*bus.Chat, error) {
	var c bus.Chat
	err := s.db.QueryRow(ctx, `SELECT id, jid, name, is_muc FROM chat.chats WHERE id = $1`, id).
		Scan(&c.ID, &c.JID, &c.Name, &c.IsMUC)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &c, err
}

// Messages returns messages of one local day in the given timezone.
func (s *Store) Messages(ctx context.Context, chatID int64, date, tz string) ([]bus.Message, error) {
	if _, err := time.Parse(time.DateOnly, date); err != nil {
		return nil, fmt.Errorf("invalid date %q: expected YYYY-MM-DD", date)
	}
	if err := checkTimezone(tz); err != nil {
		return nil, err
	}

	chat, err := s.Chat(ctx, chatID)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, ts, kind, nick, text, outgoing
		FROM chat.messages
		WHERE chat_id = $1
		  AND ts >= ($2::date::timestamp AT TIME ZONE $3)
		  AND ts <  (($2::date + 1)::timestamp AT TIME ZONE $3)
		ORDER BY ts, id`,
		chatID, date, tz,
	)
	if err != nil {
		return nil, err
	}
	return collectMessages(rows, chat)
}

// Dates returns local dates (YYYY-MM-DD) that have messages in the chat.
//
// Instead of reading every message it walks the (chat_id, ts) index: from each
// found day it jumps straight to the first message after the next local
// midnight. The cost is one index seek per day with messages, regardless of
// how many messages there are, and any timezone works without caching.
func (s *Store) Dates(ctx context.Context, chatID int64, tz string) ([]string, error) {
	if err := checkTimezone(tz); err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, `
		WITH RECURSIVE d(day) AS (
			SELECT (min(ts) AT TIME ZONE $2)::date
			FROM chat.messages WHERE chat_id = $1
			UNION ALL
			SELECT (SELECT (m.ts AT TIME ZONE $2)::date
			        FROM chat.messages m
			        WHERE m.chat_id = $1
			          AND m.ts >= ((d.day + 1)::timestamp AT TIME ZONE $2)
			        ORDER BY m.ts
			        LIMIT 1)
			FROM d WHERE d.day IS NOT NULL
		)
		SELECT to_char(day, 'YYYY-MM-DD') FROM d WHERE day IS NOT NULL`,
		chatID, tz,
	)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

// LastN returns the last n messages of the given kinds in chronological order.
func (s *Store) LastN(ctx context.Context, chatID int64, n int, kinds []bus.Kind) ([]bus.Message, error) {
	chat, err := s.Chat(ctx, chatID)
	if err != nil {
		return nil, err
	}

	codes := make([]int16, 0, len(kinds))
	for _, k := range kinds {
		codes = append(codes, k.Code())
	}

	rows, err := s.db.Query(ctx, `
		SELECT * FROM (
			SELECT id, ts, kind, nick, text, outgoing
			FROM chat.messages
			WHERE chat_id = $1 AND (cardinality($3::smallint[]) = 0 OR kind = ANY($3))
			ORDER BY ts DESC, id DESC
			LIMIT $2
		) last ORDER BY ts, id`,
		chatID, n, codes,
	)
	if err != nil {
		return nil, err
	}
	return collectMessages(rows, chat)
}

func collectMessages(rows pgx.Rows, chat *bus.Chat) ([]bus.Message, error) {
	ref := bus.ChatRef{JID: chat.JID, Name: chat.Name, IsMUC: chat.IsMUC}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (bus.Message, error) {
		m := bus.Message{ChatID: chat.ID, Chat: ref}
		var kind int16
		err := row.Scan(&m.ID, &m.Time, &kind, &m.Nick, &m.Text, &m.Outgoing)
		m.Kind = bus.KindFromCode(kind)
		return m, err
	})
}

func (s *Store) NickColors(ctx context.Context) ([]bus.NickColor, error) {
	rows, err := s.db.Query(ctx, `SELECT nick, color FROM chat.nick_colors ORDER BY nick`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[bus.NickColor])
}

func (s *Store) SetNickColor(ctx context.Context, nick, color string) error {
	if color == "" {
		_, err := s.db.Exec(ctx, `DELETE FROM chat.nick_colors WHERE nick = $1`, nick)
		return err
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO chat.nick_colors (nick, color) VALUES ($1, $2)
		ON CONFLICT (nick) DO UPDATE SET color = excluded.color`,
		nick, color,
	)
	return err
}

func checkTimezone(tz string) error {
	if tz == "" {
		return errors.New("timezone is required")
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return fmt.Errorf("unknown timezone %q", tz)
	}
	return nil
}
