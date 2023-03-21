import asyncio
import logging
import typing as t
from datetime import datetime, timedelta

import pytz
from aioxmpp import stanza
from aioxmpp.structs import JID, LanguageMap, LanguageTag, MessageType
from pydantic import BaseModel

from db import Chat, Message, NickColor, db_session, select
from models import ChatModel, MessageModel

logger = logging.getLogger(__name__)
outgoung_msg_queue = asyncio.Queue()

OUTGOING_MESSAGES_LANG = LanguageTag.fromstr("en")


class WebSocketCommandHandler:
    command: str = ""

    class Schema(BaseModel):
        pass

    def __init__(self, message):
        self.message = message

    def execute(self):
        try:
            data = self.Schema.parse_obj(self.message).dict()
            data["result"] = self.handle(**data)
            data["command"] = self.command
            return data
        except Exception as e:
            logger.exception(f"Handler {self.__class__.__name__} failed")
            data["command"] = self.command
            data["error"] = f"{e.__class__.__name__}: {e}"
            return data

    def handle(self, *args, **kwargs) -> dict:
        raise NotImplemented


class ChatListHandler(WebSocketCommandHandler):
    command = "get_chat_list"

    def handle(self) -> dict:
        with db_session:
            return [ChatModel.from_orm(c).dict() for c in Chat.select()]


class ChatMessagesHandler(WebSocketCommandHandler):
    command = "get_messages"

    class Schema(BaseModel):
        chat_id: int
        date: str  # YYYY/MM/DD
        client_timezone: str

    def handle(self, chat_id: int, date: str, client_timezone: str) -> dict:
        tz = pytz.timezone(client_timezone)
        start_date = tz.normalize(tz.localize(datetime.strptime(date, "%Y/%m/%d")))
        start_date = start_date.astimezone(pytz.utc)
        stop_date = start_date + timedelta(days=1)

        with db_session:
            query = select(
                m
                for m in Message
                if m.utctime >= start_date
                and m.utctime < stop_date
                and m.chat.id == chat_id
            )
            return [MessageModel.from_orm(m).dict() for m in query]


class DatesHandler(WebSocketCommandHandler):
    command = "get_dates"

    class Schema(BaseModel):
        client_timezone: str

    def handle(self, client_timezone: str) -> dict:
        tz = pytz.timezone(client_timezone)
        result = {}

        with db_session:
            all_dates = select((m.chat.id, m.utctime) for m in Message)

            for chat_id, date in all_dates:
                dt_utc = pytz.utc.normalize(pytz.utc.localize(date))
                dt_loc = dt_utc.astimezone(tz)

                year, month, day = dt_loc.strftime("%Y,%b,%d").split(",")

                days = (
                    result.setdefault(int(chat_id), {})
                    .setdefault(year, {})
                    .setdefault(month, [])
                )

                if day not in days:
                    days.append(day)

        return result


class GetNickColorsHandler(WebSocketCommandHandler):
    command = "get_nick_colors"

    def handle(self):
        with db_session:
            return [nc.to_dict() for nc in NickColor.select()]


class SetNickColorHandler(WebSocketCommandHandler):
    command = "set_nick_color"

    class Schema(BaseModel):
        nick: str
        color: str

    def handle(self, nick: str, color: str):
        with db_session:
            nc = NickColor.select(lambda n: n.nick == nick)

            if nc.exists():
                nc.first().color = color
            else:
                NickColor(nick=nick, color=color)

        return "OK"


class SendMessageHandler(WebSocketCommandHandler):
    command = "send_message"

    class Schema(BaseModel):
        chat_id: int
        text: str

    def handle(self, chat_id, text: str) -> dict:
        with db_session:
            chat = Chat[chat_id]

        message_type = MessageType.GROUPCHAT if chat.is_muc else MessageType.CHAT
        chat_jid = JID.fromstr(chat.jid)
        body = LanguageMap()
        body[OUTGOING_MESSAGES_LANG] = text

        msg = stanza.Message(type_=message_type, to=chat_jid)
        msg.body.update(body)

        outgoung_msg_queue.put_nowait(msg)
        return "OK"


class WebSocketRouter:
    def __init__(self, handlers: t.Tuple[WebSocketCommandHandler]) -> None:
        self.handlers = {handler.command: handler for handler in handlers}

    def execute(self, message) -> dict:
        command = message.get("command", "")
        handler = self.handlers.get(command, None)

        if not handler:
            return {"command": command, "error": "No such command"}

        return handler(message).execute()


command_router = WebSocketRouter(
    handlers=(
        ChatListHandler,
        ChatMessagesHandler,
        DatesHandler,
        SendMessageHandler,
        GetNickColorsHandler,
        SetNickColorHandler,
    )
)
