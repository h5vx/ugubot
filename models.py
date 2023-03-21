import typing as t

from pydantic import BaseModel, root_validator, validator

from db import MessageType


class MessageModel(BaseModel):
    chat: int
    utctime: int
    msg_type: str
    nick: str
    text: str = ""

    @validator("msg_type", pre=True)
    def get_message_type(cls, v):
        return MessageType(v).name

    @validator("utctime", pre=True)
    def get_timestamp(cls, v):
        return int(v.timestamp() * 1000)

    @validator("chat", pre=True)
    def get_chat_id(cls, v):
        return v.id

    class Config:
        orm_mode = True


class ChatModel(BaseModel):
    id: int
    jid: str
    name: str
    is_muc: bool

    @root_validator()
    def add_type(cls, values):
        values["type"] = "muc" if values["is_muc"] else "user"
        return values

    class Config:
        orm_mode = True
