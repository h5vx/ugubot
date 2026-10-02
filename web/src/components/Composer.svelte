<script lang="ts">
  import { store } from '../lib/store.svelte'
  import Icon from './Icon.svelte'

  let text = $state('')
  let forAI = $state(false)
  let sending = $state(false)
  let error = $state<string | null>(null)
  let area: HTMLTextAreaElement

  function resize() {
    area.style.height = 'auto'
    area.style.height = `${Math.min(area.scrollHeight, 160)}px`
  }

  async function send() {
    const value = text.trim()
    if (!value || sending) return
    sending = true
    error = null
    try {
      await store.send(value, forAI)
      text = ''
      requestAnimationFrame(resize)
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      sending = false
      area.focus()
    }
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault()
      void send()
    }
  }
</script>

<form class="composer" class:ai={forAI} onsubmit={(e) => (e.preventDefault(), send())}>
  {#if error}<p class="error" role="alert">Не отправлено: {error}</p>{/if}
  <div class="box">
    <button
      type="button"
      class="icon-button"
      class:active={forAI}
      onclick={() => (forAI = !forAI)}
      aria-pressed={forAI}
      title={forAI ? 'Сообщение получит только AI' : 'Отправить в чат'}
      aria-label="Писать AI"
    >
      <Icon name="sparkles" />
    </button>
    <label class="sr-only" for="composer-input">Сообщение</label>
    <textarea
      id="composer-input"
      bind:this={area}
      bind:value={text}
      oninput={resize}
      onkeydown={onKey}
      rows="1"
      spellcheck="true"
      placeholder={forAI ? 'Вопрос для AI, в чат не уйдёт' : 'Сообщение'}
    ></textarea>
    <button class="icon-button send" type="submit" disabled={!text.trim() || sending} aria-label="Отправить">
      <Icon name="send" />
    </button>
  </div>
</form>

<style>
  .composer {
    flex-shrink: 0;
    padding: 8px 12px calc(10px + env(safe-area-inset-bottom, 0px));
    border-top: 1px solid var(--line);
    background: var(--panel);
  }

  .box {
    display: flex;
    align-items: flex-end;
    gap: 4px;
    padding: 3px;
    border: 1px solid var(--line);
    border-radius: 10px;
    background: var(--bg);
  }

  .box:focus-within {
    border-color: var(--accent);
  }

  .ai .box:focus-within,
  .ai .box {
    border-color: var(--ai);
  }

  .ai .icon-button.active {
    color: var(--ai);
  }

  textarea {
    flex: 1;
    min-width: 0;
    resize: none;
    border: 0;
    outline: none;
    background: none;
    color: var(--fg);
    font: inherit;
    line-height: 22px;
    padding: 6px 4px;
    max-height: 160px;
  }

  .send:not(:disabled) {
    color: var(--accent);
  }

  .error {
    margin: 0 0 6px;
    font-size: 13px;
    color: var(--danger);
  }

  .sr-only {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
</style>
