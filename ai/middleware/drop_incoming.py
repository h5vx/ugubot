import logging
import typing as t

from config import settings
from db import is_user_blocked, add_user_in_blocklist, remove_user_from_blocklist, get_blocked_users
from util.plurals import pluralize

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
    Drop incoming MUC message if it is not started with bot nickname
    Otherwise cut bot nick from message
    """

    bot_nick = settings.openai.user_nick

    def incoming(self, message: IncomingMessage) -> IncomingMessage | OutgoingMessage | None:
        if not message.is_muc:
            return message

        if not message.text.startswith(self.bot_nick):
            logger.info(f"{self.__class__.__name__}: Message dropped")
            return None

        message.text = message.text[len(self.bot_nick) + 1 :].strip()
        return message


class DropIncomingIfUserIsBlockedMiddleware(AIBotMiddleware):
    """
    Drop incoming messages if user is blocked
    Also handle commands to block/unblock user
    """

    command_block_user = "block"
    command_unblock_user = "unblock"
    command_list_blocked_users = "blocklist"

    def __init__(self) -> None:
        super().__init__()

        if "admin_jids" not in settings:
            logger.warning(f"{self.__class__.__name__}: admin_jids is not configured")

    def incoming(self, message: IncomingMessage) -> IncomingMessage | OutgoingMessage | None:
        if message.is_muc and is_user_blocked(message.sender_nick):
            logger.info(f"{self.__class__.__name__}: Message dropped (blocked nickname)")
            return None

        if not message.is_muc and is_user_blocked(message.chat_jid):
            logger.info(f"{self.__class__.__name__}: Message dropped (blocked jid)")
            return None

        if self.command_block_user in message.commands:
            return self._handle_command_block(message)

        if self.command_unblock_user in message.commands:
            return self._handle_command_unblock(message)

        if self.command_list_blocked_users in message.commands:
            return self._handle_command_unblock(message)

        return message

    def _check_privileges(self, message: IncomingMessage) -> t.Tuple[bool, OutgoingMessage / None]:
        if message.is_muc:
            return False, OutgoingMessage("This command works only in private conversation")

        if "admin_jids" not in settings or message.chat_jid not in settings.admin_jids:
            return False, OutgoingMessage("You don't have access to use this command")

        return True, None

    def _handle_command_block(self, message: IncomingMessage) -> OutgoingMessage:
        can_execute, error_message = self._check_privileges(message)

        if not can_execute:
            return error_message

        jid_or_nick = message.text
        add_user_in_blocklist(jid_or_nick)

        return OutgoingMessage(f"{jid_or_nick} is added to blocklist")

    def _handle_command_unblock(self, message: IncomingMessage) -> OutgoingMessage:
        can_execute, error_message = self._check_privileges(message)

        if not can_execute:
            return error_message

        jid_or_nick = message.text
        remove_user_from_blocklist(jid_or_nick)

        return OutgoingMessage(f"{jid_or_nick} is removed from blocklist")

    def _handle_command_blocklist(self, message: IncomingMessage) -> OutgoingMessage:
        can_execute, error_message = self._check_privileges(message)

        if not can_execute:
            return error_message

        blocked_users = get_blocked_users()

        if len(blocked_users) == 0:
            return OutgoingMessage("Happy news: no one blocked. There is 0 blocked users")

        report_text = (
            f"There is {len(blocked_users)} blocked " f"{pluralize(len(blocked_users), 'user', 'users', 'users')}:"
        )

        for i, jid_or_nick in enumerate(blocked_users):
            report_text += f"\n{i}. {jid_or_nick}"

        return OutgoingMessage(report_text)
