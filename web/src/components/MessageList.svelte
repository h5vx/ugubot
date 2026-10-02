<script lang="ts">
  import { tick } from 'svelte'
  import { localTime } from '../lib/dates'
  import { store } from '../lib/store.svelte'
  import { linkify, nickHue } from '../lib/text'
  import type { Message } from '../lib/types'
  import Icon from './Icon.svelte'
  import NickColor from './NickColor.svelte'

  let scroller: HTMLDivElement
  let atBottom = true
  let picker = $state<{ nick: string; x: number; y: number } | null>(null)

  const visible = $derived(
    store.showPresence
      ? store.messages
      : store.messages.filter((m) => m.kind !== 'PART_JOIN' && m.kind !== 'PART_LEAVE'),
  )

  function onScroll() {
    atBottom = scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 48
  }

  // A newly opened day starts at the bottom; new messages follow only if the
  // reader is already there.
  let shownKey = ''
  $effect(() => {
    const key = `${store.activeChatId}/${store.selectedDate}`
    const count = visible.length
    void count
    const reset = key !== shownKey
    shownKey = key
    if (reset || atBottom) {
      void tick().then(() => {
        scroller.scrollTop = scroller.scrollHeight
        atBottom = true
      })
    }
  })

  function nickStyle(nick: string) {
    const custom = store.nickColors[nick]
    return custom ? `color: ${custom}` : `color: hsl(${nickHue(nick)} 70% var(--nick-l))`
  }

  function openPicker(e: MouseEvent, nick: string) {
    picker = { nick, x: e.clientX, y: e.clientY }
  }

  function isTalk(m: Message) {
    return m.kind === 'USER' || m.kind === 'FOR_AI' || m.kind === 'MUC_PRIVMSG'
  }

  function leaveText(text: string) {
    if (!text || text === 'NORMAL') return 'вышел(а)'
    const reasons: Record<string, string> = {
      KICKED: 'выгнан(а)',
      BANNED: 'забанен(а)',
      AFFILIATION_CHANGE: 'вышел(а): изменились права',
      MODERATION_CHANGE: 'вышел(а): комната стала закрытой',
      SYSTEM_SHUTDOWN: 'вышел(а): сервер выключается',
      DISCONNECTED: 'отключился(-ась)',
    }
    if (text.startsWith('renamed to ')) return `теперь ${text.slice('renamed to '.length)}`
    return reasons[text] ?? `вышел(а) (${text.toLowerCase()})`
  }
</script>

