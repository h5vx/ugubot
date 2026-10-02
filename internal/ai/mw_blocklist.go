package ai

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
)

// Blocklist drops messages from blocked users and handles ~block, ~unblock
// and ~blocklist. Rooms are checked by nick, private chats by JID.
type Blocklist struct {
	base
	Store  Store
	Admins []string
}

func (m *Blocklist) Incoming(ctx context.Context, in *Incoming) (*Outgoing, bool) {
	who := in.Chat.JID
	if in.Chat.IsMUC {
		who = in.Nick
	}

	blocked, err := m.Store.IsBlocked(ctx, who)
	if err != nil {
		slog.Error("check blocklist", "err", err)
	}
	if blocked {
		slog.Info("message dropped: user is blocked", "who", who)
		return nil, true
	}

	switch {
	case in.HasCommand("block"):
		return m.guard(in, func() string {
			target := strings.TrimSpace(in.Text)
			if err := m.Store.Block(ctx, target); err != nil {
				return "Error: " + err.Error()
			}
			return target + " is added to blocklist"
		}), false
	case in.HasCommand("unblock"):
		return m.guard(in, func() string {
			target := strings.TrimSpace(in.Text)
			removed, err := m.Store.Unblock(ctx, target)
			if err != nil {
				return "Error: " + err.Error()
			}
			if !removed {
				return target + " is not found in blocklist"
			}
			return target + " is removed from blocklist"
		}), false
	case in.HasCommand("blocklist"):
		return m.guard(in, func() string {
			users, err := m.Store.Blocklist(ctx)
			if err != nil {
				return "Error: " + err.Error()
			}
			if len(users) == 0 {
				return "Happy news: no one blocked. There is 0 blocked users"
			}
			var b strings.Builder
			fmt.Fprintf(&b, "There is %d blocked %s:", len(users), Pluralize(len(users), "user", "users", "users"))
			for i, u := range users {
				fmt.Fprintf(&b, "\n%d. %s", i+1, u)
			}
			return b.String()
		}), false
	}
	return nil, false
}

// guard runs an admin command only in a private chat with an admin.
func (m *Blocklist) guard(in *Incoming, run func() string) *Outgoing {
	if in.Chat.IsMUC {
		return in.Reply("This command works only in private conversation")
	}
	if !slices.Contains(m.Admins, in.Chat.JID) {
		return in.Reply("You don't have access to use this command")
	}
	return in.Reply(run())
}
