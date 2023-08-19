import logging
import typing as t

from config import settings

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware

logger = logging.getLogger(__name__)


class AlternateModelSwitcherMiddleware(AIBotMiddleware):
    """
    Switch AI model to alternate when specified command passed
    """

    alternate_model = settings.openai.model_secondary
    switch_command = settings.openai.model_secondary_command

    def incoming(self, message: IncomingMessage) -> t.Optional[t.Union[IncomingMessage, OutgoingMessage]]:
        if self.switch_command in message.commands:
            logger.info(f"{self.__class__.__name__} Switched to model {self.alternate_model}")
            message.model = self.alternate_model
        return message
