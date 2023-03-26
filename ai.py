import logging
from asyncio import Queue
from dataclasses import dataclass

import openai
import pytz

from config import settings
from db import Chat, Message, db_session, get_last_n_messages_for_ai, MessageType
from util.token_counter import count_tokens_for_message, get_encoder_for_model

logger = logging.getLogger(__name__)

incoming_queue = Queue()
outgoing_queue = Queue()


@dataclass
class AIMessage:
    chat_id: int
    text: str


class AIBot(object):
    def __init__(self) -> None:
        logger.info(f"Loading encoder for {settings.openai.model}...")
        self.encoder = get_encoder_for_model(settings.openai.model)
        logger.info(f"Encoder is loaded")

        prelude_text = settings.openai.prelude.text.replace("\n", " ").format(
            system_nick=settings.openai.system_nick,
            user_nick=settings.openai.user_nick,
        )

        self.prelude = [
            {"role": "user", "content": prelude_text}
        ] + settings.openai.prelude.example

        self.prelude_tokens = count_tokens_for_message(self.encoder, self.prelude)

        logger.info(f"Prelude size: {self.prelude_tokens} tokens")

        self.max_input_tokens = (
            settings.openai.max_tokens
            - settings.openai.tokens_reserved_for_response
            - self.prelude_tokens
        )

        logger.info(f"{self.max_input_tokens} tokens available for messages")

        self.messages_cache = {}
        self.messages_cache_tokens = {}

        self._prepare_messages_cache()

        openai.api_key = settings.openai.api_key

    def _db_message_to_ai_message(self, message: Message):
        tz = pytz.timezone(settings.openai.timezone)
        time = pytz.utc.normalize(pytz.utc.localize(message.utctime)).astimezone(tz)
        time_str = time.strftime("%Y/%m/%d %H:%M")

        role = "assistant" if message.outgoing else "user"
        content = f"{message.nick}: {message.text}"

        return {"role": role, "content": content}

    def _prepare_messages_cache(self):
        logger.info("Building AI messages cache")

        with db_session:
            chats = Chat.select()

            for chat in chats:
                tokens = self.prelude_tokens

                self.messages_cache.setdefault(chat.id, [])

                for msg in get_last_n_messages_for_ai(chat, 100):
                    msg = self._db_message_to_ai_message(msg)
                    msg_tokens = count_tokens_for_message(self.encoder, (msg,))

                    if (tokens + msg_tokens) >= self.max_input_tokens:
                        break

                    tokens += msg_tokens
                    self.messages_cache[chat.id].append(msg)

                self.messages_cache_tokens.setdefault(chat.id, tokens)

        logger.info("AI messages cache was built")

    def _remove_oldest_message_in_cache(self, chat_id: int):
        if chat_id not in self.messages_cache or not self.messages_cache[chat_id]:
            return

        removed_message = self.messages_cache[chat_id].pop(0)
        removed_message_tokens = count_tokens_for_message(
            self.encoder, [removed_message]
        )
        self.messages_cache_tokens[chat_id] -= removed_message_tokens

    def _cache_new_message(self, message):
        message_data = self._db_message_to_ai_message(message)
        message_tokens = count_tokens_for_message(self.encoder, [message_data])
        chat_id = message.chat.id

        if message_tokens > self.max_input_tokens:
            raise ValueError(f"Message #{message.id} is too big for AI")

        self.messages_cache.setdefault(chat_id, []).append(message_data)
        self.messages_cache_tokens.setdefault(chat_id, self.prelude_tokens)

        self.messages_cache_tokens[chat_id] += message_tokens

        while self.messages_cache_tokens[chat_id] > self.max_input_tokens:
            self._remove_oldest_message_in_cache(chat_id)

    def _process_completion(self, message, completion):
        text = completion["choices"][0]["message"]["content"]

        # AI is instructed to return [] when it don't want to respond
        if text in ("[]", "[[]]"):
            logger.info(f"AI return empty response for message #{message.id}")
            return

        logger.info(f"AI writes: {text}")
        # AI is instructed to separate each messages with empty line
        messages = text.split("\n\n")

        for msg in messages:
            outgoing_queue.put_nowait(AIMessage(chat_id=message.chat.id, text=msg))
            self.messages_cache[message.chat.id].append(
                {"role": "assistant", "content": msg}
            )

    async def get_completion(self, messages):
        if not settings.openai.enabled:
            raise ValueError("AI is disabled, so we simulate empty response")

        result = await openai.ChatCompletion.acreate(
            model=settings.openai.model,
            max_tokens=settings.openai.tokens_reserved_for_response,
            messages=messages,
        )

        return result

    async def run(self):
        while True:
            message: Message = await incoming_queue.get()

            logger.info(f"Start AI completion for message #{message.id}")

            try:
                self._cache_new_message(message)
            except ValueError as e:
                logger.warn(str(e))
                continue

            if (
                not message.msg_type == MessageType.FOR_AI.value
                and message.chat.is_muc
                and not message.text.startswith(settings.openai.user_nick)
            ):
                logger.info(
                    "Skip creating completion, because message doesn't starts with "
                    + settings.openai.user_nick
                )
                continue

            attempts = 2
            failed = False

            while attempts > 0:
                try:
                    completion = await self.get_completion(
                        self.prelude + self.messages_cache[message.chat.id]
                    )
                    failed = False
                    break
                except Exception as e:
                    failed = True
                    attempts -= 1
                    logger.exception(e)
                    logger.info(f"Tokens cache was: {self.messages_cache_tokens}")
                    self.messages_cache[message.chat.id] = []
                    self.messages_cache_tokens[message.chat.id] = 0
                    self._cache_new_message(message)
                    continue

            if not failed:
                self._process_completion(message, completion)
