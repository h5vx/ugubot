import asyncio
import logging

import aioxmpp
import aioxmpp.muc
import uvicorn

import db
from config import settings
from models import MessageModel
from webui import app, ws_clients
from ws_handler import outgoung_msg_queue
from xmpp import ClientVersion, Handler, XMPPBot

logger = logging.getLogger(__name__)


def notify_ws_clients(data):
    for _, q in ws_clients.items():
        q.put_nowait(data)


def send_message_to_ws_clients(message: db.Message):
    notify_ws_clients(
        {
            "command": "new_message",
            "message": MessageModel.from_orm(message).dict(),
        }
    )


async def bot_task():
    bot = XMPPBot(
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

        send_message_to_ws_clients(message_in_db)

    @bot.register_handler(Handler.MUC_MESSAGE)
    def on_muc_message(
        message: aioxmpp.Message, member: aioxmpp.muc.Occupant, source, **kwargs
    ):
        message = db.store_muc_message(message, member)
        send_message_to_ws_clients(message)

    @bot.register_handler(Handler.OUTGOING_MUC_MESSAGE)
    def on_muc_outgoing(
        message: aioxmpp.Message, member: aioxmpp.muc.Occupant, source, **kwargs
    ):
        message = db.store_muc_message(message, member, outgoing=True)
        send_message_to_ws_clients(message)

    @bot.register_handler(Handler.OUTGOING_MESSAGE)
    def on_non_muc_outgoing(message: aioxmpp.Message):
        message = db.store_message(message, outgoing=True)
        send_message_to_ws_clients(message)

    @bot.register_handler(Handler.MUC_USER_JOIN)
    def on_muc_user_join(member: aioxmpp.muc.Occupant, **kwargs):
        message = db.store_muc_user_join(member)
        send_message_to_ws_clients(message)

    @bot.register_handler(Handler.MUC_USER_LEAVE)
    def on_muc_leave(
        occupant: aioxmpp.muc.Occupant,
        muc_leave_mode: aioxmpp.muc.LeaveMode = None,
        **kwargs
    ):
        message = db.store_muc_user_leave(occupant, muc_leave_mode)
        send_message_to_ws_clients(message)

    @bot.register_handler(Handler.MUC_TOPIC_CHANGED)
    def on_topic_changed(member: aioxmpp.muc.ServiceMember, new_topic, *args, **kwargs):
        message = db.store_muc_topic(member, new_topic)
        send_message_to_ws_clients(message)

    for _, room in settings.xmpp.rooms.items():
        if not room.join:
            continue

        bot.join_room(room.jid, room.nick)

    async def msg_sender():
        while True:
            msg = await outgoung_msg_queue.get()
            bot.send(msg)

    try:
        await asyncio.wait(
            (bot.run(), msg_sender()), return_when=asyncio.FIRST_COMPLETED
        )
    finally:
        bot.stop()


@app.on_event("startup")
def main():
    db.db_init()

    loop = asyncio.get_event_loop()
    loop.create_task(bot_task())


if __name__ == "__main__":
    uvicorn.run(
        "webui:app",
        host=settings.webui.listen,
        port=settings.webui.port,
        loop="asyncio",
    )
