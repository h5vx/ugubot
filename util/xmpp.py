from aioxmpp.stanza import Message
from aioxmpp.structs import JID, LanguageMap, LanguageTag, MessageType

OUTGOING_MESSAGES_LANG = LanguageTag.fromstr("en")


def create_message(to: str, text: str, is_muc: bool) -> Message:
    message_type = MessageType.GROUPCHAT if is_muc else MessageType.CHAT
    chat_jid = JID.fromstr(to)

    body = LanguageMap()
    body[OUTGOING_MESSAGES_LANG] = text

    result = Message(type_=message_type, to=chat_jid)
    result.body.update(body)

    return result
