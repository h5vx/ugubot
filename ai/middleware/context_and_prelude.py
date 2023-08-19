import logging
import typing as t
from collections import defaultdict, deque
from dataclasses import dataclass
from textwrap import shorten

from ai.types import OutgoingMessage
from config import settings
from db import Chat, Message, db_session, get_last_n_messages_for_ai
from util.plurals import pluralize
from util.token_counter import count_tokens_for_message, get_encoder_for_model

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware

logger = logging.getLogger(__name__)


class ContextWithPreludeMiddleware(AIBotMiddleware):
    """
    Provides separate chat context for each chat
    """

    bot_nick = settings.openai.user_nick
    default_model = settings.openai.model
    max_tokens = settings.openai.max_tokens - settings.openai.tokens_reserved_for_response

    command_change_prelude = "prelude"
    command_show_context = "context"
    command_clear_context = "clear"

    @dataclass
    class ContextItem:
        model: str
        tokens: int
        message: t.Mapping[str, str]  # {"role": "...", "content": "..."}

    def __init__(self) -> None:
        super().__init__()

        self._context: t.Dict[int, t.Deque[self.ContextItem]] = {}  # {chat_id: [messages]}
        self._context_tokens: t.DefaultDict[int, int] = defaultdict(int)  # {chat_id: context_tokens_count}
        self._prelude: t.Dict[int, t.List[t.Dict[str, str]]] = {}  # {chat_id: prelude_messages}
        self._prelude_tokens: t.DefaultDict[int, int] = defaultdict(int)  # {chat_id: prelude_tokens_count}
        self._encoders: t.Dict[str, object] = {}  # {model_name: encoder}

        self._load_context_from_db()

    def incoming(self, message: IncomingMessage) -> t.Optional[t.Union[IncomingMessage, OutgoingMessage]]:
        if self.command_clear_context in message.commands:
            self._handle_command_clear_context(message)
        if self.command_change_prelude in message.commands:
            return self._handle_command_prelude(message)
        if self.command_show_context in message.commands:
            return self._handle_command_context(message)

        rotation_result = self._rotate_context(message)

        if isinstance(rotation_result, OutgoingMessage):
            return rotation_result

        if message.chat_id in self._prelude:
            message.full_with_context += self._prelude[message.chat_id]

        message.full_with_context += [ctx.message for ctx in self._context[message.chat_id]]

        return message

    def outgoing(self, message: OutgoingMessage) -> t.Optional[OutgoingMessage]:
        self._rotate_context(message)
        return message

    def _rotate_context(self, message: t.Union[IncomingMessage, OutgoingMessage]) -> t.Optional[OutgoingMessage]:
        """
        Put new item in context and remove oldest items if max tokens exceeded
        """
        is_outgoing = isinstance(message, OutgoingMessage)

        if is_outgoing and not message.model:
            # If model is None, this should be direct answer without completion, so we'll not store it in context
            return

        role = "assistant" if is_outgoing else "user"
        encoder = self._get_encoder(message.model)
        message_data = {"role": role, "content": message.text}
        message_tokens = (
            message.usage.reply_tokens if is_outgoing else count_tokens_for_message(encoder, [message_data])
        )
        prelude_tokens = self._prelude_tokens[message.chat_id]

        if not is_outgoing and prelude_tokens + message_tokens > self.max_tokens:
            tokens_plural = pluralize(message_tokens, "токен", "токенов", "токена")
            return OutgoingMessage(
                chat_id=message.chat_id,
                reply_for=message.database_id,
                text=f"Сообщение слишком большое ({message_tokens} {tokens_plural})",
            )

        if message.chat_id not in self._context:
            self._context[message.chat_id] = deque()

        self._context.append(
            self.ContextItem(
                model=message.model,
                tokens=message_tokens,
                message=message_data,
            )
        )

        self._context_tokens[message.chat_id] += message_tokens

        while self._context_tokens[message.chat_id] + prelude_tokens > self.max_tokens:
            self._remove_oldest_message_in_context(message.chat_id)

    def _handle_command_prelude(self, message: IncomingMessage) -> OutgoingMessage:
        encoder = self._get_encoder(message.model)
        tokens = count_tokens_for_message(encoder, [{"role": "user", "content": message.text}])
        tokens_plural = pluralize(tokens, "токен", "токенов", "токена")

        if tokens > self.max_tokens:
            report = (
                f"{message.sender_nick}: Слишком большая прелюдия: {tokens} {tokens_plural}"
                f" из максимально возможных {self.max_tokens}"
            )

            return OutgoingMessage(chat_id=message.chat_id, reply_for=message.database_id, text=report)

        new_prelude = [{"role": "user", "content": message.text}]

        self._prelude[message.chat_id] = new_prelude
        self._prelude_tokens[message.chat_id] = count_tokens_for_message(encoder, new_prelude)

        return OutgoingMessage(
            chat_id=message.chat_id,
            reply_for=message.database_id,
            text=f"{message.sender_nick}: Установлена новая прелюдия длиной в {tokens} {tokens_plural}",
        )

    def _handle_command_context(self, message: IncomingMessage) -> OutgoingMessage:
        result = []

        if message.chat_id not in self._context:
            return OutgoingMessage(chat_id=message.chat_id, reply_for=message.database_id, text="Контекст пуст")

        chat_context = self._context[message.chat_id]

        def ctx_item_to_result_string(n: int, ctx_item: self.ContextItem):
            shortened_msg = shorten(ctx_item.message["content"], 30, placeholder="…")
            result.append(f"{n}: {shortened_msg} ({ctx_item.tokens} tok)")

        if len(chat_context) > 6:
            for n, ctx_item in enumerate(chat_context[:3], 1):
                ctx_item_to_result_string(n, ctx_item)

            result.append(f"< ... {len(chat_context)- 6} пропущено ... >")

            for n, ctx_item in enumerate(chat_context[-3:], len(chat_context) - 2):
                ctx_item_to_result_string(n, ctx_item)
        else:
            for n, ctx_item in enumerate(chat_context, 1):
                ctx_item_to_result_string(n, ctx_item)

        total_token_count = self._context_tokens[message.chat_id]
        total_token_count_plural = pluralize(total_token_count, "токен", "токенов", "токена")
        prelude_token_count_plural = pluralize(self._prelude_tokens[message.chat_id], "токен", "токенов", "токена")

        result.append(
            (
                f"В контексте {total_token_count} {total_token_count_plural}, "
                f"для прелюдии используется {self._prelude_tokens[message.chat_id]} {prelude_token_count_plural}"
            )
        )

        result = "\n".join(result)

        return OutgoingMessage(
            chat_id=message.chat_id,
            reply_for=message.database_id,
            text=f"{message.sender_nick}: Текущее содержимое контекста:\n{result}",
        )

    def clear_context(self, chat_id: int):
        self._context[chat_id] = deque()
        self._context_tokens[chat_id] = 0

    def _handle_command_clear_context(self, message: IncomingMessage) -> IncomingMessage:
        self.clear_context(message.chat_id)
        return message

    def _get_encoder(self, model_name: str):
        if model_name in self._encoders:
            return self._encoders[model_name]

        logger.info(f"{self.__class__.__name__}: Loading encoder for {model_name}...")
        self._encoders[model_name] = get_encoder_for_model(model_name)
        logger.info(f"{self.__class__.__name__}: Encoder for {model_name} is loaded.")
        return self._encoders[model_name]

    def _db_message_to_ai_message(self, message: Message):
        role = "assistant" if message.outgoing else "user"
        nick = f"{message.nick}: " if message.nick != "[FOR AI]" else ""
        content = nick + message.text

        return {"role": role, "content": content}

    def _load_context_from_db(self):
        logger.info(f"{self.__class__.__name__}: Populating AI context from DB...")

        def get_message_model(msg: Message) -> str:
            msg_usage = None

            if msg.outgoing:
                msg_usage = msg.ai_usage.select().first()
            else:
                msg_usage = msg.user_usage.select().first()

            return msg_usage.model.name if msg_usage else self.default_model

        with db_session:
            chats = Chat.select()

            for chat in chats:
                tokens = self._prelude_tokens[chat.id]
                self._context.setdefault(chat.id, deque())

                for msg in get_last_n_messages_for_ai(chat, 300):
                    if chat.is_muc and msg.nick != self.bot_nick and not msg.text.strip().startswith(self.bot_nick):
                        continue

                    msg_model = get_message_model(msg)
                    msg_encoder = self._get_encoder(msg_model)
                    msg = self._db_message_to_ai_message(msg)
                    msg_tokens = count_tokens_for_message(msg_encoder, (msg,))

                    if (tokens + msg_tokens) >= self.max_tokens:
                        break

                    tokens += msg_tokens
                    self._context[chat.id].appendleft(self.ContextItem(message=msg, model=msg_model, tokens=msg_tokens))

                self._context_tokens[chat.id] = tokens

        logger.info(f"{self.__class__.__name__}: AI context was populated")

    def _remove_oldest_message_in_context(self, chat_id: int):
        if chat_id not in self._context or not self._context[chat_id]:
            return

        removed_message = self._context[chat_id].popleft()
        self._context_tokens[chat_id] -= removed_message.tokens
