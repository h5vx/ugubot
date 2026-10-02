package ai

import (
	"context"
	"embed"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Store keeps ai-worker's own data: usage, blocklist and preludes.
type Store interface {
	AddUsage(ctx context.Context, u UsageRecord) error
	SetCompletionMessage(ctx context.Context, promptID, completionID int64) error
	UsageSince(ctx context.Context, since time.Time, chatID int64) ([]UsageRecord, error)

	IsBlocked(ctx context.Context, jidOrNick string) (bool, error)
	Block(ctx context.Context, jidOrNick string) error
	Unblock(ctx context.Context, jidOrNick string) (bool, error)
	Blocklist(ctx context.Context) ([]string, error)

	Preludes(ctx context.Context) (map[int64]string, error)
	SetPrelude(ctx context.Context, chatID int64, text string) error
}

type UsageRecord struct {
	Time             time.Time
	ChatID           int64
	ChatName         string
	Nick             string
	Model            string
	PromptMessageID  int64
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type pgStore struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) Store {
	return &pgStore{db: db}
}

func (s *pgStore) AddUsage(ctx context.Context, u UsageRecord) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO ai.usage (ts, chat_id, chat_name, nick, model, prompt_message_id,
		                      prompt_tokens, completion_tokens, total_tokens)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		u.Time, u.ChatID, u.ChatName, u.Nick, u.Model, u.PromptMessageID,
		u.PromptTokens, u.CompletionTokens, u.TotalTokens,
	)
	return err
}

func (s *pgStore) SetCompletionMessage(ctx context.Context, promptID, completionID int64) error {
	_, err := s.db.Exec(ctx, `
		UPDATE ai.usage SET completion_message_id = $2
		WHERE prompt_message_id = $1 AND completion_message_id IS NULL`,
		promptID, completionID,
	)
	return err
}

// UsageSince returns usage after since; chatID 0 means all chats.
func (s *pgStore) UsageSince(ctx context.Context, since time.Time, chatID int64) ([]UsageRecord, error) {
	rows, err := s.db.Query(ctx, `
		SELECT ts, chat_id, chat_name, nick, model, coalesce(prompt_message_id, 0),
		       prompt_tokens, completion_tokens, total_tokens
		FROM ai.usage
		WHERE ts >= $1 AND ($2 = 0 OR chat_id = $2)`,
		since, chatID,
	)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (UsageRecord, error) {
		var u UsageRecord
		err := row.Scan(&u.Time, &u.ChatID, &u.ChatName, &u.Nick, &u.Model, &u.PromptMessageID,
			&u.PromptTokens, &u.CompletionTokens, &u.TotalTokens)
		return u, err
	})
}

func (s *pgStore) IsBlocked(ctx context.Context, jidOrNick string) (bool, error) {
	var blocked bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM ai.blocklist WHERE jid_or_nick = $1)`, jidOrNick).Scan(&blocked)
	return blocked, err
}

func (s *pgStore) Block(ctx context.Context, jidOrNick string) error {
	_, err := s.db.Exec(ctx, `INSERT INTO ai.blocklist (jid_or_nick) VALUES ($1) ON CONFLICT DO NOTHING`, jidOrNick)
	return err
}

func (s *pgStore) Unblock(ctx context.Context, jidOrNick string) (bool, error) {
	tag, err := s.db.Exec(ctx, `DELETE FROM ai.blocklist WHERE jid_or_nick = $1`, jidOrNick)
	return tag.RowsAffected() > 0, err
}

func (s *pgStore) Blocklist(ctx context.Context) ([]string, error) {
	rows, err := s.db.Query(ctx, `SELECT jid_or_nick FROM ai.blocklist ORDER BY jid_or_nick`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

func (s *pgStore) Preludes(ctx context.Context) (map[int64]string, error) {
	rows, err := s.db.Query(ctx, `SELECT chat_id, text FROM ai.preludes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := map[int64]string{}
	for rows.Next() {
		var id int64
		var text string
		if err := rows.Scan(&id, &text); err != nil {
			return nil, err
		}
		result[id] = text
	}
	return result, rows.Err()
}

func (s *pgStore) SetPrelude(ctx context.Context, chatID int64, text string) error {
	if text == "" {
		_, err := s.db.Exec(ctx, `DELETE FROM ai.preludes WHERE chat_id = $1`, chatID)
		return err
	}
	_, err := s.db.Exec(ctx, `
		INSERT INTO ai.preludes (chat_id, text) VALUES ($1, $2)
		ON CONFLICT (chat_id) DO UPDATE SET text = excluded.text`,
		chatID, text,
	)
	return err
}
