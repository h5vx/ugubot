import typing as t

from config import settings

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware


class UserDefinedPromptMiddleware(AIBotMiddleware):
    """
    Handles user defined prompts
    """

    def incoming(self, message: IncomingMessage) -> t.Optional[t.Union[IncomingMessage, OutgoingMessage]]:
        if not "prompt" in settings.openai:
            return message

        for _, prompt in settings.openai.prompt.items():
            if prompt.command in message.commands:
                message.text = prompt.text + " " + message.text

        return message
