from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware


class StripMessageTextMiddleware(AIBotMiddleware):
    """
    Trim space characters in message text
    """

    def incoming(self, message: IncomingMessage) -> IncomingMessage | OutgoingMessage | None:
        message.text = message.text.strip()
        return message
