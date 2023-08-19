import logging

from config import settings

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware

logger = logging.getLogger(__name__)


class DropIncomingIfAIDisabledMiddleware(AIBotMiddleware):
    """
    Drop incoming message if AI is disabled
    """

    def incoming(self, message: IncomingMessage) -> IncomingMessage | OutgoingMessage | None:
        if not settings.openai.enabled:
            logger.info(f"{self.__class__.__name__}: Message dropped")
            return None
        return message


class DropIncomingIfNotAddressedMiddleware(AIBotMiddleware):
    """
    Drop incoming message if it is not started with bot nickname
    Otherwise, cut bot nick from message
    """

    bot_nick = settings.openai.user_nick

    def incoming(self, message: IncomingMessage) -> IncomingMessage | OutgoingMessage | None:
        if not message.text.startswith(self.bot_nick):
            logger.info(f"{self.__class__.__name__}: Message dropped")
            return None

        message.text = message.text[len(self.bot_nick) + 1 :].strip()
        return message
