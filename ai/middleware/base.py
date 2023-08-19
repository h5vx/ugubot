import typing as t

from ..types import IncomingMessage, OutgoingMessage


class AIBotMiddleware(object):
    """
    Base middleware class for AIBot
    """

    def incoming(self, message: IncomingMessage) -> t.Optional[t.Union[IncomingMessage, OutgoingMessage]]:
        """
        This method should handle incoming message.
        It should return either:
            - IncomingMessage instance to complete processing
            - OutgoungMessage instance for direct answer without completion by AI
            - None to stop processing
        """
        return message

    def outgoing(self, message: OutgoingMessage) -> t.Optional[OutgoingMessage]:
        """
        This method should handle outgoing message.
        It should return message instance, probably modified, or None to stop processing
        """
        return message
