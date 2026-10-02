package ai

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/h5vx/ugubot/internal/config"
)

// Prices calculates request cost by model name prefix.
type Prices map[string]config.Price

func (p Prices) For(model string) config.Price {
	best := ""
	for prefix := range p {
		if strings.HasPrefix(model, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	return p[best]
}

func (p Prices) Cost(model string, promptTokens, completionTokens int) (in, out float64) {
	price := p.For(model)
	return float64(promptTokens) / 1000 * price.Input, float64(completionTokens) / 1000 * price.Output
}

func money(v float64) string {
	if v > 0 && v < 0.01 {
		return fmt.Sprintf("$%.4f", v)
	}
	return fmt.Sprintf("$%.2f", v)
}

// UsageReport answers "~usage [global] [days]".
type UsageReport struct {
	base
	Store  Store
	Prices Prices
	Now    func() time.Time
}

func (m *UsageReport) Incoming(ctx context.Context, in *Incoming) (*Outgoing, bool) {
	if !in.HasCommand("usage") {
		return nil, false
	}

	global := false
	days := 30
	for _, part := range strings.Fields(in.Text) {
		if n, err := strconv.Atoi(part); err == nil {
			days = min(max(n, 1), 30)
		} else if strings.EqualFold(part, "global") {
			global = true
		}
	}

	report, err := m.report(ctx, in.ChatID, global, days)
	if err != nil {
		return in.Reply("Error: " + err.Error()), false
	}
	return in.Reply(report), false
}

type usageRow struct {
	name           string
	in, out, total float64
}

func (m *UsageReport) report(ctx context.Context, chatID int64, global bool, days int) (string, error) {
	now := time.Now
	if m.Now != nil {
		now = m.Now
	}

	filterChat := chatID
	if global {
		filterChat = 0
	}
	records, err := m.Store.UsageSince(ctx, now().AddDate(0, 0, -days), filterChat)
	if err != nil {
		return "", err
	}

	rows := map[string]*usageRow{}
	total := usageRow{name: "TOTAL"}
	for _, r := range records {
		key := r.Nick
		if global {
			key = r.ChatName
		}
		row := rows[key]
		if row == nil {
			row = &usageRow{name: key}
			rows[key] = row
		}
		in, out := m.Prices.Cost(r.Model, r.PromptTokens, r.CompletionTokens)
		row.in += in
		row.out += out
		row.total += in + out
		total.in += in
		total.out += out
		total.total += in + out
	}

	sorted := make([]*usageRow, 0, len(rows))
	for _, r := range rows {
		sorted = append(sorted, r)
	}
	slices.SortFunc(sorted, func(a, b *usageRow) int { return cmp.Compare(b.total, a.total) })

	daysText := "сегодня"
	if days != 1 {
		daysText = fmt.Sprintf("последние %d %s", days, Pluralize(days, "день", "дней", "дня"))
	}

	var title, who string
	if global {
		title, who = "Статистика использования по всем чатам за "+daysText+":", "Chat"
	} else {
		title, who = "Статистика использования в этом чате за "+daysText+":", "User"
	}

	table := [][]string{{"#", who, "Total", "In", "Out"}}
	for i, r := range sorted {
		table = append(table, []string{strconv.Itoa(i + 1), r.name, money(r.total), money(r.in), money(r.out)})
	}
	table = append(table, []string{"-", total.name, money(total.total), money(total.in), money(total.out)})

	return title + "\n" + formatTable(table), nil
}

// formatTable renders rows as " : "-separated columns with a rule after the
// header and before the total row.
func formatTable(table [][]string) string {
	const maxWidth = 30

	widths := make([]int, len(table[0]))
	for _, row := range table {
		for i, col := range row {
			widths[i] = min(max(widths[i], utf8.RuneCountInString(col)), maxWidth)
		}
	}

	rule := make([]string, len(widths))
	for i, w := range widths {
		rule[i] = strings.Repeat("-", w)
	}
	hline := strings.Join(rule, "-:-")

	var lines []string
	for i, row := range table {
		if i == len(table)-1 {
			lines = append(lines, hline)
		}
		cols := make([]string, len(row))
		for j, col := range row {
			col = shorten(col, maxWidth)
			cols[j] = col + strings.Repeat(" ", widths[j]-utf8.RuneCountInString(col))
		}
		lines = append(lines, strings.Join(cols, " : "))
		if i == 0 {
			lines = append(lines, hline)
		}
	}
	return strings.Join(lines, "\n")
}

func shorten(s string, width int) string {
	if utf8.RuneCountInString(s) <= width {
		return s
	}
	r := []rune(s)
	return string(r[:width-1]) + "…"
}

// InlineUsage prepends the request cost to the answer when ~$ is given.
type InlineUsage struct {
	base
	Prices Prices
}

func (m *InlineUsage) Outgoing(_ context.Context, out *Outgoing) bool {
	if !out.HasCommand("$") || out.Model == "" || out.Usage == nil {
		return false
	}
	in, o := m.Prices.Cost(out.Model, out.Usage.PromptTokens, out.Usage.CompletionTokens)
	out.Text = fmt.Sprintf("[%s (IN %s / OUT %s, %s)] %s", money(in+o), money(in), money(o), out.Model, out.Text)
	return false
}
