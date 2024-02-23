import typing as t

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware


class TemperatureCommandMiddleware(AIBotMiddleware):
    """
    Handle temperature (~t) user command
    """

    command_set_temperature = "t"

    def incoming(self, message: IncomingMessage) -> t.Optional[t.Union[IncomingMessage, OutgoingMessage]]:
        if self.command_set_temperature in message.commands:
            return self._handle_command_t(message)
        return message

    def _handle_command_t(self, message: IncomingMessage) -> t.Union[IncomingMessage, OutgoingMessage]:
        temp_str, message.text = message.text.split(" ", maxsplit=2)
        temp = 0.5

        if not temp_str.replace(".", "").isnumeric():
            return OutgoingMessage(
                f"Error: ~t command expects numeric argument; you pass '{temp_str}', which"
                " is not a valid number. Specify temperature as number, like this: ~t 0.5"
            )

        if len(temp_str) > 5:
            return OutgoingMessage(f"Error: your temperature is too precise")

        try:
            temp = float(temp_str)
        except ValueError as e:
            return OutgoingMessage(
                f"Error: '{temp_str}' is not a valid number. Specify temperature as number, like this: ~t 0.5"
            )

        if temp < 0.0 or temp > 1.0:
            return OutgoingMessage(f"Error: Temperature must be in range 0 - 1")

        message.openai_api_params["temperature"] = temp
        return message
