import asyncio
import logging
import typing as t
from dataclasses import dataclass
from datetime import datetime, timedelta

import pytz
from pydantic import BaseModel

from db import Chat, Message, NickColor, db_session, select
from models import ChatModel, MessageModel
from redis_cache import cache

logger = logging.getLogger(__name__)
outgoing_queue = asyncio.Queue()


@dataclass
class OutgoingMessage:
    jid: str
    is_muc: bool
    text: str
    for_ai: bool


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
                m for m in Message if m.utctime >= start_date and m.utctime < stop_date and m.chat.id == chat_id
            )
            return [MessageModel.from_orm(m).dict() for m in query]


class DatesHandler(WebSocketCommandHandler):
    command = "get_dates"

    class Schema(BaseModel):
        client_timezone: str

    def handle(self, client_timezone: str) -> dict:
        tz = pytz.timezone(client_timezone)

        if not cache.available:
            return self.get_all_dates_from_db(tz)

        return self.get_all_dates_from_cache(tz)

    @db_session
    def get_all_dates_from_db(self, timezone: pytz.BaseTzInfo) -> dict:
        result = {}
        all_dates = select((m.chat.id, m.utctime) for m in Message)

        for chat_id, date in all_dates:
            dt_utc = pytz.utc.normalize(pytz.utc.localize(date))
            dt_loc = dt_utc.astimezone(timezone)

            year, month, day = dt_loc.strftime("%Y,%b,%d").split(",")

            days = result.setdefault(int(chat_id), {}).setdefault(year, {}).setdefault(month, [])

            if day not in days:
                days.append(day)

        return result

    def get_all_dates_from_cache(self, timezone: pytz.BaseTzInfo) -> dict:
        result = {}

        self.update_cache_if_needed()

        for key in cache.scan_keys("chat_dates:*"):
            if key.endswith("_latest"):
                continue

            chat_id = int(key.split(":")[-1])
            chat_dates = map(datetime.fromtimestamp, cache.get(key))

            for date in chat_dates:
                dt_utc = pytz.utc.normalize(pytz.utc.localize(date))
                dt_loc = dt_utc.astimezone(timezone)

                year, month, day = dt_loc.strftime("%Y,%b,%d").split(",")
                days = result.setdefault(chat_id, {}).setdefault(year, {}).setdefault(month, [])

                if day not in days:
                    days.append(day)

        return result

    @db_session
    def update_cache_if_needed(self) -> None:
        dates_db = select((c.id, max(m.utctime)) for c in Chat for m in Message if m.chat.id == c.id)

        for chat_id, db_last_date in dates_db:
            cache_last_date = cache.get(f"chat_dates:{chat_id}_latest")

            if not cache_last_date:
                self.update_dates_cache_for_chat(chat_id)
            elif cache_last_date != db_last_date:
                self.update_dates_cache_for_chat(chat_id, since=cache_last_date)

    @db_session
    def update_dates_cache_for_chat(self, chat_id: int, since: t.Optional[datetime] = None) -> None:
        logger.info(f"Update dates cache for chat #{chat_id}")

        dates_in_cache: list = cache.get(f"chat_dates:{chat_id}") or []

        if since:
            db_chat_dates = select(m.utctime for m in Chat[chat_id].messages if m.utctime > since)
        else:
            db_chat_dates = select(m.utctime for m in Chat[chat_id].messages)

        for date in db_chat_dates:
            dates_in_cache.append(int(date.timestamp()))

        cache.set(f"chat_dates:{chat_id}", dates_in_cache)
        cache.set(f"chat_dates:{chat_id}_latest", date)


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

        for_ai = text.startswith("!!")

        if for_ai:
            text = text[2:]

        msg = OutgoingMessage(jid=chat.jid, is_muc=chat.is_muc, text=text, for_ai=for_ai)
        # msg = create_message(chat.jid, text, chat.is_muc)
        outgoing_queue.put_nowait(msg)
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
