import { posts as postsAPI, comments as commentsAPI, APIError } from './api.ts'
import { getUser } from './auth.ts'
import type { Post, Comment } from './types.ts'
import { avatar, timeAgo, mentionify, escapeHTML } from './feed.ts'

// ─── Page entry point (renders inside shell content area) ─────────

export async function renderPost(root: HTMLElement, params: Record<string, string>) {
  const postId = parseInt(params['id'])
  root.innerHTML = `<div class="flex items-center justify-center min-h-screen"><div class="text-gray-500">Loading...</div></div>`

  try {
    const [post, commentsRes] = await Promise.all([
      postsAPI.get(postId),
      postsAPI.comments(postId),
    ])
    root.innerHTML = ''
    mountLightboxContent(root, post, commentsRes.comments ?? [], { embedded: true })
  } catch {
    root.innerHTML = `<div class="text-center text-red-400 py-24">Post not found</div>`
  }
}

// ─── Lightbox (opened from feed / explore) ────────────────────────

export function openPostLightbox(initialPost: Post, _siblings: Post[] = []) {
  const overlay = document.createElement('div')
  // Mobile: full-screen scrollable sheet. Desktop: centred modal.
  overlay.className = 'fixed inset-0 bg-black/80 z-50 flex items-start md:items-center justify-center p-0 md:p-6 overflow-y-auto md:overflow-hidden scrollbar-none'
  overlay.style.cssText = 'scrollbar-width:none;-ms-overflow-style:none;'
  overlay.innerHTML = `<div id="lightbox-inner" class="bg-gray-950 rounded-none md:rounded-2xl w-full md:max-w-5xl md:max-h-[90vh] md:overflow-hidden flex flex-col md:flex-row relative min-h-screen md:min-h-0">
    <!-- Mobile top bar with drag handle + close -->
    <div class="md:hidden flex items-center justify-between px-4 py-3 border-b border-gray-800 flex-shrink-0">
      <div class="w-8"></div>
      <div class="w-10 h-1 bg-gray-700 rounded-full"></div>
      <button id="lb-close" class="w-8 h-8 flex items-center justify-center text-gray-400 hover:text-white bg-gray-800 rounded-full text-sm transition">✕</button>
    </div>
  </div>`

  document.body.appendChild(overlay)

  const inner = overlay.querySelector<HTMLElement>('#lightbox-inner')!
  inner.innerHTML += `<div class="text-gray-500 p-8">Loading...</div>`

  const close = (dir: 'left' | 'right' | null = null) => {
    // Animate out on mobile swipe, instant on desktop
    if (dir && window.innerWidth < 768) {
      inner.style.transition = 'transform 0.22s ease, opacity 0.22s ease'
      inner.style.transform = `translateX(${dir === 'right' ? '100%' : '-100%'})`
      inner.style.opacity = '0'
      setTimeout(() => overlay.remove(), 220)
    } else {
      overlay.remove()
    }
  }

  overlay.querySelector('#lb-close')?.addEventListener('click', () => close())
  overlay.addEventListener('click', (e) => { if (e.target === overlay) close() })
  document.addEventListener('keydown', function esc(e) {
    if (e.key === 'Escape') { close(); document.removeEventListener('keydown', esc) }
  })

  // Swipe left/right to close (mobile only)
  let touchStartX = 0
  let touchStartY = 0
  let dragging = false
  inner.addEventListener('touchstart', (e) => {
    touchStartX = e.touches[0].clientX
    touchStartY = e.touches[0].clientY
    dragging = false
  }, { passive: true })
  inner.addEventListener('touchmove', (e) => {
    const dx = e.touches[0].clientX - touchStartX
    const dy = e.touches[0].clientY - touchStartY
    // Only track horizontal swipes (not vertical scrolls)
    if (!dragging && Math.abs(dx) > Math.abs(dy) && Math.abs(dx) > 10) {
      dragging = true
    }
    if (dragging) {
      inner.style.transform = `translateX(${dx}px)`
      inner.style.opacity = String(Math.max(0, 1 - Math.abs(dx) / 300))
    }
  }, { passive: true })
  inner.addEventListener('touchend', (e) => {
    const dx = e.changedTouches[0].clientX - touchStartX
    if (dragging && Math.abs(dx) > 90) {
      close(dx > 0 ? 'right' : 'left')
    } else {
      // Snap back
      inner.style.transition = 'transform 0.18s ease, opacity 0.18s ease'
      inner.style.transform = ''
      inner.style.opacity = ''
      setTimeout(() => { inner.style.transition = '' }, 180)
    }
    dragging = false
  }, { passive: true })

  Promise.all([postsAPI.get(initialPost.id), postsAPI.comments(initialPost.id)]).then(([post, commentsRes]) => {
    inner.innerHTML = `<button id="lb-close-btn" class="absolute top-3 right-3 text-white text-xl z-20 bg-black/40 rounded-full w-9 h-9 items-center justify-center hidden md:flex">✕</button>`
    inner.querySelector('#lb-close-btn')?.addEventListener('click', close)
    mountLightboxContent(inner, post, commentsRes.comments ?? [], { embedded: false })
  }).catch(() => {
    inner.innerHTML = `<div class="text-red-400 p-8">Failed to load post</div>`
  })
}

