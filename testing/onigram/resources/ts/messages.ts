import { messages as messagesAPI } from './api.ts'
import { getUser } from './auth.ts'
import { navigate } from './router.ts'
import type { Message, Conversation } from './types.ts'
import { avatar, timeAgo, escapeHTML } from './feed.ts'

let dmWs: WebSocket | null = null
let dmCallbacks: Array<(msg: any) => void> = []

// ─── Page ─────────────────────────────────────────────────────────

export async function renderMessages(root: HTMLElement, openUsername: string | null) {
  const me = getUser()!

  root.innerHTML = `
  <div class="flex h-screen max-h-screen overflow-hidden">
    <!-- Conversation list -->
    <div id="inbox-panel" class="w-full md:w-72 min-w-0 border-r border-gray-800 flex flex-col overflow-hidden ${openUsername ? 'hidden md:flex' : 'flex'}">
      <div class="p-4 border-b border-gray-800 flex items-center justify-between flex-shrink-0">
        <h2 class="font-bold text-lg truncate">${escapeHTML(me.username)}</h2>
        <button id="new-dm-btn" class="text-purple-400 hover:text-purple-300 flex-shrink-0 ml-2">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
        </button>
      </div>
      <div id="inbox-list" class="flex-1 overflow-y-auto overflow-x-hidden">
        <div class="text-center text-gray-500 py-8 text-sm">Loading...</div>
      </div>
    </div>

    <!-- Thread panel -->
    <div id="thread-panel" class="flex-1 min-w-0 flex flex-col overflow-hidden ${openUsername ? 'flex' : 'hidden md:flex'}">
      ${openUsername ? '' : emptyThread()}
    </div>
  </div>`

  loadInbox(root, openUsername)

  root.querySelector('#new-dm-btn')?.addEventListener('click', () => openNewDMModal(root))

  if (openUsername) {
    loadThread(root, openUsername)
  }
}

async function loadInbox(root: HTMLElement, activeUsername: string | null) {
  const list = root.querySelector<HTMLElement>('#inbox-list')!
  try {
    const { conversations } = await messagesAPI.inbox()
    if (conversations.length === 0) {
      list.innerHTML = `<div class="text-center text-gray-500 py-8 text-sm">No messages yet</div>`
      return
    }
    list.innerHTML = conversations.map(c => buildConvoItem(c, activeUsername)).join('')
    list.querySelectorAll<HTMLButtonElement>('.convo-item').forEach(btn => {
      btn.addEventListener('click', () => {
        const username = btn.dataset.username!
        root.querySelector('#inbox-panel')?.classList.add('hidden')
        root.querySelector('#inbox-panel')?.classList.remove('md:flex')
        const threadPanel = root.querySelector<HTMLElement>('#thread-panel')!
        threadPanel.classList.remove('hidden')
        threadPanel.innerHTML = ''
        navigate(`/messages/${username}`)
        loadThread(root, username)
      })
    })
  } catch {
    list.innerHTML = `<div class="text-center text-red-400 py-8 text-sm">Failed to load</div>`
  }
}

function buildConvoItem(c: Conversation, activeUsername: string | null): string {
  const other = c.other_user
  const isActive = other?.username === activeUsername
  return `
  <button class="convo-item w-full flex items-center gap-3 px-4 py-3 hover:bg-gray-800/50 transition text-left min-w-0 ${isActive ? 'bg-gray-800' : ''}" data-username="${other?.username ?? ''}">
    <div class="flex-shrink-0">${avatar(other, 10)}</div>
    <div class="flex-1 min-w-0 overflow-hidden">
      <div class="flex items-center justify-between gap-2">
        <span class="font-semibold text-sm truncate">${escapeHTML(other?.username ?? '')}</span>
        ${c.last_message ? `<span class="text-xs text-gray-500 flex-shrink-0">${timeAgo(c.last_message.created_at)}</span>` : ''}
      </div>
      ${c.last_message ? `<p class="text-xs text-gray-400 truncate">${escapeHTML(c.last_message.body)}</p>` : ''}
    </div>
    ${c.unread_count > 0 ? `<span class="w-5 h-5 bg-purple-600 rounded-full text-xs flex items-center justify-center flex-shrink-0">${c.unread_count}</span>` : ''}
  </button>`
}

