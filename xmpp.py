import asyncio
import logging
import typing as t
from enum import Enum

import aioxmpp
import aioxmpp.dispatcher
from aioxmpp import vcard
from aioxmpp.dispatcher import SimpleMessageDispatcher
from aioxmpp.stanza import Message, Presence
from aioxmpp.structs import MessageType
from aioxmpp.version.xso import Query

logger = logging.getLogger(__name__)


class ClientVersion:
    def __init__(self, name, version, os=None) -> None:
        self.name = name
        self.version = version
        self.os = os


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
    def __init__(
        self,
        jid: str,
        password: str,
        ssl_verify: bool,
        version_info: ClientVersion = None,
        auto_approve_subscribe: bool = False,
    ) -> None:
        jid = aioxmpp.JID.fromstr(jid)
        password = password

        self.auto_approve_subscribe = auto_approve_subscribe

        self.client = aioxmpp.PresenceManagedClient(
            jid, aioxmpp.make_security_layer(password, no_verify=not ssl_verify)
        )
        self.version_info = version_info or ClientVersion(None, None, None)
        self.message_dispatcher: SimpleMessageDispatcher = (
            self._setup_message_dispatcher()
        )
        self.roster: aioxmpp.RosterClient = self._setup_roster_service()
        self.muc: aioxmpp.MUCClient = self._setup_muc_service()

        self.vcard = self.client.summon(vcard.VCardService)

        self.client.stream.register_iq_request_handler(
            aioxmpp.IQType.GET, Query, self.on_iq_version_query
        )

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

    def _setup_message_dispatcher(self) -> SimpleMessageDispatcher:
        md: SimpleMessageDispatcher = self.client.summon(SimpleMessageDispatcher)
        md.register_callback(MessageType.CHAT, None, self.on_message)

        return md

    def _setup_muc_service(self) -> aioxmpp.MUCClient:
        muc: aioxmpp.MUCClient = self.client.summon(aioxmpp.MUCClient)
        muc.on_conversation_new.connect(self.on_muc_new_conversation)

        return muc

    def _setup_roster_service(self) -> aioxmpp.RosterClient:
        roster: aioxmpp.RosterClient = self.client.summon(aioxmpp.RosterClient)
        roster.on_subscribe.connect(self.on_subscribe_request)
        roster.on_subscribed.connect(self.on_subscribed)
        roster.on_initial_roster_received(self.on_roster_received)
        return roster

    def on_subscribe_request(self, stanza: Presence):
        logger.info(f"Subscribe request: {stanza}")
        if self.auto_approve_subscribe:
            self.roster.approve(stanza.from_)

    def on_subscribed(self, stanza):
        logger.info(f"Subscribed: {stanza}")

    def on_roster_received(self):
        logger.info(f"Roster received: {self.roster.items}")

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

    async def on_iq_version_query(self, iq: aioxmpp.IQ):
        logger.info("IQ request from {!r}".format(iq.from_))
        result = Query()
        result.name = self.version_info.name
        result.version = self.version_info.version
        result.os = self.version_info.os
        return result

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
