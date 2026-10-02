import type { Chat, Message, NickColor, Status } from './types'

export async function getSession(): Promise<boolean> {
  const res = await fetch('/api/session')
  const body = await res.json()
  return Boolean(body.authenticated)
}

export async function login(password: string): Promise<string | null> {
  const res = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })
  if (res.ok) return null
  if (res.status === 401) return 'Неверный пароль'
  return `Ошибка сервера (${res.status})`
}

export async function logout(): Promise<void> {
  await fetch('/api/logout', { method: 'POST' })
}

interface Pending {
  resolve: (data: unknown) => void
  reject: (err: Error) => void
}

/**
 * WebSocket client: request/response by id plus server pushes.
 * Reconnects by itself; pending requests fail on disconnect.
 */
export class Api {
  private ws: WebSocket | null = null
  private nextId = 1
  private pending = new Map<number, Pending>()
  private retry = 0
  private stopped = false

  constructor(
    private onStatus: (s: Status) => void,
    private onMessage: (m: Message) => void,
  ) {}

  connect() {
    this.stopped = false
    this.onStatus('connecting')
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    const ws = new WebSocket(`${proto}://${location.host}/ws`)
    this.ws = ws

    ws.onopen = () => {
      this.retry = 0
      this.onStatus('online')
    }
    ws.onmessage = (e) => this.handle(JSON.parse(e.data))
    ws.onclose = () => {
      if (this.ws !== ws) return
      this.ws = null
      for (const p of this.pending.values()) p.reject(new Error('соединение потеряно'))
      this.pending.clear()
      if (this.stopped) return
      void this.reconnect()
    }
  }

  close() {
    this.stopped = true
    this.ws?.close()
  }

  private async reconnect() {
    // The upgrade fails with 401 when the session expired.
    try {
      if (!(await getSession())) {
        this.onStatus('unauthorized')
        return
      }
    } catch {
      // server is down, keep retrying
    }
    this.onStatus('offline')
    const delay = Math.min(1000 * 2 ** this.retry, 15000)
    this.retry++
    setTimeout(() => this.connect(), delay)
  }

  private handle(msg: { id?: number; type?: string; data?: unknown; error?: string }) {
    if (msg.type === 'message') {
      this.onMessage(msg.data as Message)
      return
    }
    if (msg.id === undefined) return
    const p = this.pending.get(msg.id)
    if (!p) return
    this.pending.delete(msg.id)
    if (msg.error) p.reject(new Error(msg.error))
    else p.resolve(msg.data ?? null)
  }

  private request<T>(type: string, params: Record<string, unknown> = {}): Promise<T> {
    const ws = this.ws
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      return Promise.reject(new Error('нет соединения'))
    }
    const id = this.nextId++
    return new Promise<T>((resolve, reject) => {
      this.pending.set(id, { resolve: resolve as (d: unknown) => void, reject })
      ws.send(JSON.stringify({ id, type, ...params }))
    })
  }

  async chats(): Promise<Chat[]> {
    return (await this.request<Chat[] | null>('chats')) ?? []
  }

  async dates(chatId: number, tz: string): Promise<string[]> {
    return (await this.request<string[] | null>('dates', { chat_id: chatId, tz })) ?? []
  }

  async messages(chatId: number, date: string, tz: string): Promise<Message[]> {
    return (await this.request<Message[] | null>('messages', { chat_id: chatId, date, tz })) ?? []
  }

  async nickColors(): Promise<NickColor[]> {
    return (await this.request<NickColor[] | null>('nick_colors')) ?? []
  }

  setNickColor(nick: string, color: string): Promise<string> {
    return this.request('set_nick_color', { nick, color })
  }

  send(chatId: number, text: string, forAI: boolean): Promise<string> {
    return this.request('send', { chat_id: chatId, text, for_ai: forAI })
  }
}