// ─── Shared content renderer ─────────────────────────────────────

function mountLightboxContent(root: HTMLElement, post: Post, commentsList: Comment[], opts: { embedded: boolean }) {
  const me = getUser()

  const imageCol = `
  <div class="flex-shrink-0 md:w-1/2 lg:w-3/5 bg-black flex items-center">
    <img src="${post.image_path}" class="w-full max-h-[55vh] md:max-h-[90vh] object-contain" />
  </div>`

  const infoCol = `
  <div class="flex flex-col flex-1 md:overflow-hidden ${opts.embedded ? '' : 'md:max-h-[90vh]'}">
    <!-- Author header -->
    <div class="flex items-center gap-3 p-4 border-b border-gray-800 flex-shrink-0">
      <a href="/profile/${post.user?.username ?? ''}" data-link class="flex-shrink-0">${avatar(post.user, 9)}</a>
      <a href="/profile/${post.user?.username ?? ''}" data-link class="font-semibold text-sm hover:underline">${escapeHTML(post.user?.username ?? '')}</a>
      <span class="ml-auto text-xs text-gray-500">${timeAgo(post.created_at)}</span>
    </div>

    <!-- Caption + comments scrollable -->
    <div id="comments-scroll" class="flex-1 md:overflow-y-auto p-4 space-y-3 scrollbar-none" style="scrollbar-width:none;-ms-overflow-style:none;">
      ${post.caption ? `
      <div class="flex gap-3">
        ${avatar(post.user, 8)}
        <div class="text-sm"><span class="font-semibold">${escapeHTML(post.user?.username ?? '')}</span> <span class="text-gray-200">${mentionify(post.caption)}</span></div>
      </div>` : ''}
      <div id="comments-list" class="space-y-3">
        ${renderCommentItems(commentsList, me?.id ?? 0)}
      </div>
    </div>

    <!-- Like / Bookmark row -->
    <div class="p-4 border-t border-gray-800 space-y-2 flex-shrink-0">
      <div class="flex items-center gap-4">
        <button id="post-like-btn" class="flex items-center gap-1.5 text-sm ${post.is_liked ? 'text-pink-500' : 'text-gray-300 hover:text-pink-400'} transition" data-liked="${post.is_liked ?? false}">
          ${post.is_liked
            ? `<svg class="w-6 h-6 fill-current" viewBox="0 0 24 24"><path d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/></svg>`
            : `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/></svg>`}
          <span id="post-like-count" class="font-medium">${post.like_count ?? 0}</span>
        </button>
        <button id="post-bookmark-btn" class="ml-auto ${post.is_bookmarked ? 'text-white' : 'text-gray-300 hover:text-white'} transition" data-bookmarked="${post.is_bookmarked ?? false}">
          ${post.is_bookmarked
            ? `<svg class="w-6 h-6 fill-current" viewBox="0 0 24 24"><path d="M5 4a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 20V4z"/></svg>`
            : `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z"/></svg>`}
        </button>
      </div>
    </div>

    <!-- Add comment -->
    ${me ? `
    <div class="p-3 border-t border-gray-800 flex gap-2 flex-shrink-0">
      <input id="comment-input" type="text" placeholder="Add a comment..." class="flex-1 bg-gray-800 border border-gray-700 rounded-xl px-3 py-2 text-sm focus:outline-none focus:border-purple-500 transition" />
      <button id="comment-submit" class="bg-purple-600 hover:bg-purple-700 px-4 rounded-xl text-sm font-semibold transition">Post</button>
    </div>` : `
    <div class="p-3 border-t border-gray-800 text-center text-xs text-gray-500">
      <a href="/login" data-link class="text-purple-400 hover:underline">Sign in</a> to comment
    </div>`}
  </div>`

  root.insertAdjacentHTML('beforeend', `${imageCol}${infoCol}`)

  // Wire like
  root.querySelector<HTMLButtonElement>('#post-like-btn')!.addEventListener('click', async (e) => {
    const btn = e.currentTarget as HTMLButtonElement
    const isLiked = btn.dataset.liked === 'true'
    try {
      const res = isLiked ? await postsAPI.unlike(post.id) : await postsAPI.like(post.id)
      btn.dataset.liked = String(!isLiked)
      root.querySelector('#post-like-count')!.textContent = String(res.like_count)
      btn.classList.toggle('text-pink-500', !isLiked)
      btn.classList.toggle('text-gray-300', isLiked)
    } catch { /* ignore */ }
  })

  // Wire bookmark
  root.querySelector<HTMLButtonElement>('#post-bookmark-btn')!.addEventListener('click', async (e) => {
    const btn = e.currentTarget as HTMLButtonElement
    const isBookmarked = btn.dataset.bookmarked === 'true'
    try {
      isBookmarked ? await postsAPI.unbookmark(post.id) : await postsAPI.bookmark(post.id)
      const newState = !isBookmarked
      btn.dataset.bookmarked = String(newState)
      btn.classList.toggle('text-white', newState)
      btn.innerHTML = newState
        ? `<svg class="w-6 h-6 fill-current" viewBox="0 0 24 24"><path d="M5 4a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 20V4z"/></svg>`
        : `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z"/></svg>`
    } catch { /* ignore */ }
  })

  // Wire comment submit
  const commentInput = root.querySelector<HTMLInputElement>('#comment-input')
  const commentSubmit = root.querySelector<HTMLButtonElement>('#comment-submit')
  const commentsListEl = root.querySelector<HTMLElement>('#comments-list')

  const submitComment = async () => {
    const body = commentInput?.value.trim() ?? ''
    if (!body || !commentInput) return
    if (commentSubmit) commentSubmit.disabled = true
    try {
      const comment = await postsAPI.addComment(post.id, body)
      commentInput.value = ''
      commentsListEl?.insertAdjacentHTML('beforeend', renderCommentItems([comment], me?.id ?? 0))
      wireCommentActions(root, me?.id ?? 0)
      commentsListEl?.lastElementChild?.scrollIntoView({ behavior: 'smooth' })
    } catch (e) {
      alert(e instanceof APIError ? e.message : 'Failed to post comment')
    } finally {
      if (commentSubmit) commentSubmit.disabled = false
    }
  }

  commentSubmit?.addEventListener('click', submitComment)
  commentInput?.addEventListener('keydown', (e) => { if (e.key === 'Enter') submitComment() })

  wireCommentActions(root, me?.id ?? 0)
}

