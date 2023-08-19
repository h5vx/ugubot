import typing as t

from ai.types import IncomingMessage, OutgoingMessage

from ..types import IncomingMessage, OutgoingMessage
from .base import AIBotMiddleware

HELP = """Команды начинаются с символа ~. У бота есть следующие команды:
~dan <text> - сгенерировать ответ используя промпт BetterDAN
~clear - очистить контекст текущего чата
~prelude <text> - установить "прелюдию" для текущего чата. Прелюдия будет постоянно присутствовать в начале контекста
~context - показать текущее содержимое контекста
~help - показать эту справку

Некоторые команды можно комбинировать, например:
  - ~clear ~dan <text> - очистит контекст и сгенерирует ответ с помощью DAN
  - ~dan ~prelude <text> - установит прелюдию с промптом DAN
Команды применяются по порядку, поэтому порядок комбинирования важен.
"""


class HelpCommandHandlerMiddleware(AIBotMiddleware):
    def incoming(self, message: IncomingMessage) -> t.Optional[t.Union[IncomingMessage, OutgoingMessage]]:
        if "help" in message.commands:
            return OutgoingMessage(
                message.chat_id,
                reply_for=message.database_id,
                text=HELP,
            )
        return message
