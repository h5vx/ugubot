import logging
import os
import typing as t
from dataclasses import dataclass
from datetime import datetime, timedelta
from enum import Enum

import aioxmpp
import pytz
from pony.orm import *

from config import settings

logger = logging.getLogger(__name__)

db = Database()


class MessageType(Enum):
    FOR_AI = 0
    USER = 1
    TOPIC = 2
    # OUTGOING = 3
    PART_JOIN = 4
    PART_LEAVE = 5
    MUC_PRIVMSG = 6


class Chat(db.Entity):
    jid = Required(str, unique=True)
    name = Required(str)
    is_muc = Required(bool)

    messages = Set("Message")
    prelude = Set("AIPrelude")


class Message(db.Entity):
    chat = Required(Chat)
    utctime = Required(datetime, index=True)
    msg_type = Required(int)
    nick = Required(str)
    text = Optional(str)
    outgoing = Required(bool)

    ai_usage = Set("AIUsage", reverse="completion")
    user_usage = Set("AIUsage", reverse="prompt")


class NickColor(db.Entity):
    nick = Required(str, unique=True)
    color = Required(str)


class BlockedUsers(db.Entity):
    jid_or_nick = Required(str, index=True, unique=True)


class AIModel(db.Entity):
    name = Required(str, unique=True)
    usages = Set("AIUsage")


class AIPrelude(db.Entity):
    chat = Required(Chat, unique=True)
    prelude = Required(str)


class AIUsage(db.Entity):
    model = Required(AIModel)

    prompt = Required(Message)
    completion = Required(Message)

    completion_tokens = Optional(int)
    prompt_tokens = Optional(int)
    total_tokens = Optional(int)


@dataclass
class AIUsageInfo:
    prompt_tokens: int
    reply_tokens: int
    total_tokens: int


def db_init():
    logger.info(f"Creating connection to database")
    db.bind(**settings.database)

    logger.info(f"Start database migration")
    db.generate_mapping(create_tables=True)


def get_or_create_muc_chat(mucjid: str):
    chat = Chat.select(is_muc=True, jid=mucjid).first()

    if chat is None:
        logger.debug("Associated chat is not found, creating new one")
        chat = Chat(jid=mucjid, name=mucjid, is_muc=True)

    return chat


def get_or_create_chat(jid: str, name: str):
    chat = Chat.select(is_muc=False, jid=jid).first()

    if chat is None:
        logger.debug("Associated chat is not found, creating new one")
        chat = Chat(jid=jid, name=name, is_muc=False)

    return chat


def get_or_create_ai_model(name: str):
    ai_model = AIModel.select(name=name).first()

    if ai_model is None:
        ai_model = AIModel(name=name)

    return ai_model


@db_session
def store_message(message: aioxmpp.Message, outgoing=False):
    logger.debug(f"Storing message {message}")

    now = datetime.now().astimezone(pytz.utc)

    contact = message.to if outgoing else message.from_
    contact_jid = str(contact.bare())
    contact_nick = message.from_.localpart if outgoing else contact.localpart

    message = Message(
        chat=get_or_create_chat(contact_jid, contact_nick),
        utctime=now,
        msg_type=MessageType.USER.value,
        nick=contact_nick,
        text=message.body.any(),
        outgoing=outgoing,
    )

    commit()

    return message


@db_session
def store_muc_message(message: aioxmpp.Message, member: aioxmpp.muc.Occupant, outgoing=False):
    logger.debug("Storing MUC message", message)

    now = datetime.now().astimezone(pytz.utc)
    mucjid = str(member.conversation_jid.bare())

    message = Message(
        chat=get_or_create_muc_chat(mucjid),
        utctime=now,
        msg_type=MessageType.USER.value,
        nick=member.nick,
        text=message.body.any(),
        outgoing=outgoing,
    )

    commit()

    return message


@db_session
def store_message_for_ai(message: aioxmpp.Message, is_muc: bool):
    now = datetime.now().astimezone(pytz.utc)

    contact_jid = str(message.to.bare())

    if is_muc:
        chat = get_or_create_muc_chat(contact_jid)
    else:
        chat = get_or_create_chat(contact_jid, message.to.localpart)

    message = Message(
        chat=chat,
        utctime=now,
        msg_type=MessageType.FOR_AI.value,
        nick="[FOR AI]",
        text=message.body.any(),
        outgoing=True,
    )

    commit()

    return message


