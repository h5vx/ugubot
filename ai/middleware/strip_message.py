import typing as t

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware


class StripMessageTextMiddleware(AIBotMiddleware):
    """
    Trim space characters in message text
    """

    def incoming(self, message: IncomingMessage) -> t.Optional[t.Union[IncomingMessage, OutgoingMessage]]:
        message.text = message.text.strip()
        return message
