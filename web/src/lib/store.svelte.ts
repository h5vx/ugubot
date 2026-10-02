import { SvelteSet } from 'svelte/reactivity'
import { Api } from './api'
import { localDay, timezone, today } from './dates'
import type { Chat, Kind, Message, Status } from './types'

const LAST_CHAT_KEY = 'ugubot:last-chat'
const SHOW_PRESENCE_KEY = 'ugubot:show-presence'

// Kinds that mark a chat as unread. Joins and leaves are just noise.
const notableKinds: Kind[] = ['USER', 'MUC_PRIVMSG', 'TOPIC', 'FOR_AI']

function storageGet(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

function storageSet(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // storage may be unavailable, it's only a convenience
  }
}

class ChatStore {
  status = $state<Status>('connecting')
  chats = $state<Chat[]>([])
  activeChatId = $state<number | null>(null)
  /** Days with messages per chat, ascending. */
  dates = $state<Record<number, string[]>>({})
  selectedDate = $state<string | null>(null)
  messages = $state<Message[]>([])
  loadingMessages = $state(false)
  error = $state<string | null>(null)
  unread = new SvelteSet<number>()
  nickColors = $state<Record<string, string>>({})
  showPresence = $state(storageGet(SHOW_PRESENCE_KEY) !== 'false')
  today = $state(today())

  readonly tz = timezone
  private api: Api
  private messagesRequest = 0

  constructor() {
    this.api = new Api(
      (s) => this.onStatus(s),
      (m) => this.onPush(m),
    )
    // Keep "today" right after midnight.
    setInterval(() => (this.today = today()), 30_000)
  }

  activeChat = $derived(this.chats.find((c) => c.id === this.activeChatId) ?? null)
  activeDates = $derived(this.activeChatId === null ? [] : (this.dates[this.activeChatId] ?? []))
  isToday = $derived(this.selectedDate === this.today)

  connect() {
    this.api.connect()
  }

  disconnect() {
    this.api.close()
  }

  private async onStatus(s: Status) {
    this.status = s
    if (s !== 'online') return

    // (Re)load everything: we might have missed messages while offline.
    try {
      const [chats, colors] = await Promise.all([this.api.chats(), this.api.nickColors()])
      this.chats = chats
      this.nickColors = Object.fromEntries(colors.map((c) => [c.nick, c.color]))
      this.dates = {}

      const remembered = Number(storageGet(LAST_CHAT_KEY))
      const id = this.activeChatId ?? (chats.some((c) => c.id === remembered) ? remembered : chats[0]?.id)
      if (id !== undefined) await this.selectChat(id, this.selectedDate)
    } catch (e) {
      this.fail(e)
    }
  }

  async selectChat(id: number, date: string | null = null) {
    this.activeChatId = id
    this.unread.delete(id)
    storageSet(LAST_CHAT_KEY, String(id))

    try {
      if (!(id in this.dates)) {
        const dates = await this.api.dates(id, this.tz)
        this.dates[id] = dates
      }
      if (this.activeChatId !== id) return
      const days = this.dates[id]
      await this.selectDate(date ?? days[days.length - 1] ?? this.today)
    } catch (e) {
      this.fail(e)
    }
  }

  async selectDate(date: string) {
    const chatId = this.activeChatId
    if (chatId === null) return
    this.selectedDate = date
    if (date === this.today) this.unread.delete(chatId)

    const req = ++this.messagesRequest
    this.loadingMessages = true
    try {
      const messages = await this.api.messages(chatId, date, this.tz)
      if (req !== this.messagesRequest) return // a newer selection won
      this.messages = messages
      this.error = null
    } catch (e) {
      if (req === this.messagesRequest) this.fail(e)
    } finally {
      if (req === this.messagesRequest) this.loadingMessages = false
    }
  }

  /** Neighbouring day with messages: -1 earlier, +1 later. */
  neighbourDay(direction: -1 | 1): string | null {
    const days = this.activeDates
    const current = this.selectedDate
    if (!current) return null
    if (direction < 0) {
      for (let i = days.length - 1; i >= 0; i--) if (days[i] < current) return days[i]
    } else {
      for (const d of days) if (d > current) return d
      if (current < this.today) return this.today
    }
    return null
  }

  private onPush(m: Message) {
    if (!this.chats.some((c) => c.id === m.chat_id)) {
      void this.api.chats().then((chats) => (this.chats = chats))
    }

    const day = localDay(m.time)
    const days = this.dates[m.chat_id]
    if (days && !days.includes(day)) {
      this.dates[m.chat_id] = [...days, day].sort()
    }

    const visible = m.chat_id === this.activeChatId && day === this.selectedDate
    if (visible) {
      if (!this.messages.some((x) => x.id === m.id)) this.messages.push(m)
    } else if (notableKinds.includes(m.kind) && !m.outgoing) {
      this.unread.add(m.chat_id)
    }
  }

  async send(text: string, forAI: boolean) {
    if (this.activeChatId === null) return
    await this.api.send(this.activeChatId, text, forAI)
  }

  async setNickColor(nick: string, color: string) {
    const previous = this.nickColors[nick]
    if (color) this.nickColors[nick] = color
    else delete this.nickColors[nick]
    try {
      await this.api.setNickColor(nick, color)
    } catch (e) {
      if (previous) this.nickColors[nick] = previous
      this.fail(e)
    }
  }

  togglePresence() {
    this.showPresence = !this.showPresence
    storageSet(SHOW_PRESENCE_KEY, String(this.showPresence))
  }

  private fail(e: unknown) {
    this.error = e instanceof Error ? e.message : String(e)
  }
}

export const store = new ChatStore()
