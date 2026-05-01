import { posts as postsAPI, APIError } from './api.ts'
import { navigate } from './router.ts'
import { getUser } from './auth.ts'
import type { Post } from './types.ts'

// ─── Feed Page ────────────────────────────────────────────────────

export function renderFeed(root: HTMLElement) {
  const user = getUser()

  root.innerHTML = `
  <div class="max-w-lg mx-auto px-4 py-6 space-y-6">
    ${renderNavBar(user?.username ?? '')}
    <div id="new-post-btn" class="bg-gray-900 border border-gray-800 rounded-2xl p-4 flex items-center gap-3 cursor-pointer hover:border-gray-600 transition">
      <div class="w-10 h-10 rounded-full bg-purple-700 flex items-center justify-center text-white font-bold">+</div>
      <span class="text-gray-400">Share a photo...</span>
    </div>
    <div id="feed-container" class="space-y-6">
      <div class="text-center text-gray-500 py-12">Loading feed...</div>
    </div>
    <div id="load-more" class="hidden text-center">
      <button class="text-purple-400 hover:text-purple-300 text-sm">Load more</button>
    </div>
  </div>

  <!-- New Post Modal -->
  <div id="post-modal" class="hidden fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4">
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-md space-y-4">
      <h3 class="font-semibold text-lg">New Post</h3>
      <input id="post-file" type="file" accept="image/*" class="block w-full text-sm text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:bg-purple-700 file:text-white file:border-0 hover:file:bg-purple-600" />
      <img id="post-preview" class="hidden rounded-lg w-full object-cover max-h-64" />
      <textarea id="post-caption" placeholder="Write a caption..." class="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-sm resize-none h-24 focus:outline-none focus:border-purple-500"></textarea>
      <div id="post-error" class="hidden text-red-400 text-sm"></div>
      <div class="flex gap-3">
        <button id="post-cancel" class="flex-1 border border-gray-700 rounded-lg py-2 text-sm hover:bg-gray-800 transition">Cancel</button>
        <button id="post-submit" class="flex-1 bg-purple-600 hover:bg-purple-700 rounded-lg py-2 text-sm font-semibold transition">Post</button>
      </div>
    </div>
  </div>`

  let page = 1
  loadFeed(1)

  root.querySelector('#new-post-btn')!.addEventListener('click', () => {
    root.querySelector('#post-modal')!.classList.remove('hidden')
  })
  root.querySelector('#post-cancel')!.addEventListener('click', () => {
    root.querySelector('#post-modal')!.classList.add('hidden')
  })

  root.querySelector('#post-file')!.addEventListener('change', (e) => {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file) return
    const preview = root.querySelector<HTMLImageElement>('#post-preview')!
    preview.src = URL.createObjectURL(file)
    preview.classList.remove('hidden')
  })

  root.querySelector('#post-submit')!.addEventListener('click', async () => {
    const fileEl = root.querySelector<HTMLInputElement>('#post-file')!
    const captionEl = root.querySelector<HTMLTextAreaElement>('#post-caption')!
    const errEl = root.querySelector<HTMLDivElement>('#post-error')!
    const file = fileEl.files?.[0]
    if (!file) { errEl.textContent = 'Please select an image'; errEl.classList.remove('hidden'); return }

    const form = new FormData()
    form.append('image', file)
    form.append('caption', captionEl.value)

    try {
      await postsAPI.create(form)
      root.querySelector('#post-modal')!.classList.add('hidden')
      page = 1
      loadFeed(1)
    } catch (e) {
      errEl.textContent = e instanceof APIError ? e.message : 'Upload failed'
      errEl.classList.remove('hidden')
    }
  })

  root.querySelector('#load-more button')?.addEventListener('click', () => {
    page++
    loadFeed(page)
  })

  async function loadFeed(p: number) {
    const container = root.querySelector<HTMLDivElement>('#feed-container')!
    if (p === 1) container.innerHTML = '<div class="text-center text-gray-500 py-12">Loading...</div>'
    try {
      const { posts } = await postsAPI.feed(p)
      if (p === 1) container.innerHTML = ''
      if (posts.length === 0 && p === 1) {
        container.innerHTML = '<div class="text-center text-gray-500 py-12">No posts yet. Follow someone!</div>'
        return
      }
      posts.forEach(post => {
        container.insertAdjacentHTML('beforeend', renderPostCard(post))
      })
      root.querySelector('#load-more')!.classList.toggle('hidden', posts.length < 20)
      wirePostActions(root)
    } catch {
      container.innerHTML = '<div class="text-center text-red-400 py-12">Failed to load feed</div>'
    }
  }
}

