<script lang="ts">
  import { untrack } from 'svelte'
  import { store } from '../lib/store.svelte'
  import { nickHue } from '../lib/text'

  let {
    nick,
    x,
    y,
    color,
    onClose,
  }: { nick: string; x: number; y: number; color: string; onClose: () => void } = $props()

  // HSL → hex for the default color, so the native picker starts from it.
  function hueToHex(h: number): string {
    const s = 0.7
    const l = 0.72
    const f = (n: number) => {
      const k = (n + h / 30) % 12
      const c = l - s * Math.min(l, 1 - l) * Math.max(-1, Math.min(k - 3, 9 - k, 1))
      return Math.round(c * 255)
        .toString(16)
        .padStart(2, '0')
    }
    return `#${f(0)}${f(8)}${f(4)}`
  }

  // The popover is recreated for every nick, initial values are enough.
  let value = $state(untrack(() => color || hueToHex(nickHue(nick))))
  let box: HTMLDivElement

  const left = $derived(Math.max(8, Math.min(x, window.innerWidth - 240)))
  const top = $derived(Math.max(8, Math.min(y + 12, window.innerHeight - 170)))

  function save() {
    void store.setNickColor(nick, value)
    onClose()
  }

  function reset() {
    void store.setNickColor(nick, '')
    onClose()
  }

  function onWindowDown(e: PointerEvent) {
    if (!box.contains(e.target as Node)) onClose()
  }
</script>

<svelte:window onpointerdown={onWindowDown} onkeydown={(e) => e.key === 'Escape' && onClose()} />

<div class="picker" bind:this={box} style="left: {left}px; top: {top}px" role="dialog" aria-label="Цвет ника {nick}">
  <div class="row">
    <input id="nick-color" type="color" bind:value />
    <label for="nick-color">Цвет ника <strong style="color: {value}">{nick}</strong></label>
  </div>
  <div class="actions">
    {#if color}<button class="secondary" onclick={reset}>Сбросить</button>{/if}
    <button class="secondary" onclick={onClose}>Отмена</button>
    <button class="primary" onclick={save}>Сохранить</button>
  </div>
</div>

<style>
  .picker {
    position: fixed;
    z-index: 40;
    width: 228px;
    padding: 12px;
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: 12px;
    box-shadow: var(--shadow);
    display: flex;
    flex-direction: column;
    gap: 12px;
  }

  .row {
    display: flex;
    align-items: center;
    gap: 10px;
  }

  label {
    font-size: 13px;
    color: var(--muted);
    min-width: 0;
    overflow-wrap: anywhere;
  }

  input[type='color'] {
    width: 40px;
    height: 40px;
    padding: 0;
    border: 1px solid var(--line);
    border-radius: 8px;
    background: none;
    flex-shrink: 0;
  }

  .actions {
    display: flex;
    justify-content: flex-end;
    gap: 6px;
  }

  .actions button {
    padding: 5px 10px;
    border-radius: 6px;
    font-size: 13px;
  }

  .secondary {
    color: var(--muted);
  }

  .secondary:hover {
    background: var(--panel-2);
    color: var(--fg);
  }

  .primary {
    background: var(--accent);
    color: var(--accent-fg);
  }
</style>
