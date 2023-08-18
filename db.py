import logging
import os
import typing as t
from datetime import datetime
from decimal import Decimal
from enum import Enum

import aioxmpp
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


class Message(db.Entity):
    chat = Required(Chat)
    utctime = Required(datetime)
    msg_type = Required(int)
    nick = Required(str)
    text = Optional(str)
    outgoing = Required(bool)


class NickColor(db.Entity):
    nick = Required(str, unique=True)
    color = Required(str)


class AIModel(db.Entity):
    name = Required(str)


class AIUsage(db.Entity):
    message = Required(Message)
    reply_for = Required(Message)
    model = Required(AIModel)
    tokens_spent = Optional(int)
    money_spent = Optional(Decimal)


def db_init():
    db_path = os.path.abspath(settings.database.path)
    logger.info(f"Opening database {db_path}")
    db.bind("sqlite", db_path, create_db=True)

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


@db_session
def store_message(message: aioxmpp.Message, outgoing=False):
    logger.debug(f"Storing message {message}")

    now = datetime.utcnow()

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

    now = datetime.utcnow()
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
    now = datetime.utcnow()

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
    now = datetime.utcnow()
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
    now = datetime.utcnow()
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
    now = datetime.utcnow()
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
    now = datetime.utcnow()
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
def get_last_n_messages_for_ai(chat: Chat, n: int):
    types = MessageType.USER.value, MessageType.FOR_AI.value

    return reversed(chat.messages.select(lambda m: m.msg_type in types).order_by(desc(Message.utctime)).limit(n))
