package ai

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"

	"github.com/h5vx/ugubot/internal/bus"
	"github.com/h5vx/ugubot/internal/config"
)

// DropIfDisabled drops everything when AI is disabled in settings.
type DropIfDisabled struct {
	base
	Enabled bool
}

func (m *DropIfDisabled) Incoming(_ context.Context, _ *Incoming) (*Outgoing, bool) {
	return nil, !m.Enabled
}

// StripText trims whitespace around the message.
type StripText struct{ base }

func (StripText) Incoming(_ context.Context, in *Incoming) (*Outgoing, bool) {
	in.Text = strings.TrimSpace(in.Text)
	return nil, false
}

// DropIfNotAddressed drops room messages that don't start with the bot nick
// and cuts the nick (and the separator after it) from the ones that do.
type DropIfNotAddressed struct {
	base
	BotNick string
}

func (m *DropIfNotAddressed) Incoming(_ context.Context, in *Incoming) (*Outgoing, bool) {
	if !in.Chat.IsMUC || in.Kind != bus.KindUser {
		return nil, false
	}
	rest, ok := cutAddress(in.Text, m.BotNick)
	if !ok {
		return nil, true
	}
	in.Text = rest
	return nil, false
}

// cutAddress strips "nick: ", "nick, " or "nick " from the start of text.
func cutAddress(text, nick string) (string, bool) {
	rest, ok := strings.CutPrefix(text, nick)
	if !ok {
		return text, false
	}
	if rest != "" {
		rest = rest[1:] // separator: ':' ',' or space, as in the Python version
	}
	return strings.TrimSpace(rest), true
}

// ParseCommands moves leading "~command" words into in.Commands.
type ParseCommands struct {
	base
	Prefix string
}

func (m *ParseCommands) Incoming(_ context.Context, in *Incoming) (*Outgoing, bool) {
	in.Text, in.Commands = parseCommands(in.Text, m.Prefix, in.Commands)
	if len(in.Commands) > 0 {
		slog.Debug("parsed commands", "commands", in.Commands)
	}
	return nil, false
}

func parseCommands(text, prefix string, commands []string) (string, []string) {
	if prefix == "" {
		return text, commands
	}
	for strings.HasPrefix(text, prefix) {
		command, rest, _ := strings.Cut(text, " ")
		text = rest
		command = strings.TrimPrefix(command, prefix)
		if command != "" && !slices.Contains(commands, command) {
			commands = append(commands, command)
		}
	}
	return text, commands
}

// SwitchModel uses the secondary model when its command is given (~4).
type SwitchModel struct {
	base
	Command string
	Model   string
}

func (m *SwitchModel) Incoming(_ context.Context, in *Incoming) (*Outgoing, bool) {
	if m.Command != "" && m.Model != "" && in.HasCommand(m.Command) {
		in.Model = m.Model
	}
	return nil, false
}

// Temperature handles "~t <temperature> text".
type Temperature struct{ base }

func (Temperature) Incoming(_ context.Context, in *Incoming) (*Outgoing, bool) {
	if !in.HasCommand("t") {
		return nil, false
	}

	arg, rest, _ := strings.Cut(in.Text, " ")
	in.Text = rest

	if len(arg) > 5 {
		return in.Reply("Error: your temperature is too precise"), false
	}
	t, err := strconv.ParseFloat(arg, 64)
	if err != nil {
		return in.Reply(fmt.Sprintf(
			"Error: ~t command expects numeric argument; you pass '%s', which is not a valid number. "+
				"Specify temperature as number, like this: ~t 0.5", arg)), false
	}
	if t < 0 || t > 2 {
		return in.Reply("Error: Temperature must be in range 0 - 2"), false
	}
	in.Temperature = &t
	return nil, false
}

// UserPrompts prepends configured prompts (e.g. ~dan) to the message.
type UserPrompts struct {
	base
	Prompts map[string]config.Prompt
}

func (m *UserPrompts) Incoming(_ context.Context, in *Incoming) (*Outgoing, bool) {
	for _, p := range m.Prompts {
		if p.Command != "" && in.HasCommand(p.Command) {
			in.Text = strings.TrimSpace(p.Text) + " " + in.Text
		}
	}
	return nil, false
}

const helpText = `Команды начинаются с символа ~. У бота есть следующие команды:
~dan <text> - сгенерировать ответ используя промпт BetterDAN
~clear - очистить контекст текущего чата
~prelude <text> - установить "прелюдию" для текущего чата. Прелюдия будет постоянно присутствовать в начале контекста. ~prelude без текста удаляет её
~context - показать текущее содержимое контекста
~help - показать эту справку
~4 - использовать альтернативную модель (дороже)
~t <temp> - задать температуру для запроса (temp - число от 0.0 до 2.0)
~$ - посчитать деньги потраченные на этот запрос
~usage [global] [days] - показать статистику использования. Примеры:
  - ~usage 7 - статистика использования в этом чате за неделю
  - ~usage global - статистика использования по всем чатам за месяц

Некоторые команды можно комбинировать, например:
  - ~clear ~dan <text> - очистит контекст и сгенерирует ответ с помощью DAN
  - ~dan ~prelude <text> - установит прелюдию с промптом DAN
Команды применяются по порядку, поэтому порядок комбинирования важен.

Служебные команды:
~block / ~unblock <jid or nick>
~blocklist
`

type Help struct{ base }

func (Help) Incoming(_ context.Context, in *Incoming) (*Outgoing, bool) {
	if in.HasCommand("help") {
		return in.Reply(helpText), false
	}
	return nil, false
}
