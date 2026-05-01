import { notifications as notifAPI } from './api.ts'
import { navigate } from './router.ts'
import { avatar, timeAgo, escapeHTML } from './feed.ts'

// ─── Page ─────────────────────────────────────────────────────────

export async function renderNotifications(root: HTMLElement) {
  root.innerHTML = `
  <div class="max-w-lg mx-auto px-4 py-6">
    <div class="flex items-center justify-between mb-4">
      <h2 class="font-bold text-xl">Notifications</h2>
      <button id="read-all-btn" class="text-xs text-purple-400 hover:text-purple-300">Mark all read</button>
    </div>
    <div id="notif-list" class="space-y-1">
      ${[1,2,3].map(() => `<div class="h-16 rounded-xl bg-gray-900 animate-pulse"></div>`).join('')}
    </div>
  </div>`

  try {
    const { notifications } = await notifAPI.list()
    const listEl = root.querySelector<HTMLDivElement>('#notif-list')!
    listEl.innerHTML = ''

    const items = notifications ?? []
    if (items.length === 0) {
      listEl.innerHTML = `<div class="text-center text-gray-500 py-16 text-sm">No notifications yet</div>`
    } else {
      items.forEach(n => {
        const text = typeText(n.type, n.actor?.username ?? 'someone')
        const div = document.createElement('div')
        div.className = `flex items-center gap-3 p-3 rounded-xl cursor-pointer transition notif-item ${n.read ? 'hover:bg-gray-900' : 'bg-purple-950/30 border border-purple-900/40 hover:bg-purple-950/50'}`
        div.dataset.id = String(n.id)
        div.dataset.postId = String(n.post_id ?? '')
        div.dataset.type = n.type
        div.dataset.actorUsername = n.actor?.username ?? ''
        div.innerHTML = `
          ${avatar(n.actor, 10)}
          <div class="flex-1 min-w-0">
            <p class="text-sm leading-snug">${text}</p>
            <p class="text-xs text-gray-500 mt-0.5">${timeAgo(n.created_at)}</p>
          </div>
          ${!n.read ? '<div class="w-2 h-2 rounded-full bg-purple-500 flex-shrink-0"></div>' : ''}`
        listEl.appendChild(div)
      })

      root.querySelectorAll<HTMLElement>('.notif-item').forEach(el => {
        el.addEventListener('click', async () => {
          const id = parseInt(el.dataset.id!)
          const postId = el.dataset.postId
          const type = el.dataset.type
          const actorUsername = el.dataset.actorUsername
          await notifAPI.markRead(id).catch(() => {})
          el.classList.remove('bg-purple-950/30', 'border', 'border-purple-900/40')
          el.querySelector('.rounded-full.bg-purple-500')?.remove()
          if (type === 'dm' && actorUsername) {
            navigate(`/messages/${actorUsername}`)
          } else if (postId) {
            navigate(`/post/${postId}`)
          } else if (actorUsername) {
            navigate(`/profile/${actorUsername}`)
          }
        })
      })
    }

    root.querySelector('#read-all-btn')!.addEventListener('click', async () => {
      await notifAPI.markAllRead()
      navigate('/notifications')
    })
  } catch {
    root.querySelector('#notif-list')!.innerHTML = `<div class="text-center text-red-400 py-16 text-sm">Failed to load notifications</div>`
  }
}

// ─── Realtime badge ───────────────────────────────────────────────

let ws: WebSocket | null = null

export function initRealtimeNotifications(
  userID: number,
  token: string,
  opts: { onBadge: () => void }
) {
  if (ws) return
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  ws = new WebSocket(`${proto}://${location.host}/ws?token=${encodeURIComponent(token)}`)

  ws.addEventListener('open', () => {
    ws!.send(JSON.stringify({ type: 'subscribe', channel: `notify.${userID}` }))
  })

  ws.addEventListener('message', (e) => {
    try {
      const event = JSON.parse(e.data as string)
      if (event.type === 'broadcast' && event.channel?.startsWith('notify.')) {
        opts.onBadge()
      }
    } catch { /* ignore */ }
  })

  ws.addEventListener('close', () => {
    ws = null
    setTimeout(() => initRealtimeNotifications(userID, token, opts), 3000)
  })
}

export function updateNotifBadge(appEl: HTMLElement, show: boolean) {
  const badges = appEl.querySelectorAll<HTMLElement>('#notif-badge-sidebar')
  badges.forEach(b => b.classList.toggle('hidden', !show))
}

// ─── Helpers ─────────────────────────────────────────────────────

function typeText(type: string, actor: string): string {
  const a = `<strong>${escapeHTML(actor)}</strong>`
  switch (type) {
    case 'like':    return `${a} liked your post`
    case 'comment': return `${a} commented on your post`
    case 'follow':  return `${a} started following you`
    case 'dm':      return `${a} sent you a message`
    default:        return `${a} did something`
  }
}
