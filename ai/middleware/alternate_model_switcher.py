from config import settings

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware


class AlternateModelSwitcherMiddleware(AIBotMiddleware):
    """
    Switch AI model to alternate when specified command passed
    """

    alternate_model = settings.openai.model_secondary
    switch_command = settings.openai.model_secondary_command

    def incoming(self, message: IncomingMessage) -> IncomingMessage | OutgoingMessage | None:
        if self.switch_command in message.commands:
            message.model = self.alternate_model
        return message
