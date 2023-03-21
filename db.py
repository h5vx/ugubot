import logging
import os
import typing as t
from datetime import datetime
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
    OUTGOING = 3
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
def store_message(message: aioxmpp.Message):
    now = datetime.utcnow()
    contact_jid = str(message.from_.bare())
    contact_nick = message.from_.localpart

    message = Message(
        chat=get_or_create_chat(contact_jid, contact_nick),
        utctime=now,
        msg_type=MessageType.USER.value,
        nick=contact_nick,
        text=message.body.any(),
    )

    commit()

    return message


@db_session
def store_muc_message(message: aioxmpp.Message, member: aioxmpp.muc.Occupant):
    logger.debug("Storing MUC message in DB")

    now = datetime.utcnow()
    mucjid = str(member.conversation_jid.bare())

    message = Message(
        chat=get_or_create_muc_chat(mucjid),
        utctime=now,
        msg_type=MessageType.USER.value,
        nick=member.nick,
        text=message.body.any(),
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
    )

    commit()

    return message


@db_session
def store_muc_user_leave(
    occupant: aioxmpp.muc.Occupant, muc_leave_mode: t.Optional[aioxmpp.muc.LeaveMode]
):
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
        nick=member.nick,
        text=new_topic,
    )

    commit()

    return message


@db_session
def store_muc_privmsg(message: aioxmpp.Message):
    now = datetime.utcnow()
    mucjid = str(message.from_.bare())
    contact_nick = message.from_.resource

    message = Message(
        chat=get_or_create_muc_chat(mucjid),
        utctime=now,
        msg_type=MessageType.MUC_PRIVMSG.value,
        nick=contact_nick,
        text=message.body.any(),
    )

    commit()

    return message