export function renderPostCard(post: Post): string {
  const avatar = post.user?.avatar_path
    ? `<img src="${post.user.avatar_path}" class="w-9 h-9 rounded-full object-cover" />`
    : `<div class="w-9 h-9 rounded-full bg-purple-700 flex items-center justify-center text-sm font-bold">${(post.user?.username ?? '?')[0].toUpperCase()}</div>`

  return `
  <div class="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden" data-post-id="${post.id}">
    <div class="flex items-center gap-3 p-4">
      <a href="/profile/${post.user?.username ?? ''}" data-link>${avatar}</a>
      <a href="/profile/${post.user?.username ?? ''}" data-link class="font-semibold text-sm hover:underline">${post.user?.username ?? 'unknown'}</a>
      <span class="ml-auto text-xs text-gray-500">${timeAgo(post.created_at)}</span>
    </div>
    <a href="/post/${post.id}" data-link>
      <img src="${post.image_path}" alt="post" class="w-full object-cover max-h-[500px]" loading="lazy" />
    </a>
    <div class="p-4 space-y-3">
      <div class="flex items-center gap-4">
        <button class="like-btn flex items-center gap-1.5 text-sm ${post.is_liked ? 'text-pink-500' : 'text-gray-400 hover:text-pink-400'} transition" data-post-id="${post.id}" data-liked="${post.is_liked}">
          <svg class="w-5 h-5" fill="${post.is_liked ? 'currentColor' : 'none'}" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/></svg>
          <span class="like-count">${post.like_count ?? 0}</span>
        </button>
        <a href="/post/${post.id}" data-link class="flex items-center gap-1.5 text-sm text-gray-400 hover:text-white transition">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
          Comment
        </a>
      </div>
      ${post.caption ? `<p class="text-sm"><span class="font-semibold">${post.user?.username ?? ''}</span> ${escapeHTML(post.caption)}</p>` : ''}
    </div>
  </div>`
}

export function wirePostActions(root: HTMLElement) {
  root.querySelectorAll<HTMLButtonElement>('.like-btn').forEach(btn => {
    // Avoid double-binding
    if (btn.dataset.bound) return
    btn.dataset.bound = '1'
    btn.addEventListener('click', async () => {
      const postID = parseInt(btn.dataset.postId!)
      const isLiked = btn.dataset.liked === 'true'
      try {
        const res = isLiked ? await postsAPI.unlike(postID) : await postsAPI.like(postID)
        btn.dataset.liked = isLiked ? 'false' : 'true'
        btn.querySelector('.like-count')!.textContent = String(res.like_count)
        btn.classList.toggle('text-pink-500', !isLiked)
        btn.classList.toggle('text-gray-400', isLiked)
        const svg = btn.querySelector('svg')!
        svg.setAttribute('fill', isLiked ? 'none' : 'currentColor')
      } catch { /* ignore */ }
    })
  })
}

function timeAgo(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`
  return `${Math.floor(diff / 86400)}d`
}

function escapeHTML(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

export function renderNavBar(username: string): string {
  return `
  <nav class="flex items-center justify-between">
    <span class="text-xl font-bold bg-gradient-to-r from-purple-400 to-pink-400 bg-clip-text text-transparent">OniGram</span>
    <div class="flex items-center gap-4">
      <a href="/notifications" data-link class="text-gray-400 hover:text-white transition" title="Notifications">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9"/></svg>
      </a>
      <a href="/profile/${username}" data-link class="text-gray-400 hover:text-white transition" title="Profile">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
      </a>
    </div>
  </nav>`
}

// Navigate to feed from anywhere
export function goFeed() {
  navigate('/feed')
}
