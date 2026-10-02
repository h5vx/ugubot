<script lang="ts">
  import { login } from '../lib/api'
  import Icon from './Icon.svelte'

  let { onSuccess }: { onSuccess: () => void } = $props()

  let password = $state('')
  let error = $state<string | null>(null)
  let busy = $state(false)
  let input: HTMLInputElement

  async function submit(e: SubmitEvent) {
    e.preventDefault()
    if (!password || busy) return
    busy = true
    error = null
    try {
      error = await login(password)
    } catch {
      error = 'Сервер недоступен'
    }
    busy = false
    if (error) {
      password = ''
      input.focus()
    } else {
      onSuccess()
    }
  }
</script>

<div class="screen">
  <form class="card" onsubmit={submit}>
    <h1>ugubot</h1>
    <p>Архив XMPP-чатов</p>
    <label for="password">Пароль</label>
    <div class="row">
      <!-- svelte-ignore a11y_autofocus -->
      <input
        id="password"
        type="password"
        autocomplete="current-password"
        bind:value={password}
        bind:this={input}
        autofocus
        aria-invalid={error ? 'true' : undefined}
      />
      <button type="submit" disabled={!password || busy} aria-label="Войти">
        <Icon name="login" />
      </button>
    </div>
    {#if error}<p class="error" role="alert">{error}</p>{/if}
  </form>
</div>

<style>
  .screen {
    min-height: 100%;
    display: grid;
    place-items: center;
    padding: 16px;
  }

  .card {
    width: min(360px, 100%);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  h1 {
    margin: 0;
    font-size: 28px;
    letter-spacing: -0.02em;
  }

  h1::after {
    content: '';
    display: inline-block;
    width: 8px;
    height: 8px;
    margin-left: 4px;
    border-radius: 50%;
    background: var(--accent);
  }

  p {
    margin: 0 0 16px;
    color: var(--muted);
  }

  label {
    font-size: 13px;
    color: var(--muted);
  }

  .row {
    display: flex;
    gap: 8px;
  }

  input {
    flex: 1;
    min-width: 0;
    font: inherit;
    color: var(--fg);
    background: var(--panel);
    border: 1px solid var(--line);
    border-radius: var(--radius);
    padding: 10px 12px;
  }

  input:focus {
    outline: none;
    border-color: var(--accent);
  }

  button {
    display: grid;
    place-items: center;
    width: 44px;
    border-radius: var(--radius);
    background: var(--accent);
    color: var(--accent-fg);
  }

  .error {
    margin: 4px 0 0;
    color: var(--danger);
    font-size: 14px;
  }
</style>
