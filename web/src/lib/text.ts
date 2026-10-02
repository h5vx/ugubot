export type Part = { text: string; href?: string }

const urlRe = /\bhttps?:\/\/[^\s<>"«»]+/g

/** Splits text into plain parts and links (rendered as text, never as HTML). */
export function linkify(text: string): Part[] {
  const parts: Part[] = []
  let last = 0
  for (const match of text.matchAll(urlRe)) {
    let url = match[0]
    // Trailing punctuation usually belongs to the sentence.
    const trimmed = url.replace(/[.,;:!?)\]]+$/, '')
    url = trimmed
    const start = match.index ?? 0
    if (start > last) parts.push({ text: text.slice(last, start) })
    parts.push({ text: url, href: url })
    last = start + url.length
  }
  if (last < text.length) parts.push({ text: text.slice(last) })
  return parts
}

/** Stable hue for a nick, used when no color was chosen. */
export function nickHue(nick: string): number {
  let h = 0
  for (const ch of nick) h = (h * 31 + ch.codePointAt(0)!) >>> 0
  return h % 360
}
