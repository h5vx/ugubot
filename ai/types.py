import typing as t
from dataclasses import dataclass

from config import settings


@dataclass
class AIUsageInfo:
    prompt_tokens: int
    reply_tokens: int
    total_tokens: int


@dataclass
class IncomingMessage:
    database_id: int
    chat_id: int
    text: str
    sender_nick: str
    commands: t.List[str] = []

    full_with_context: t.List[t.Dict[str, str]] = []
    model: str = settings.openai.model


@dataclass
class OutgoingMessage:
    chat_id: int
    reply_for: int  # Database ID of incoming message for which this reply
    text: str
    model: t.Optional[str] = None
    usage: t.Optional[AIUsageInfo] = None
