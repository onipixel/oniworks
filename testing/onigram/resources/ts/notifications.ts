import { notifications as notifAPI } from './api.ts'
import { getUser } from './auth.ts'
import { navigate } from './router.ts'
import { renderNavBar } from './feed.ts'

export async function renderNotifications(root: HTMLElement) {
  const me = getUser()
  root.innerHTML = `
  <div class="max-w-lg mx-auto px-4 py-6 space-y-4">
    ${renderNavBar(me?.username ?? '')}
    <div class="flex items-center justify-between">
      <h2 class="font-bold text-lg">Notifications</h2>
      <button id="read-all-btn" class="text-xs text-purple-400 hover:text-purple-300">Mark all read</button>
    </div>
    <div id="notif-list" class="space-y-2 text-center text-gray-500 py-12">Loading...</div>
  </div>`

  try {
    const data = await notifAPI.list()
    const notifications = data.notifications ?? []
    const listEl = root.querySelector<HTMLDivElement>('#notif-list')!
    listEl.innerHTML = ''

    if (notifications.length === 0) {
      listEl.innerHTML = '<div class="text-center text-gray-500 py-12">No notifications yet</div>'
    } else {
      notifications.forEach(n => {
        const text = typeText(n.type, n.actor?.username ?? 'someone')
        const avatar = n.actor?.avatar_path
          ? `<img src="${n.actor.avatar_path}" class="w-10 h-10 rounded-full object-cover flex-shrink-0" />`
          : `<div class="w-10 h-10 rounded-full bg-purple-700 flex items-center justify-center text-sm font-bold flex-shrink-0">${(n.actor?.username ?? '?')[0].toUpperCase()}</div>`

        listEl.insertAdjacentHTML('beforeend', `
          <div class="flex items-center gap-3 bg-gray-900 border ${n.read ? 'border-gray-800' : 'border-purple-900/50 bg-purple-950/20'} rounded-xl p-3 cursor-pointer hover:border-gray-600 transition notif-item" data-id="${n.id}" data-post-id="${n.post_id ?? ''}">
            ${avatar}
            <div class="flex-1 min-w-0">
              <p class="text-sm">${text}</p>
              <p class="text-xs text-gray-500">${timeAgo(n.created_at)}</p>
            </div>
            ${!n.read ? '<div class="w-2 h-2 rounded-full bg-purple-500 flex-shrink-0"></div>' : ''}
          </div>`)
      })
    }

    // Wire click
    root.querySelectorAll<HTMLDivElement>('.notif-item').forEach(el => {
      el.addEventListener('click', async () => {
        const id = parseInt(el.dataset.id!)
        const postId = el.dataset.postId
        await notifAPI.markRead(id).catch(() => {})
        if (postId) navigate(`/post/${postId}`)
      })
    })

    root.querySelector('#read-all-btn')!.addEventListener('click', async () => {
      await notifAPI.markAllRead()
      navigate('/notifications')
    })

  } catch {
    root.querySelector('#notif-list')!.innerHTML = '<div class="text-center text-red-400 py-12">Failed to load notifications</div>'
  }
}

// ─── Realtime WebSocket for live notification badge ───────────────

let ws: WebSocket | null = null
let badgeEl: HTMLElement | null = null

export function initRealtimeNotifications(userID: number, token: string) {
  if (ws) return // already connected
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(token)}`)

  ws.addEventListener('open', () => {
    // Subscribe to the per-user notification channel
    ws!.send(JSON.stringify({ type: 'subscribe', channel: `notify.${userID}` }))
  })

  ws.addEventListener('message', (e) => {
    try {
      const event = JSON.parse(e.data as string)
      if (event.type === 'broadcast' && event.channel?.startsWith('notify.')) {
        showBadge()
      }
    } catch { /* ignore */ }
  })

  ws.addEventListener('close', () => {
    ws = null
    // Reconnect after 3s
    setTimeout(() => initRealtimeNotifications(userID, token), 3000)
  })
}

function showBadge() {
  if (!badgeEl) {
    badgeEl = document.createElement('span')
    badgeEl.className = 'absolute -top-1 -right-1 w-4 h-4 bg-red-500 rounded-full text-xs flex items-center justify-center'
    badgeEl.textContent = '!'
    const notifLink = document.querySelector('a[href="/notifications"]')
    if (notifLink) {
      (notifLink as HTMLElement).style.position = 'relative'
      notifLink.appendChild(badgeEl)
    }
  }
}

function typeText(type: string, actor: string): string {
  switch (type) {
    case 'like':    return `<strong>${actor}</strong> liked your post`
    case 'comment': return `<strong>${actor}</strong> commented on your post`
    case 'follow':  return `<strong>${actor}</strong> started following you`
    default:        return `<strong>${actor}</strong> did something`
  }
}

function timeAgo(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`
  return `${Math.floor(diff / 86400)}d ago`
}
