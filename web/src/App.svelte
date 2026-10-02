<script lang="ts">
  import { getSession, logout } from './lib/api'
  import { store } from './lib/store.svelte'
  import Login from './components/Login.svelte'
  import Sidebar from './components/Sidebar.svelte'
  import Header from './components/Header.svelte'
  import MessageList from './components/MessageList.svelte'
  import Composer from './components/Composer.svelte'

  let auth = $state<'checking' | 'in' | 'out'>('checking')
  let drawerOpen = $state(false)

  getSession()
    .then((ok) => (auth = ok ? 'in' : 'out'))
    .catch(() => (auth = 'out'))

  $effect(() => {
    if (auth !== 'in') return
    store.connect()
    return () => store.disconnect()
  })

  $effect(() => {
    if (store.status === 'unauthorized') auth = 'out'
  })

  async function signOut() {
    await logout()
    auth = 'out'
  }

  function pickChat(id: number) {
    drawerOpen = false
    void store.selectChat(id)
  }
</script>

{#if auth === 'out'}
  <Login onSuccess={() => (auth = 'in')} />
{:else if auth === 'in'}
  <div class="layout" class:drawer-open={drawerOpen}>
    <Sidebar onPick={pickChat} onLogout={signOut} onClose={() => (drawerOpen = false)} />
    <button class="scrim" aria-label="Закрыть список чатов" onclick={() => (drawerOpen = false)}></button>

    <main>
      <Header onMenu={() => (drawerOpen = true)} />
      {#if store.status !== 'online'}
        <div class="banner" role="status">
          {store.status === 'connecting' ? 'Подключаюсь…' : 'Нет соединения с сервером, переподключаюсь…'}
        </div>
      {/if}
      {#if store.error}
        <div class="banner error" role="alert">
          {store.error}
          <button onclick={() => (store.error = null)}>Скрыть</button>
        </div>
      {/if}
      <MessageList />
      {#if store.isToday && store.activeChat}
        <Composer />
      {/if}
    </main>
  </div>
{/if}

<style>
  .layout {
    display: grid;
    grid-template-columns: var(--sidebar) minmax(0, 1fr);
    height: 100%;
  }

  main {
    display: flex;
    flex-direction: column;
    min-height: 0;
    min-width: 0;
    height: 100%;
  }

  .scrim {
    display: none;
  }

  .banner {
    padding: 6px 16px;
    font-size: 13px;
    background: var(--panel-2);
    color: var(--muted);
    border-bottom: 1px solid var(--line);
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .banner.error {
    color: var(--danger);
  }

  .banner button {
    margin-left: auto;
    color: var(--muted);
    text-decoration: underline;
  }

  @media (max-width: 860px) {
    .layout {
      grid-template-columns: minmax(0, 1fr);
    }

    .layout :global(.sidebar) {
      position: fixed;
      inset: 0 auto 0 0;
      width: min(var(--sidebar), 85vw);
      z-index: 20;
      transform: translateX(-100%);
      transition: transform 0.2s ease;
      box-shadow: var(--shadow);
    }

    .drawer-open :global(.sidebar) {
      transform: none;
    }

    .drawer-open .scrim {
      display: block;
      position: fixed;
      inset: 0;
      z-index: 10;
      background: rgb(0 0 0 / 0.4);
    }
  }
</style>