function renderCommentItems(list: Comment[], myID: number): string {
  return list.map(c => `
  <div class="flex gap-3 group" data-comment-id="${c.id}">
    ${avatar(c.user, 8)}
    <div class="flex-1 min-w-0">
      <p class="text-sm"><span class="font-semibold">${escapeHTML(c.user?.username ?? '')}</span> <span class="text-gray-200">${mentionify(c.body)}</span></p>
      <div class="flex items-center gap-3 mt-0.5">
        <span class="text-xs text-gray-500">${timeAgo(c.created_at)}</span>
        <button class="comment-like-btn text-xs text-gray-500 hover:text-pink-400 transition" data-comment-id="${c.id}" data-liked="${c.is_liked ?? false}">
          ${c.is_liked ? '♥' : '♡'} <span class="comment-like-count">${c.like_count ?? 0}</span>
        </button>
        ${c.user_id === myID ? `<button class="comment-delete-btn text-xs text-gray-600 hover:text-red-400 transition opacity-0 group-hover:opacity-100" data-comment-id="${c.id}">Delete</button>` : ''}
      </div>
    </div>
  </div>`).join('')
}

function wireCommentActions(root: HTMLElement, _myID: number) {
  root.querySelectorAll<HTMLButtonElement>('.comment-like-btn').forEach(btn => {
    if (btn.dataset.bound) return
    btn.dataset.bound = '1'
    btn.addEventListener('click', async () => {
      const cid = parseInt(btn.dataset.commentId!)
      const isLiked = btn.dataset.liked === 'true'
      try {
        const res = isLiked ? await commentsAPI.unlike(cid) : await commentsAPI.like(cid)
        btn.dataset.liked = String(!isLiked)
        btn.querySelector('.comment-like-count')!.textContent = String(res.like_count)
        btn.textContent = (!isLiked ? '♥ ' : '♡ ') + res.like_count
        btn.dataset.bound = ''
      } catch { /* ignore */ }
    })
  })

  root.querySelectorAll<HTMLButtonElement>('.comment-delete-btn').forEach(btn => {
    if (btn.dataset.bound) return
    btn.dataset.bound = '1'
    btn.addEventListener('click', async () => {
      const cid = parseInt(btn.dataset.commentId!)
      try {
        await commentsAPI.delete(cid)
        btn.closest('[data-comment-id]')?.remove()
      } catch { /* ignore */ }
    })
  })
}
