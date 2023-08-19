import typing as t
from collections import defaultdict
from dataclasses import dataclass
from textwrap import shorten

from ai.types import OutgoingMessage
from db import get_usage_for_last_n_days
from util.plurals import pluralize

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware

GPT_4_PRICE_PER_1K_INPUT_TOKENS = 0.03
GPT_4_PRICE_PER_1K_OUTPUT_TOKENS = 0.06
GPT_3_5_PRICE_PER_1K_INPUT_TOKENS = 0.0015
GPT_3_5_PRICE_PER_1K_OUTPUT_TOKENS = 0.002


@dataclass
class UsageStats:
    # In tokens
    gpt4_input: int = 0
    gpt4_output: int = 0
    gpt35_input: int = 0
    gpt35_output: int = 0
    total: int = 0

    # In money
    gpt4_input_money: float = 0.0
    gpt4_output_money: float = 0.0
    gpt35_input_money: float = 0.0
    gpt35_output_money: float = 0.0
    total_money: float = 0.0

    def convert_tokens_to_money(self):
        self.gpt4_input_money = self.gpt4_input / 1000 * GPT_4_PRICE_PER_1K_INPUT_TOKENS
        self.gpt4_output_money = self.gpt4_output / 1000 * GPT_4_PRICE_PER_1K_OUTPUT_TOKENS
        self.gpt35_input_money = self.gpt35_input / 1000 * GPT_3_5_PRICE_PER_1K_INPUT_TOKENS
        self.gpt35_output_money = self.gpt35_output / 1000 * GPT_3_5_PRICE_PER_1K_OUTPUT_TOKENS
        self.total_money = sum(
            self.gpt4_input_money, self.gpt4_output_money, self.gpt35_input_money, self.gpt35_output_money
        )


class UsageCommandMiddleware(AIBotMiddleware):
    """
    Report money usage for each chat or user
    """

    command = "usage"

    def _make_report(self, global_: bool, n_days=30) -> str:
        result = []
        table = []
        table_col_width = defaultdict(int)

        chat_usage = defaultdict(UsageStats)
        user_usage = defaultdict(UsageStats)
        total_usage = UsageStats()

        def row_output(*args):
            row = []

            for i, col in enumerate(args):
                row.append(col)
                table_col_width[i] = max(table_col_width[i], len(col))

            table.append(row)

        def result_hline():
            result.append(":".join(("-" * min(30, table_col_width[i]) for i in range(len(table_col_width)))))

        days_plural = pluralize(n_days, "день", "дней", "дня")
        days_text = "сегодня" if n_days == 1 else f"последние {n_days} {days_plural}"

        if global_:
            result.append(f"Статистика использования по всем чатам за {days_text}:")
            row_output("#", "Chat", "Total", "4 in", "4 out", "3.5 in", "3.5 out")
        else:
            result.append(f"Статистика использования в этом чате за {days_text}:")
            row_output("#", "User", "Total", "4 in", "4 out", "3.5 in", "3.5 out")

        # Count in tokens
        for msg, usage in get_usage_for_last_n_days(n_days):
            if global_:
                stat = chat_usage[msg.chat.name]
            else:
                stat = user_usage[msg.nick]

            if usage.model.startswith("gpt-3.5"):
                stat.gpt35_input += usage.prompt_tokens
                stat.gpt35_output += usage.completion_tokens
                total_usage.gpt35_input += usage.prompt_tokens
                total_usage.gpt35_output += usage.completion_tokens
            elif usage.model.startswith("gpt-4"):
                stat.gpt4_input += usage.prompt_tokens
                stat.gpt4_output += usage.completion_tokens
                total_usage.gpt4_input += usage.prompt_tokens
                total_usage.gpt4_output += usage.completion_tokens

            stat.total += usage.prompt_tokens + usage.completion_tokens
            total_usage.total += stat.total

        # Count in money
        stats = chat_usage if global_ else user_usage

        for _, stat in stats.items():
            stat.convert_tokens_to_money()

        # Sort by total money DESC
        stats = sorted(stats.items(), key=lambda v: v[1].total_money, reverse=True)

        # Append to table
        for n, stat in enumerate(stats, 1):
            user_or_chat, stat = stat

            total, g4i, g4o, g3i, g3o = map(
                "${:.2f}".format,
                (
                    stat.total_money,
                    stat.gpt4_input_money,
                    stat.gpt4_output_money,
                    stat.gpt35_input_money,
                    stat.gpt35_output_money,
                ),
            )

            row_output(n, user_or_chat, total, g4i, g4o, g3i, g3o)

        # Append total
        total_usage.convert_tokens_to_money()
        total, g4i, g4o, g3i, g3o = map(
            "${:.2f}".format,
            (
                total_usage.total_money,
                total_usage.gpt4_input_money,
                total_usage.gpt4_output_money,
                total_usage.gpt35_input_money,
                total_usage.gpt35_output_money,
            ),
        )

        row_output("-", "TOTAL", total, g4i, g4o, g3i, g3o)

        # Format the table
        result_hline()  # Line after header
        for row in table:
            if row[0] == "-":  # Line before total
                result_hline()

            row = (shorten(row[i], 30, placeholder="…").ljust(min(30, table_col_width[i])) for i in range(len(row)))
            result.append(" : ".join(row))

        return "\n".join(result)

    def incoming(self, message: IncomingMessage) -> t.Optional[t.Union[IncomingMessage, OutgoingMessage]]:
        if self.command not in message.commands:
            return message

        global_ = False
        n_days = 30

        for msg_part in message.text.split(" ", maxsplit=2):
            if msg_part.isdigit():
                n_days = max(1, min(30, int(msg_part)))
            elif msg_part.lower() == "global":
                global_ = True

        return OutgoingMessage(
            chat_id=message.chat_id,
            reply_for=message.database_id,
            text=self._make_report(global_, n_days),
        )


class UsageInlineCommandMiddleware(AIBotMiddleware):
    """
    Report money usage right in response
    """

    command = "$"

    def outgoing(self, message: OutgoingMessage) -> t.Optional[OutgoingMessage]:
        if self.command not in message.commands:
            return message

        if not message.model or not message.usage:
            return message

        if message.model.startswith("gpt-4"):
            prompt_money = message.usage.prompt_tokens / 1000 * GPT_4_PRICE_PER_1K_INPUT_TOKENS
            reply_money = message.usage.reply_tokens / 1000 * GPT_4_PRICE_PER_1K_OUTPUT_TOKENS
        else:
            prompt_money = message.usage.prompt_tokens / 1000 * GPT_3_5_PRICE_PER_1K_INPUT_TOKENS
            reply_money = message.usage.reply_tokens / 1000 * GPT_3_5_PRICE_PER_1K_OUTPUT_TOKENS

        total_money = prompt_money + reply_money
        total, prompt, reply = map("${:.2f}".format, (total_money, prompt_money, reply_money))

        message.text = f"[{total} (IN {prompt} / OUT {reply}, {message.model})] " + message.text

        return message
