import asyncio
import logging
import typing as t
from uuid import UUID

import aioxmpp
import aioxmpp.muc

import db
from ai import ai_bot
from ai import types as ai_types
from config import settings
from models import MessageModel
from util.xmpp import create_message
from ws_handler import OutgoingMessage, outgoing_queue
from xmpp import ClientVersion, Handler, XMPPClient

logger = logging.getLogger(__name__)


def notify_ws_clients(clients, data):
    for _, q in clients.items():
        q.put_nowait(data)


def send_message_to_ws_clients(clients, message: db.Message):
    notify_ws_clients(
        clients,
        {
            "command": "new_message",
            "message": MessageModel.from_orm(message).dict(),
        },
    )


async def bot_task(ws_clients: t.Mapping[UUID, asyncio.Queue]):
    bot = XMPPClient(
        jid=settings.xmpp.jid,
        password=settings.xmpp.password,
        ssl_verify=settings.xmpp.ssl_verify,
        version_info=ClientVersion(**settings.xmpp.iq.version),
        auto_approve_subscribe=settings.xmpp.subscribes.auto_approve,
    )

    @bot.register_handler(Handler.MESSAGE)
    def on_message(message: aioxmpp.Message):
        barejid = message.from_.bare()
        is_muc_privmsg = bot.get_room_by_muc_jid(barejid) is not None

        if is_muc_privmsg:
            message_in_db = db.store_muc_privmsg(message)
        else:
            message_in_db = db.store_message(message)

        send_message_to_ws_clients(ws_clients, message_in_db)

        ai_bot.incoming_queue.put_nowait(
            ai_types.IncomingMessage(
                database_id=message_in_db.id,
                is_muc=False,
                chat_id=message_in_db.chat.id,
                text=message_in_db.text,
                sender_nick=message_in_db.nick,
            )
        )

    @bot.register_handler(Handler.MUC_MESSAGE)
    def on_muc_message(message: aioxmpp.Message, member: aioxmpp.muc.Occupant, source, **kwargs):
        message = db.store_muc_message(message, member)
        send_message_to_ws_clients(ws_clients, message)
        ai_bot.incoming_queue.put_nowait(
            ai_types.IncomingMessage(
                database_id=message.id,
                is_muc=True,
                chat_id=message.chat.id,
                text=message.text,
                sender_nick=message.nick,
            )
        )

    @bot.register_handler(Handler.MUC_USER_JOIN)
    def on_muc_user_join(member: aioxmpp.muc.Occupant, **kwargs):
        message = db.store_muc_user_join(member)
        send_message_to_ws_clients(ws_clients, message)

    @bot.register_handler(Handler.MUC_USER_LEAVE)
    def on_muc_leave(occupant: aioxmpp.muc.Occupant, muc_leave_mode: aioxmpp.muc.LeaveMode = None, **kwargs):
        message = db.store_muc_user_leave(occupant, muc_leave_mode)
        send_message_to_ws_clients(ws_clients, message)

    @bot.register_handler(Handler.MUC_TOPIC_CHANGED)
    def on_topic_changed(member: aioxmpp.muc.ServiceMember, new_topic, *args, **kwargs):
        message = db.store_muc_topic(member, new_topic)
        send_message_to_ws_clients(ws_clients, message)

    for _, room in settings.xmpp.rooms.items():
        if not room.join:
            continue

        bot.join_room(room.jid, room.nick)

    async def webui_outgoing_messages_handler():
        while True:
            msg: OutgoingMessage = await outgoing_queue.get()
            msg_xmpp = create_message(msg.jid, msg.text, msg.is_muc)

            if msg.for_ai:
                message_in_db = db.store_message_for_ai(msg_xmpp, msg.is_muc)
                ai_bot.incoming_queue.put_nowait(
                    ai_types.IncomingMessage(
                        database_id=message_in_db.id,
                        is_muc=message_in_db.chat.is_muc,
                        chat_id=message_in_db.chat.id,
                        text=message_in_db.text,
                        sender_nick=message_in_db.nick,
                    )
                )
                send_message_to_ws_clients(ws_clients, message_in_db)
            else:
                bot.send(msg_xmpp)

    ai = ai_bot.AIBot()

    async def ai_outgoing_messages_handler():
        while True:
            msg: ai_types.OutgoingMessage = await ai_bot.outgoing_queue.get()

            with db.db_session:
                chat = db.Chat[msg.chat_id]

            msg_xmpp = create_message(chat.jid, msg.text, chat.is_muc)
            bot.send(msg_xmpp)

            send_message_to_ws_clients(ws_clients, message_in_db)

            if chat.is_muc:
                barejid = msg_xmpp.from_.bare()
                room = bot.get_room_by_muc_jid(barejid)
                message_in_db = db.store_muc_message(msg_xmpp, room.me, outgoing=True)
            else:
                message_in_db = db.store_message(msg_xmpp, outgoing=True)

            if msg.model:
                db.store_ai_usage(
                    db.Message[msg.reply_for],
                    message_in_db,
                    msg.model,
                    db.AIUsageInfo(
                        prompt_tokens=msg.usage.prompt_tokens,
                        reply_tokens=msg.usage.reply_tokens,
                        total_tokens=msg.usage.total_tokens,
                    ),
                )

    try:
        all_tasks = (
            asyncio.create_task(bot.run()),
            asyncio.create_task(webui_outgoing_messages_handler()),
            asyncio.create_task(ai.run()),
            asyncio.create_task(ai_outgoing_messages_handler()),
        )
        await asyncio.wait(all_tasks, return_when=asyncio.FIRST_COMPLETED)
    finally:
        bot.stop()
