import logging
import typing as t
from datetime import datetime, timedelta

import pytz
from pydantic import BaseModel, validator

from db import Chat, Message, MessageType, db_session, select

logger = logging.getLogger(__name__)


class MessageModel(BaseModel):
    utctime: int
    msg_type: str
    nick: str
    text: str = ""

    @validator("msg_type", pre=True)
    def get_message_type(cls, v):
        return MessageType(v).name

    @validator("utctime", pre=True)
    def get_timestamp(cls, v):
        return int(v.timestamp() * 1000)

    class Config:
        orm_mode = True


class ChatModel(BaseModel):
    id: int
    jid: str
    name: str
    is_muc: bool

    class Config:
        orm_mode = True


class WebSocketCommandHandler:
    command: str = ""

    class Schema(BaseModel):
        pass

    def __init__(self, message):
        self.message = message

    def execute(self):
        try:
            data = self.Schema.parse_obj(self.message).dict()
            return {"command": self.command, "result": self.handle(**data)}
        except Exception as e:
            logger.exception("Handler {self.__class__.__name__} failed")
            return {"command": self.command, "error": f"{e.__class__.__name__}: {e}"}

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
    )
)
