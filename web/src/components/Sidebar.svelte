<script lang="ts">
  import { store } from '../lib/store.svelte'
  import Icon from './Icon.svelte'

  let {
    onPick,
    onLogout,
    onClose,
  }: { onPick: (id: number) => void; onLogout: () => void; onClose: () => void } = $props()

  const rooms = $derived(store.chats.filter((c) => c.is_muc))
  const people = $derived(store.chats.filter((c) => !c.is_muc))

  // Rooms are shown without the conference domain.
  function label(name: string, isMuc: boolean) {
    return isMuc ? name.split('@')[0] : name
  }
</script>

<aside class="sidebar">
  <div class="top">
    <span class="brand">ugubot</span>
    <button class="icon-button close" onclick={onClose} aria-label="Закрыть">
      <Icon name="close" />
    </button>
  </div>

  <nav>
    {#each [{ title: 'Комнаты', list: rooms }, { title: 'Личные', list: people }] as group (group.title)}
      {#if group.list.length}
        <h2>{group.title}</h2>
        <ul>
          {#each group.list as chat (chat.id)}
            <li>
              <button
                class="chat"
                class:active={chat.id === store.activeChatId}
                aria-current={chat.id === store.activeChatId ? 'page' : undefined}
                title={chat.jid}
                onclick={() => onPick(chat.id)}
              >
                <Icon name={chat.is_muc ? 'room' : 'user'} size={16} />
                <span class="name">{label(chat.name, chat.is_muc)}</span>
                {#if store.unread.has(chat.id)}
                  <span class="dot" aria-label="новые сообщения"></span>
                {/if}
              </button>
            </li>
          {/each}
        </ul>
      {/if}
    {/each}
    {#if store.chats.length === 0 && store.status === 'online'}
      <p class="empty">Чатов пока нет. Они появятся, когда бот получит первое сообщение.</p>
    {/if}
  </nav>

  <div class="bottom">
    <span class="tz" title="Часовой пояс браузера">{store.tz}</span>
    <button class="icon-button" onclick={onLogout} aria-label="Выйти" title="Выйти">
      <Icon name="logout" />
    </button>
  </div>
</aside>

<style>
  .sidebar {
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--panel);
    border-right: 1px solid var(--line);
  }

  .top,
  .bottom {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 8px 0 16px;
    height: 52px;
    flex-shrink: 0;
  }

  .bottom {
    border-top: 1px solid var(--line);
  }

  .brand {
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  .brand::after {
    content: '';
    display: inline-block;
    width: 6px;
    height: 6px;
    margin-left: 3px;
    border-radius: 50%;
    background: var(--accent);
  }

  .close {
    display: none;
  }

  nav {
    flex: 1;
    overflow-y: auto;
    padding: 4px 8px 16px;
  }

  h2 {
    margin: 16px 8px 6px;
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.08em;
    color: var(--faint);
  }

  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .chat {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 7px 8px;
    border-radius: 6px;
    color: var(--muted);
    text-align: left;
  }

  .chat:hover {
    background: var(--panel-2);
    color: var(--fg);
  }

  .chat.active {
    background: var(--accent-soft);
    color: var(--fg);
  }

  .chat.active :global(svg) {
    color: var(--accent);
  }

  .name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--accent);
    flex-shrink: 0;
  }

  .empty {
    padding: 8px;
    color: var(--muted);
    font-size: 14px;
  }

  .tz {
    font-size: 12px;
    color: var(--faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  @media (max-width: 860px) {
    .close {
      display: inline-grid;
    }
  }
</style>
