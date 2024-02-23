import typing as t
from dataclasses import dataclass, field

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
    chat_jid: str
    is_muc: bool
    text: str
    sender_nick: str
    commands: t.List[str] = field(default_factory=list)

    openai_api_params: t.Dict[str, t.Any] = field(default_factory=dict)
    full_with_context: t.List[t.Dict[str, str]] = field(default_factory=list)
    model: str = settings.openai.model


@dataclass
class OutgoingMessage:
    chat_id: int
    reply_for: int  # Database ID of incoming message for which this reply
    text: str
    model: t.Optional[str] = None
    commands: t.Optional[t.List[str]] = field(default_factory=list)
    usage: t.Optional[AIUsageInfo] = None