@db_session
def store_muc_user_join(occupant: aioxmpp.muc.Occupant):
    now = datetime.now().astimezone(pytz.utc)
    mucjid = str(occupant.conversation_jid.bare())

    message = Message(
        chat=get_or_create_muc_chat(mucjid),
        utctime=now,
        msg_type=MessageType.PART_JOIN.value,
        nick=occupant.nick,
        outgoing=False,
    )

    commit()

    return message


@db_session
def store_muc_user_leave(occupant: aioxmpp.muc.Occupant, muc_leave_mode: t.Optional[aioxmpp.muc.LeaveMode]):
    now = datetime.now().astimezone(pytz.utc)
    mucjid = str(occupant.conversation_jid.bare())
    text = None

    if muc_leave_mode is not None:
        text = muc_leave_mode.name

    message = Message(
        chat=get_or_create_muc_chat(mucjid),
        utctime=now,
        msg_type=MessageType.PART_LEAVE.value,
        nick=occupant.nick,
        text=text,
        outgoing=False,
    )

    commit()

    return message


@db_session
def store_muc_topic(member: aioxmpp.muc.ServiceMember, new_topic: str):
    now = datetime.now().astimezone(pytz.utc)
    mucjid = str(member.conversation_jid.bare())
    new_topic = new_topic.any()

    message = Message(
        chat=get_or_create_muc_chat(mucjid),
        utctime=now,
        msg_type=MessageType.TOPIC.value,
        nick=member.nick or "<?>",
        text=new_topic or "",
        outgoing=False,
    )

    commit()

    return message


@db_session
def store_muc_privmsg(message: aioxmpp.Message, outgoing=False):
    now = datetime.now().astimezone(pytz.utc)
    mucjid = str(message.from_.bare())
    contact_nick = message.from_.resource

    message = Message(
        chat=get_or_create_muc_chat(mucjid),
        utctime=now,
        msg_type=MessageType.MUC_PRIVMSG.value,
        nick=contact_nick,
        text=message.body.any(),
        outgoing=outgoing,
    )

    commit()

    return message


@db_session
def store_ai_usage(prompt_message_id: int, completion_message_id: int, ai_model: str, usage_info: AIUsageInfo):
    logger.debug(
        (
            f"Storing AI Usage; #{prompt_message_id} -> #{completion_message_id},"
            f"model: {ai_model}, {usage_info.total_tokens} total tokens"
        )
    )

    ai_model = get_or_create_ai_model(ai_model)

    ai_usage = AIUsage(
        model=ai_model,
        prompt=prompt_message_id,
        completion=completion_message_id,
        completion_tokens=usage_info.reply_tokens,
        prompt_tokens=usage_info.prompt_tokens,
        total_tokens=usage_info.total_tokens,
    )

    commit()

    return ai_usage


@db_session
def get_last_n_messages_for_ai(chat: Chat, n: int):
    types = MessageType.USER.value, MessageType.FOR_AI.value

    return reversed(chat.messages.select(lambda m: m.msg_type in types).order_by(desc(Message.utctime)).limit(n))


@db_session
def get_usage_for_last_n_days(days: int, chat_id: int = None):
    start_date = datetime.now() - timedelta(days=days)
    start_date = start_date.astimezone(pytz.utc)

    chat = Chat.select(id=chat_id).get() if chat_id else None

    types = MessageType.USER.value, MessageType.FOR_AI.value
    messages = chat.messages if chat else Message
    messages = messages.select(lambda m: m.msg_type in types and m.utctime >= start_date)

    return select(
        (
            msg.chat.name,
            msg.nick,
            usage.model.name,
            usage.completion_tokens,
            usage.prompt_tokens,
            usage.total_tokens,
        )
        for msg in messages
        for usage in AIUsage
        if usage.prompt == msg
    ).fetch()


@db_session
def is_user_blocked(jid_or_nick: str) -> bool:
    return BlockedUsers.select(jid_or_nick=jid_or_nick).exists()


@db_session
def add_user_in_blocklist(jid_or_nick: str) -> None:
    BlockedUsers(jid_or_nick=jid_or_nick)
    commit()


@db_session
def remove_user_from_blocklist(jid_or_nick: str) -> bool:
    u = BlockedUsers.select(jid_or_nick=jid_or_nick).first()

    if not u:
        return False

    u.delete()
    commit()

    return True


@db_session
def get_blocked_users() -> t.List[str]:
    return list(select(u.jid_or_nick for u in BlockedUsers))
