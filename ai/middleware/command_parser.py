import logging

from config import settings

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware

logger = logging.getLogger(__name__)


class CommandParserMiddleware(AIBotMiddleware):
    """
    Parse commands and store it in message instance
    """

    command_prefix = settings.openai.command_prefix

    def incoming(self, message: IncomingMessage) -> IncomingMessage | OutgoingMessage | None:
        while message.text.startswith(self.command_prefix):
            command_and_text = message.text.split(" ", maxsplit=1)

            if len(command_and_text) > 1:
                command, message.text = command_and_text
            else:
                command, message.text = command_and_text[0], ""

            # Remove prefix from command
            command = command[len(self.command_prefix) :]

            if command not in message.commands:
                message.commands.append(command)

        if message.commands:
            logger.info(f"{self.__class__.__name__}: Parsed commands: {message.commands}")

        return message
