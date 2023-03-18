import asyncio
import logging

import aioxmpp
import aioxmpp.muc

import db
from config import settings
from xmpp import Handler, XMPPBot

logger = logging.getLogger(__name__)


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
            db.store_muc_privmsg(message)
        else:
            db.store_message(message)

    @bot.register_handler(Handler.MUC_MESSAGE)
    def on_muc_message(
        message: aioxmpp.Message, member: aioxmpp.muc.Occupant, source, **kwargs
    ):
        db.store_muc_message(message, member)

    @bot.register_handler(Handler.MUC_USER_JOIN)
    def on_muc_user_join(member: aioxmpp.muc.Occupant, **kwargs):
        db.store_muc_user_join(member)

    @bot.register_handler(Handler.MUC_USER_LEAVE)
    def on_muc_leave(
        occupant: aioxmpp.muc.Occupant,
        muc_leave_mode: aioxmpp.muc.LeaveMode = None,
        **kwargs
    ):
        db.store_muc_user_leave(occupant, muc_leave_mode)

    @bot.register_handler(Handler.MUC_TOPIC_CHANGED)
    def on_topic_changed(member: aioxmpp.muc.ServiceMember, new_topic, *args, **kwargs):
        db.store_muc_topic(member, new_topic)

    for _, room in settings.xmpp.rooms.items():
        if not room.join:
            continue

        bot.join_room(room.jid, room.nick)
    try:
        await bot.run()
    finally:
        bot.stop()


def main():
    db.db_init()

    loop = asyncio.new_event_loop()

    try:
        loop.run_until_complete(bot_task())
    finally:
        loop.close()


if __name__ == "__main__":
    main()
