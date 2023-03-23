import asyncio
import logging
import typing as t
from enum import Enum

import aioxmpp
import aioxmpp.dispatcher
from aioxmpp.stanza import Message
from aioxmpp.structs import MessageType

logger = logging.getLogger(__name__)


class Handler(Enum):
    MESSAGE = 0
    MUC_MESSAGE = 1
    MUC_ENTER = 2
    MUC_USER_JOIN = 3
    MUC_USER_LEAVE = 4
    MUC_TOPIC_CHANGED = 5
    OUTGOING_MESSAGE = 6
    OUTGOING_MUC_MESSAGE = 7


class XMPPBot:
    def __init__(self, jid: str, password: str, ssl_verify: bool) -> None:
        jid = aioxmpp.JID.fromstr(jid)
        password = password

        self.client = aioxmpp.PresenceManagedClient(
            jid, aioxmpp.make_security_layer(password, no_verify=not ssl_verify)
        )
        self.message_dispatcher = self.client.summon(
            aioxmpp.dispatcher.SimpleMessageDispatcher
        )
        self.muc: aioxmpp.MUCClient = self.client.summon(aioxmpp.MUCClient)

        self.message_dispatcher.register_callback(
            aioxmpp.MessageType.CHAT,
            None,
            self.on_message,
        )

        self.muc.on_conversation_new.connect(self.on_muc_new_conversation)

        self.futures_queue = asyncio.Queue()
        self.running = False

        self.joined_rooms: t.List[aioxmpp.muc.Room] = []

        self.handlers = {
            Handler.MESSAGE.value: [],
            Handler.MUC_MESSAGE.value: [],
            Handler.MUC_ENTER.value: [],
            Handler.MUC_USER_JOIN.value: [],
            Handler.MUC_USER_LEAVE.value: [],
            Handler.MUC_TOPIC_CHANGED.value: [],
            Handler.OUTGOING_MESSAGE.value: [],
            Handler.OUTGOING_MUC_MESSAGE.value: [],
        }

    def get_room_by_muc_jid(self, muc_jid: aioxmpp.JID) -> t.Optional[aioxmpp.muc.Room]:
        for room in self.joined_rooms:
            if room.jid.localpart == muc_jid.localpart:
                return room

    def register_handler(self, handler: Handler):
        assert (
            handler.value in self.handlers
        ), f"register_handler: Unknown handler {handler}"

        def deco(func):
            self.handlers[handler.value].append(func)
            return func

        return deco

    def on_muc_new_conversation(self, room: aioxmpp.muc.Room):
        self.joined_rooms.append(room)

    def on_message(self, msg: aioxmpp.Message):
        logger.info(f">> {msg.from_}: {msg.body.any()}")

        for handler in self.handlers[Handler.MESSAGE.value]:
            handler(msg)

    def on_muc_message(
        self, message: aioxmpp.Message, member: aioxmpp.muc.Occupant, source, **kwargs
    ):
        room = self.get_room_by_muc_jid(member.conversation_jid)

        if room and room.muc_state == aioxmpp.muc.RoomState.HISTORY:
            logger.info(
                f"[HISTORY] {member.conversation_jid.localpart}@{member.conversation_jid.domain} / {member.nick}: {message.body.any()}"
            )
            return

        log = f"{member.conversation_jid.localpart}@{member.conversation_jid.domain} / {member.nick}: {message.body.any()}"

        if member.is_self:
            logger.info(f"(outgoing) {log}")

            for handler in self.handlers[Handler.OUTGOING_MUC_MESSAGE.value]:
                handler(message, member, source, **kwargs)

            return

        logger.info(log)

        for handler in self.handlers[Handler.MUC_MESSAGE.value]:
            handler(message, member, source, **kwargs)

    def on_muc_enter(
        self, presence: aioxmpp.Presence, occupant: aioxmpp.muc.Occupant, **kwargs
    ):
        logger.info(f"Joined room {presence.from_} {occupant.nick}")

        for handler in self.handlers[Handler.MUC_ENTER.value]:
            handler(presence, occupant, **kwargs)

    def on_muc_user_join(self, member: aioxmpp.muc.Occupant, **kwargs):
        muc_jid: aioxmpp.JID = member.conversation_jid
        logger.info(f"{muc_jid.bare()}: +{member.nick}")

        for handler in self.handlers[Handler.MUC_USER_JOIN.value]:
            handler(member, **kwargs)

    def on_muc_leave(
        self,
        occupant: aioxmpp.muc.Occupant,
        muc_leave_mode: aioxmpp.muc.LeaveMode = None,
        **kwargs,
    ):
        muc_jid: aioxmpp.JID = occupant.conversation_jid
        leave_mode = repr(muc_leave_mode) if muc_leave_mode else "Unknown reason"
        logger.info(f"{muc_jid.bare()}: -{occupant.nick} ({leave_mode})")

        for handler in self.handlers[Handler.MUC_USER_LEAVE.value]:
            handler(occupant, muc_leave_mode, **kwargs)

    def on_muc_topic_changed(
        self, member: aioxmpp.muc.ServiceMember, new_topic, *args, **kwargs
    ):
        logger.info(f"Topic changed by {member.conversation_jid}\n{new_topic.any()}")

        for handler in self.handlers[Handler.MUC_TOPIC_CHANGED.value]:
            handler(member, new_topic, *args, **kwargs)

    def join_room(self, jid: str, nick: str):
        jid = aioxmpp.JID.fromstr(jid)
        room, future = self.muc.join(jid, nick)

        room.on_message.connect(self.on_muc_message)
        room.on_muc_enter.connect(self.on_muc_enter)
        room.on_leave.connect(self.on_muc_leave)
        room.on_topic_changed.connect(self.on_muc_topic_changed)
        room.on_join.connect(self.on_muc_user_join)

        logger.info(f"Joining to room {jid}...")
        self.futures_queue.put_nowait(future)

    def send(self, stanza: aioxmpp.stanza.StanzaBase):
        self.client.enqueue(stanza)

        if isinstance(stanza, Message) and stanza.type_ == MessageType.CHAT:
            stanza.from_ = self.client.local_jid

            for handler in self.handlers[Handler.OUTGOING_MESSAGE.value]:
                handler(stanza)

    async def run(self):
        self.running = True

        async with self.client.connected() as stream:
            while self.running:
                task = self.futures_queue.get()
                await task
                self.futures_queue.task_done()

    def stop(self):
        self.running = False
        self.client.stop()
