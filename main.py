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
from xmpp import Handler, XMPPBot

logger = logging.getLogger(__name__)


def notify_ws_clients(data):
    for _, q in ws_clients.items():
        q.put_nowait(data)


async def bot_task():
    bot = XMPPBot(
        jid=settings.xmpp.jid,
        password=settings.xmpp.password,
        ssl_verify=settings.xmpp.ssl_verify,
    )

    @bot.register_handler(Handler.MESSAGE)
    def on_message(message: aioxmpp.Message):
        barejid = message.from_.bare()
        is_muc_privmsg = bot.get_room_by_muc_jid(barejid) is not None

        if is_muc_privmsg:
            message_in_db = db.store_muc_privmsg(message)
        else:
            message_in_db = db.store_message(message)

        notify_ws_clients(
            {
                "command": "new_message",
                "message": MessageModel.from_orm(message_in_db).dict(),
            }
        )

    @bot.register_handler(Handler.MUC_MESSAGE)
    def on_muc_message(
        message: aioxmpp.Message, member: aioxmpp.muc.Occupant, source, **kwargs
    ):
        message_in_db = db.store_muc_message(message, member)

        notify_ws_clients(
            {
                "command": "new_message",
                "message": MessageModel.from_orm(message_in_db).dict(),
            }
        )

    @bot.register_handler(Handler.MUC_USER_JOIN)
    def on_muc_user_join(member: aioxmpp.muc.Occupant, **kwargs):
        message = db.store_muc_user_join(member)

        notify_ws_clients(
            {
                "command": "new_message",
                "message": MessageModel.from_orm(message).dict(),
            }
        )

    @bot.register_handler(Handler.MUC_USER_LEAVE)
    def on_muc_leave(
        occupant: aioxmpp.muc.Occupant,
        muc_leave_mode: aioxmpp.muc.LeaveMode = None,
        **kwargs
    ):
        message = db.store_muc_user_leave(occupant, muc_leave_mode)
        notify_ws_clients(
            {
                "command": "new_message",
                "message": MessageModel.from_orm(message).dict(),
            }
        )

    @bot.register_handler(Handler.MUC_TOPIC_CHANGED)
    def on_topic_changed(member: aioxmpp.muc.ServiceMember, new_topic, *args, **kwargs):
        message = db.store_muc_topic(member, new_topic)
        notify_ws_clients(
            {
                "command": "new_message",
                "message": MessageModel.from_orm(message).dict(),
            }
        )

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
