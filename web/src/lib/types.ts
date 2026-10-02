// Mirrors internal/bus types of the Go backend.

export type Kind = 'FOR_AI' | 'USER' | 'TOPIC' | 'PART_JOIN' | 'PART_LEAVE' | 'MUC_PRIVMSG'

export interface Chat {
  id: number
  jid: string
  name: string
  is_muc: boolean
}

export interface Message {
  id: number
  chat_id: number
  time: string // RFC 3339
  kind: Kind
  nick: string
  text: string
  outgoing: boolean
}

export interface NickColor {
  nick: string
  color: string
}

export type Status = 'connecting' | 'online' | 'offline' | 'unauthorized'