<div class="scroller" bind:this={scroller} onscroll={onScroll}>
  {#if store.loadingMessages && store.messages.length === 0}
    <p class="placeholder">Загружаю…</p>
  {:else if visible.length === 0}
    <p class="placeholder">
      {store.isToday ? 'Сегодня сообщений ещё не было.' : 'За этот день сообщений нет.'}
    </p>
  {/if}

  <ol class="log" class:loading={store.loadingMessages}>
    {#each visible as m (m.id)}
      <li
        class="msg kind-{m.kind}"
        class:outgoing={m.outgoing}
        class:talk={isTalk(m)}
      >
        <time datetime={m.time}>{localTime(m.time)}</time>
        <div class="body">
          {#if m.kind === 'FOR_AI'}
            <span class="badge ai" title="Сообщение для AI, в чат не отправлялось"><Icon name="sparkles" size={13} /> AI</span>
          {:else if m.kind === 'MUC_PRIVMSG'}
            <span class="badge private" title="Личное сообщение в комнате"><Icon name="lock" size={12} /></span>
          {/if}

          {#if m.kind === 'FOR_AI'}
            <!-- no nick: written from the web interface -->
          {:else if m.kind === 'MUC_PRIVMSG' && m.outgoing}
            <button class="nick" style={nickStyle(m.nick)} onclick={(e) => openPicker(e, m.nick)}>→ {m.nick}</button>
          {:else}
            <button class="nick" style={nickStyle(m.nick)} onclick={(e) => openPicker(e, m.nick)}>{m.nick}</button>
          {/if}

          {#if isTalk(m)}
            <span class="text">{#each linkify(m.text) as part, i (i)}{#if part.href}<a href={part.href} target="_blank" rel="noopener noreferrer">{part.text}</a>{:else}{part.text}{/if}{/each}</span>
          {:else if m.kind === 'TOPIC'}
            <span class="event">сменил(а) тему: <q>{#each linkify(m.text) as part, i (i)}{#if part.href}<a href={part.href} target="_blank" rel="noopener noreferrer">{part.text}</a>{:else}{part.text}{/if}{/each}</q></span>
          {:else if m.kind === 'PART_JOIN'}
            <span class="event">зашёл(а)</span>
          {:else if m.kind === 'PART_LEAVE'}
            <span class="event">{leaveText(m.text)}</span>
          {/if}
        </div>
      </li>
    {/each}
  </ol>
</div>

{#if picker}
  <NickColor
    nick={picker.nick}
    x={picker.x}
    y={picker.y}
    color={store.nickColors[picker.nick] ?? ''}
    onClose={() => (picker = null)}
  />
{/if}

<style>
  .scroller {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    overscroll-behavior: contain;
  }

  .placeholder {
    margin: 32px 16px;
    text-align: center;
    color: var(--muted);
  }

  .log {
    list-style: none;
    margin: 0;
    padding: 8px 0 16px;
    transition: opacity 0.15s;
  }

  .log.loading {
    opacity: 0.5;
  }

  .msg {
    display: grid;
    grid-template-columns: 64px minmax(0, 1fr);
    gap: 10px;
    padding: 2px 16px;
    border-left: 2px solid transparent;
  }

  .msg:hover {
    background: var(--panel);
  }

  .msg.outgoing {
    border-left-color: var(--accent);
    background: color-mix(in srgb, var(--accent-soft) 45%, transparent);
  }

  .kind-FOR_AI {
    border-left-color: var(--ai) !important;
    background: color-mix(in srgb, var(--ai-soft) 70%, transparent) !important;
  }

  time {
    font: 12px/22px var(--mono);
    color: var(--faint);
    font-variant-numeric: tabular-nums;
    user-select: none;
  }

  .body {
    min-width: 0;
    line-height: 22px;
    overflow-wrap: anywhere;
  }

  .nick {
    font-weight: 600;
    margin-right: 6px;
  }

  .nick:hover {
    text-decoration: underline;
  }

  .talk .nick::after {
    content: ':';
  }

  .text {
    white-space: pre-wrap;
  }

  .event {
    color: var(--muted);
  }

  .kind-PART_JOIN,
  .kind-PART_LEAVE {
    font-size: 13px;
  }

  .kind-PART_JOIN .nick,
  .kind-PART_LEAVE .nick {
    font-weight: 500;
    opacity: 0.8;
  }

  .kind-PART_JOIN .event {
    color: var(--join);
  }

  .kind-PART_LEAVE .event {
    color: var(--leave);
  }

  .kind-TOPIC .body {
    padding: 2px 8px;
    border-radius: 6px;
    background: var(--panel-2);
  }

  q {
    color: var(--fg);
    quotes: '«' '»';
  }

  .kind-MUC_PRIVMSG .text {
    color: var(--private);
  }

  .badge {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    margin-right: 6px;
    font-size: 11px;
    font-weight: 700;
    vertical-align: 1px;
  }

  .badge.ai {
    color: var(--ai);
  }

  .badge.private {
    color: var(--private);
  }

  a {
    color: var(--link);
    text-decoration: none;
    word-break: break-all;
  }

  a:hover {
    text-decoration: underline;
  }

  @media (max-width: 520px) {
    .msg {
      grid-template-columns: minmax(0, 1fr);
      gap: 0;
      padding: 4px 12px;
    }

    time {
      line-height: 16px;
      font-size: 11px;
    }
  }
</style>
