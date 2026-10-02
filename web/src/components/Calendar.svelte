<script lang="ts">
  import { untrack } from 'svelte'
  import { formatDay, humanMonth, parseDay } from '../lib/dates'
  import Icon from './Icon.svelte'

  let {
    days,
    selected,
    today,
    onPick,
  }: { days: string[]; selected: string; today: string; onPick: (day: string) => void } = $props()

  const weekdays = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс']

  // The calendar opens on the selected month and is then navigated freely.
  const start = untrack(() => parseDay(selected))
  let year = $state(start.year)
  let month = $state(start.month)

  const available = $derived(new Set(days))
  // Months that have messages, as "YYYY-MM", plus the current one.
  const months = $derived([...new Set([...days, today].map((d) => d.slice(0, 7)))].sort())
  const current = $derived(`${year}-${String(month).padStart(2, '0')}`)
  const prevMonth = $derived(months.filter((m) => m < current).at(-1))
  const nextMonth = $derived(months.find((m) => m > current))

  const cells = $derived.by(() => {
    const first = new Date(Date.UTC(year, month - 1, 1))
    const offset = (first.getUTCDay() + 6) % 7 // Monday first
    const count = new Date(Date.UTC(year, month, 0)).getUTCDate()
    const result: (string | null)[] = Array(offset).fill(null)
    for (let d = 1; d <= count; d++) result.push(formatDay(year, month, d))
    return result
  })

  const countInMonth = $derived(days.filter((d) => d.startsWith(current)).length)

  function jump(ym: string | undefined) {
    if (!ym) return
    const [y, m] = ym.split('-').map(Number)
    year = y
    month = m
  }
</script>

<div class="calendar">
  <div class="head">
    <button class="icon-button" disabled={!prevMonth} onclick={() => jump(prevMonth)} aria-label="Предыдущий месяц с сообщениями">
      <Icon name="left" />
    </button>
    <div class="month">
      <strong>{humanMonth(year, month)}</strong>
      <small>{countInMonth ? `дней с сообщениями: ${countInMonth}` : 'сообщений нет'}</small>
    </div>
    <button class="icon-button" disabled={!nextMonth} onclick={() => jump(nextMonth)} aria-label="Следующий месяц с сообщениями">
      <Icon name="right" />
    </button>
  </div>

  <div class="grid">
    {#each weekdays as w (w)}
      <span class="weekday">{w}</span>
    {/each}
    {#each cells as day, i (day ?? `empty-${i}`)}
      {#if day}
        <button
          class="day"
          class:has={available.has(day)}
          class:selected={day === selected}
          class:today={day === today}
          disabled={!available.has(day) && day !== today}
          onclick={() => onPick(day)}
          aria-label={day}
          aria-current={day === selected ? 'date' : undefined}
        >
          {Number(day.slice(8))}
        </button>
      {:else}
        <span></span>
      {/if}
    {/each}
  </div>
</div>

<style>
  .calendar {
    padding: 12px;
    width: 296px;
    max-width: 100%;
  }

  .head {
    display: flex;
    align-items: center;
    gap: 4px;
    margin-bottom: 8px;
  }

  .month {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    line-height: 1.2;
  }

  .month small {
    color: var(--muted);
    font-size: 12px;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(7, 1fr);
    gap: 2px;
    text-align: center;
  }

  .weekday {
    font-size: 11px;
    color: var(--faint);
    padding: 4px 0;
  }

  .day {
    aspect-ratio: 1;
    border-radius: 8px;
    font-variant-numeric: tabular-nums;
    color: var(--faint);
  }

  .day:disabled {
    opacity: 1;
  }

  .day.has {
    color: var(--fg);
    font-weight: 600;
    background: var(--panel-2);
  }

  .day.has:hover,
  .day.today:hover {
    background: var(--accent-soft);
  }

  .day.today {
    box-shadow: inset 0 0 0 1px var(--accent);
    color: var(--fg);
  }

  .day.selected {
    background: var(--accent);
    color: var(--accent-fg);
  }

  .day.selected:hover {
    background: var(--accent);
  }
</style>