async function loadThread(root: HTMLElement, username: string) {
  const me = getUser()!
  const panel = root.querySelector<HTMLElement>('#thread-panel')!
  panel.innerHTML = `<div class="flex-1 flex items-center justify-center text-gray-500">Loading...</div>`

  try {
    const { messages: msgs, other_user } = await messagesAPI.thread(username)

    panel.innerHTML = `
    <div class="flex items-center gap-3 p-4 border-b border-gray-800 flex-shrink-0">
      <button id="back-to-inbox" class="md:hidden text-gray-400 mr-1">
        <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/></svg>
      </button>
      ${avatar(other_user, 9)}
      <a href="/profile/${escapeHTML(other_user.username)}" data-link class="font-semibold hover:underline">${escapeHTML(other_user.username)}</a>
    </div>
    <div id="thread-messages" class="flex-1 overflow-y-auto p-4 space-y-2 flex flex-col"></div>
    <div class="p-3 border-t border-gray-800 flex gap-2 flex-shrink-0">
      <input id="dm-input" type="text" placeholder="Message..." class="flex-1 bg-gray-800 border border-gray-700 rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-purple-500 transition" />
      <button id="dm-send" class="bg-purple-600 hover:bg-purple-700 px-4 rounded-xl text-sm font-semibold transition">Send</button>
    </div>`

    panel.querySelector('#back-to-inbox')?.addEventListener('click', () => {
      navigate('/messages')
    })

    const threadEl = panel.querySelector<HTMLElement>('#thread-messages')!
    msgs.forEach(msg => threadEl.insertAdjacentHTML('beforeend', renderMsg(msg, me.id)))
    threadEl.scrollTop = threadEl.scrollHeight

    const send = async () => {
      const input = panel.querySelector<HTMLInputElement>('#dm-input')!
      const body = input.value.trim()
      if (!body) return
      input.value = ''
      try {
        const msg = await messagesAPI.send(username, body)
        threadEl.insertAdjacentHTML('beforeend', renderMsg(msg, me.id))
        threadEl.scrollTop = threadEl.scrollHeight
      } catch { /* ignore */ }
    }

    panel.querySelector('#dm-send')!.addEventListener('click', send)
    panel.querySelector<HTMLInputElement>('#dm-input')!.addEventListener('keydown', (e) => {
      if (e.key === 'Enter') send()
    })

    // Listen for incoming messages and read receipts
    dmCallbacks.push((data) => {
      const event = data?.data?.event ?? data?.event
      const payload = data?.data?.data ?? data?.data

      if (event === 'message_read') {
        // Flip all our sent ✓ ticks to ✓✓ in this thread
        panel.querySelectorAll<HTMLElement>('.msg-tick').forEach(t => {
          t.textContent = '✓✓'
          t.classList.replace('text-purple-300/60', 'text-sky-300')
        })
        return
      }

      if (payload?.from_username === username || payload?.message?.sender_id !== me.id) {
        const msg: Message = payload?.message
        if (msg) {
          threadEl.insertAdjacentHTML('beforeend', renderMsg(msg, me.id))
          threadEl.scrollTop = threadEl.scrollHeight
        }
      }
    })

  } catch {
    panel.innerHTML = `<div class="flex-1 flex items-center justify-center text-red-400 text-sm">Failed to load thread</div>`
  }
}

