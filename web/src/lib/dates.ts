export const timezone = Intl.DateTimeFormat().resolvedOptions().timeZone

const dayFormat = new Intl.DateTimeFormat('en-CA', {
  timeZone: timezone,
  year: 'numeric',
  month: '2-digit',
  day: '2-digit',
})

const timeFormat = new Intl.DateTimeFormat('ru-RU', {
  timeZone: timezone,
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  hourCycle: 'h23',
})

/** Local calendar day of a moment, as YYYY-MM-DD. */
export function localDay(date: Date | string): string {
  return dayFormat.format(typeof date === 'string' ? new Date(date) : date)
}

export function localTime(date: string): string {
  return timeFormat.format(new Date(date))
}

export function today(): string {
  return localDay(new Date())
}

/** YYYY-MM-DD → its parts; months are 1-based. */
export function parseDay(day: string): { year: number; month: number; day: number } {
  const [year, month, d] = day.split('-').map(Number)
  return { year, month, day: d }
}

export function formatDay(year: number, month: number, day: number): string {
  return `${year}-${String(month).padStart(2, '0')}-${String(day).padStart(2, '0')}`
}

const longFormat = new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long', year: 'numeric', timeZone: 'UTC' })
const weekdayFormat = new Intl.DateTimeFormat('ru-RU', { weekday: 'short', timeZone: 'UTC' })
const monthFormat = new Intl.DateTimeFormat('ru-RU', { month: 'long', year: 'numeric', timeZone: 'UTC' })

function utc(day: string): Date {
  const { year, month, day: d } = parseDay(day)
  return new Date(Date.UTC(year, month - 1, d))
}

/** "3 октября 2026 г., пт" */
export function humanDay(day: string): string {
  return `${longFormat.format(utc(day))}, ${weekdayFormat.format(utc(day))}`
}

export function humanMonth(year: number, month: number): string {
  const s = monthFormat.format(new Date(Date.UTC(year, month - 1, 1)))
  return s.charAt(0).toUpperCase() + s.slice(1)
}
