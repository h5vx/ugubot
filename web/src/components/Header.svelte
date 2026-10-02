<script lang="ts">
  import { humanDay } from '../lib/dates'
  import { store } from '../lib/store.svelte'
  import Calendar from './Calendar.svelte'
  import Icon from './Icon.svelte'

  let { onMenu }: { onMenu: () => void } = $props()

  let calendarOpen = $state(false)
  let wrapper = $state<HTMLDivElement>()

  const prev = $derived(store.selectedDate ? store.neighbourDay(-1) : null)
  const next = $derived(store.selectedDate ? store.neighbourDay(1) : null)

  function go(day: string | null) {
    if (day) void store.selectDate(day)
  }

  function onWindowClick(e: MouseEvent) {
    if (calendarOpen && wrapper && !wrapper.contains(e.target as Node)) calendarOpen = false
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Escape') calendarOpen = false
  }
</script>

<svelte:window onclick={onWindowClick} onkeydown={onKey} />

<header>
  <button class="icon-button menu" onclick={onMenu} aria-label="Чаты">
    <Icon name="menu" />
  </button>

  <div class="title">
    {#if store.activeChat}
      <h1>{store.activeChat.is_muc ? store.activeChat.jid.split('@')[0] : store.activeChat.name}</h1>
      <span class="jid">{store.activeChat.jid}</span>
    {:else}
      <h1>…</h1>
    {/if}
  </div>

  <button
    class="icon-button"
    class:active={!store.showPresence}
    onclick={() => store.togglePresence()}
    title={store.showPresence ? 'Скрыть входы и выходы' : 'Показать входы и выходы'}
    aria-label={store.showPresence ? 'Скрыть входы и выходы' : 'Показать входы и выходы'}
  >
    <Icon name={store.showPresence ? 'eye' : 'eyeOff'} />
  </button>

  {#if store.selectedDate}
    <div class="dates" bind:this={wrapper}>
      <button class="icon-button" disabled={!prev} onclick={() => go(prev)} aria-label="Предыдущий день с сообщениями">
        <Icon name="left" />
      </button>
      <button
        class="day"
        onclick={() => (calendarOpen = !calendarOpen)}
        aria-expanded={calendarOpen}
        aria-haspopup="dialog"
      >
        <Icon name="calendar" size={16} />
        <span>{store.isToday ? 'Сегодня' : humanDay(store.selectedDate)}</span>
      </button>
      <button class="icon-button" disabled={!next} onclick={() => go(next)} aria-label="Следующий день с сообщениями">
        <Icon name="right" />
      </button>

      {#if calendarOpen}
        <div class="popover" role="dialog" aria-label="Выбор даты">
          <Calendar
            days={store.activeDates}
            selected={store.selectedDate}
            today={store.today}
            onPick={(d) => {
              calendarOpen = false
              go(d)
            }}
          />
        </div>
      {/if}
    </div>
  {/if}
</header>

<style>
  header {
    display: flex;
    align-items: center;
    gap: 4px;
    height: 52px;
    padding: 0 8px 0 16px;
    border-bottom: 1px solid var(--line);
    background: var(--panel);
    flex-shrink: 0;
  }

  .menu {
    display: none;
    margin-left: -8px;
  }

  .title {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: 10px;
  }

  .jid {
    font-size: 13px;
    color: var(--faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  h1 {
    margin: 0;
    font-size: 16px;
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dates {
    position: relative;
    display: flex;
    align-items: center;
    gap: 2px;
  }

  .day {
    display: flex;
    align-items: center;
    gap: 8px;
    height: 34px;
    padding: 0 10px;
    border-radius: var(--radius);
    font-variant-numeric: tabular-nums;
    white-space: nowrap;
  }

  .day:hover,
  .day[aria-expanded='true'] {
    background: var(--panel-2);
  }

  .day :global(svg) {
    color: var(--muted);
  }

  .popover {
    position: absolute;
    top: calc(100% + 8px);
    right: 0;
    z-index: 30;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 12px;
    box-shadow: var(--shadow);
  }

  @media (max-width: 860px) {
    .menu {
      display: inline-grid;
    }
  }

  @media (max-width: 520px) {
    header {
      padding-left: 12px;
    }

    .jid {
      display: none;
    }

    .day span {
      max-width: 9ch;
      overflow: hidden;
      text-overflow: ellipsis;
    }

    .popover {
      position: fixed;
      top: 60px;
      left: 8px;
      right: 8px;
    }
  }
</style>