function renderMsg(msg: Message, myID: number): string {
  const isMine = msg.sender_id === myID
  const readTick = isMine
    ? `<span class="msg-tick text-xs ml-1 ${msg.read ? 'text-sky-300' : 'text-purple-300/60'}">${msg.read ? '✓✓' : '✓'}</span>`
    : ''
  return `
  <div class="flex ${isMine ? 'justify-end' : 'justify-start'}" data-msg-id="${msg.id}">
    <div class="${isMine ? 'bg-purple-600 text-white' : 'bg-gray-800 text-gray-100'} rounded-2xl px-4 py-2 max-w-xs text-sm break-words">
      ${escapeHTML(msg.body)}
      <div class="flex items-center justify-end gap-1 mt-0.5">
        <span class="text-xs ${isMine ? 'text-purple-300' : 'text-gray-500'}">${timeAgo(msg.created_at)}</span>
        ${readTick}
      </div>
    </div>
  </div>`
}

function emptyThread(): string {
  return `<div class="flex-1 flex flex-col items-center justify-center text-gray-500 space-y-2">
    <svg class="w-16 h-16 text-gray-700" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
    <p class="font-semibold">Your Messages</p>
    <p class="text-sm">Send a message to start a conversation.</p>
  </div>`
}

function openNewDMModal(_root: HTMLElement) {
  const overlay = document.createElement('div')
  overlay.className = 'fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4'
  overlay.innerHTML = `
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-5 w-full max-w-sm space-y-3">
      <div class="flex items-center justify-between">
        <h3 class="font-semibold">New Message</h3>
        <button id="ndm-close" class="text-gray-400 hover:text-white">✕</button>
      </div>
      <input id="ndm-search" type="text" placeholder="Search username..." class="w-full bg-gray-800 border border-gray-700 rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-purple-500 transition" />
      <div id="ndm-results"></div>
    </div>`
  document.body.appendChild(overlay)

  const close = () => overlay.remove()
  overlay.querySelector('#ndm-close')!.addEventListener('click', close)
  overlay.addEventListener('click', (e) => { if (e.target === overlay) close() })

  let t: ReturnType<typeof setTimeout>
  overlay.querySelector<HTMLInputElement>('#ndm-search')!.addEventListener('input', (e) => {
    clearTimeout(t)
    const q = (e.target as HTMLInputElement).value.trim()
    const results = overlay.querySelector<HTMLElement>('#ndm-results')!
    if (q.length < 2) { results.innerHTML = ''; return }
    t = setTimeout(async () => {
      const { users: usersAPI } = await import('./api.ts')
      const { users } = await usersAPI.search(q)
      results.innerHTML = users.slice(0, 8).map(u => `
        <button class="w-full flex items-center gap-3 p-2 hover:bg-gray-800 rounded-xl transition ndm-user" data-username="${u.username}">
          ${avatar(u, 9)}
          <span class="text-sm font-medium">${escapeHTML(u.username)}</span>
        </button>`).join('')
      results.querySelectorAll<HTMLButtonElement>('.ndm-user').forEach(btn => {
        btn.addEventListener('click', () => {
          close()
          navigate(`/messages/${btn.dataset.username}`)
        })
      })
    }, 300)
  })
}

// ─── Realtime ─────────────────────────────────────────────────────

export function initRealtimeDMs(userID: number, token: string, opts: { onMessage: () => void }) {
  if (dmWs) return
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  dmWs = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(token)}`)

  dmWs.addEventListener('open', () => {
    dmWs!.send(JSON.stringify({ type: 'subscribe', channel: `dm.${userID}` }))
  })

  dmWs.addEventListener('message', (e) => {
    try {
      const event = JSON.parse(e.data as string)
      if (event.type === 'broadcast' && event.channel?.startsWith('dm.')) {
        opts.onMessage()
        dmCallbacks.forEach(cb => cb(event.data))
      }
    } catch { /* ignore */ }
  })

  dmWs.addEventListener('close', () => {
    dmWs = null
    setTimeout(() => initRealtimeDMs(userID, token, opts), 3000)
  })
}
