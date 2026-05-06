import { posts as postsAPI, users as usersAPI, hashtags as hashtagsAPI } from './api.ts'
import { navigate } from './router.ts'
import type { Post } from './types.ts'
import { renderStoryBar } from './stories.ts'
import { openPostLightbox } from './post.ts'

// ─── Feed Page ────────────────────────────────────────────────────

// Cached posts for lightbox navigation
let feedPosts: Post[] = []

export function renderFeed(root: HTMLElement) {
  root.innerHTML = `
  <div class="flex gap-8 max-w-5xl mx-auto px-4 py-4">
    <!-- Feed column -->
    <div class="flex-1 min-w-0 space-y-4 max-w-xl">
    <!-- Search bar -->
    <div class="relative">
      <svg class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-500 pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-4.35-4.35M17 11A6 6 0 1 1 5 11a6 6 0 0 1 12 0z"/></svg>
      <input id="feed-search" type="text" placeholder="Search users..." class="w-full bg-gray-900 border border-gray-800 rounded-xl pl-9 pr-4 py-2.5 text-sm focus:outline-none focus:border-purple-500 transition placeholder-gray-600" />
      <div id="search-results" class="absolute top-full mt-1 left-0 right-0 bg-gray-900 border border-gray-800 rounded-xl overflow-hidden z-30 hidden shadow-xl"></div>
    </div>
    <!-- Story bar -->
    <div id="story-bar" class="py-1"></div>
    <!-- Posts -->
    <div id="feed-container" class="space-y-6">
      ${skeleton()}${skeleton()}
    </div>
      <div id="load-more-wrap" class="hidden text-center py-4">
        <button id="load-more-btn" class="text-purple-400 hover:text-purple-300 text-sm font-medium">Load more</button>
      </div>
    </div>

    <!-- Right sidebar (desktop only) — sticky so it stays in view while feed scrolls -->
    <aside class="hidden xl:flex flex-col gap-5 w-72 flex-shrink-0 pt-1 sticky top-4 self-start">
      <div id="sidebar-suggestions" class="bg-gray-900 border border-gray-800 rounded-2xl p-4 space-y-3">
        <h3 class="text-sm font-semibold text-gray-300">Suggested for you</h3>
        <div id="suggestions-list" class="space-y-3">
          <div class="h-10 bg-gray-800 rounded-xl animate-pulse"></div>
          <div class="h-10 bg-gray-800 rounded-xl animate-pulse"></div>
          <div class="h-10 bg-gray-800 rounded-xl animate-pulse"></div>
        </div>
      </div>
      <div class="bg-gray-900 border border-gray-800 rounded-2xl p-4 space-y-3">
        <h3 class="text-sm font-semibold text-gray-300">Trending</h3>
        <div id="trending-tags" class="space-y-2">
          <div class="h-4 bg-gray-800 rounded animate-pulse"></div>
          <div class="h-4 bg-gray-800 rounded animate-pulse"></div>
          <div class="h-4 bg-gray-800 rounded animate-pulse"></div>
        </div>
      </div>
    </aside>
  </div>`

  let page = 1
  renderStoryBar(root.querySelector('#story-bar')!)
  loadFeed(1)
  loadSuggestions(root)
  loadTrending(root)
  wireSearch(root)

  root.querySelector('#load-more-btn')?.addEventListener('click', () => {
    page++
    loadFeed(page)
  })

  async function loadFeed(p: number) {
    const container = root.querySelector<HTMLDivElement>('#feed-container')!
    if (p === 1) { container.innerHTML = skeleton() + skeleton(); feedPosts = [] }
    try {
      const { posts } = await postsAPI.feed(p)
      if (p === 1) container.innerHTML = ''
      if (posts.length === 0 && p === 1) {
        container.innerHTML = emptyState('No posts yet — follow someone or create a post!')
        return
      }
      feedPosts = [...feedPosts, ...posts]
      posts.forEach(post => container.insertAdjacentHTML('beforeend', renderPostCard(post)))

      // When feed runs dry, surface suggested posts from explore
      if (posts.length < 20) {
        root.querySelector('#load-more-wrap')!.classList.add('hidden')
        if (p === 1 || posts.length === 0) {
          loadSuggestedPosts(container, feedPosts)
        }
      } else {
        root.querySelector('#load-more-wrap')!.classList.remove('hidden')
      }
      wirePostActions(root, feedPosts)
    } catch {
      if (p === 1) container.innerHTML = errorState('Failed to load feed')
    }
  }

  async function loadSuggestedPosts(container: HTMLDivElement, existing: Post[]) {
    try {
      const { posts: suggested } = await postsAPI.explore(1)
      const existingIDs = new Set(existing.map(p => p.id))
      const fresh = suggested.filter(p => !existingIDs.has(p.id))
      if (!fresh.length) return
      container.insertAdjacentHTML('beforeend', `
        <div class="flex items-center gap-3 py-4">
          <div class="flex-1 h-px bg-gray-800"></div>
          <span class="text-xs text-gray-500 font-medium">Suggested for you</span>
          <div class="flex-1 h-px bg-gray-800"></div>
        </div>`)
      fresh.slice(0, 6).forEach(post => container.insertAdjacentHTML('beforeend', renderPostCard(post)))
      feedPosts = [...feedPosts, ...fresh]
      wirePostActions(root, feedPosts)
    } catch { /* non-critical */ }
  }
}

async function loadSuggestions(root: HTMLElement) {
  const list = root.querySelector<HTMLElement>('#suggestions-list')
  if (!list) return
  try {
    const { users } = await usersAPI.suggestions()
    const suggestions = users.slice(0, 5)
    if (!suggestions.length) { list.innerHTML = `<p class="text-xs text-gray-500">No suggestions yet</p>`; return }
    list.innerHTML = suggestions.map(u => `
      <div class="flex items-center gap-3">
        <a href="/profile/${u.username}" data-link class="flex-shrink-0">${avatar(u, 9)}</a>
        <div class="flex-1 min-w-0">
          <a href="/profile/${u.username}" data-link class="text-sm font-semibold hover:underline truncate block">${escapeHTML(u.username)}</a>
          ${u.bio ? `<p class="text-xs text-gray-500 truncate">${escapeHTML(u.bio.slice(0, 40))}</p>` : ''}
        </div>
        <button class="suggest-follow-btn text-xs font-semibold text-purple-400 hover:text-purple-300 transition flex-shrink-0" data-username="${u.username}">Follow</button>
      </div>`).join('')
    list.querySelectorAll<HTMLButtonElement>('.suggest-follow-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        await usersAPI.follow(btn.dataset.username!)
        btn.textContent = 'Following'
        btn.classList.replace('text-purple-400', 'text-gray-500')
        btn.disabled = true
      })
    })
  } catch { /* non-critical */ }
}

async function loadTrending(root: HTMLElement) {
  const el = root.querySelector<HTMLElement>('#trending-tags')
  if (!el) return
  try {
    const { hashtags } = await hashtagsAPI.trending()
    if (!hashtags.length) {
      el.innerHTML = `<p class="text-xs text-gray-500">No trending tags yet</p>`
      return
    }
    el.innerHTML = hashtags.slice(0, 6).map(h => `
      <button class="w-full flex items-center justify-between group trending-tag-btn" data-tag="${escapeHTML(h.tag)}">
        <div class="text-left">
          <div class="text-sm font-medium text-sky-400 group-hover:text-sky-300 transition">#${escapeHTML(h.tag)}</div>
          <div class="text-xs text-gray-500">${h.post_count} post${h.post_count === 1 ? '' : 's'}</div>
        </div>
        <svg class="w-4 h-4 text-gray-600 group-hover:text-gray-400 transition" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
      </button>`).join('')
    el.querySelectorAll<HTMLButtonElement>('.trending-tag-btn').forEach(btn => {
      btn.addEventListener('click', () => navigate('/explore?tag=' + encodeURIComponent(btn.dataset.tag!)))
    })
  } catch { /* non-critical */ }
}

function wireSearch(root: HTMLElement) {
  const input = root.querySelector<HTMLInputElement>('#feed-search')!
  const results = root.querySelector<HTMLElement>('#search-results')!
  let t: ReturnType<typeof setTimeout>

  input.addEventListener('input', () => {
    clearTimeout(t)
    const q = input.value.trim()
    if (q.length < 2) { results.classList.add('hidden'); results.innerHTML = ''; return }
    t = setTimeout(async () => {
      try {
        const { users } = await usersAPI.search(q)
        if (!users.length) {
          results.innerHTML = `<div class="px-4 py-3 text-sm text-gray-500">No users found</div>`
        } else {
          results.innerHTML = users.slice(0, 6).map(u => `
            <button class="search-user-btn w-full flex items-center gap-3 px-4 py-2.5 hover:bg-gray-800 transition text-left" data-username="${u.username}">
              ${avatar(u, 9)}
              <div class="min-w-0">
                <div class="text-sm font-semibold truncate">${escapeHTML(u.username)}</div>
                ${u.bio ? `<div class="text-xs text-gray-500 truncate">${escapeHTML(u.bio)}</div>` : ''}
              </div>
            </button>`).join('')
        }
        results.classList.remove('hidden')
        results.querySelectorAll<HTMLButtonElement>('.search-user-btn').forEach(btn => {
          btn.addEventListener('click', () => {
            results.classList.add('hidden')
            input.value = ''
            navigate(`/profile/${btn.dataset.username}`)
          })
        })
      } catch { /* ignore */ }
    }, 280)
  })

  // Hide on click outside
  document.addEventListener('click', (e) => {
    if (!root.contains(e.target as Node)) results.classList.add('hidden')
  })
  input.addEventListener('blur', () => setTimeout(() => results.classList.add('hidden'), 150))
}

// ─── Post Card ────────────────────────────────────────────────────

export function renderPostCard(post: Post): string {
  const avatar = post.user?.avatar_path
    ? `<img src="${post.user.avatar_path}" class="w-9 h-9 rounded-full object-cover flex-shrink-0" />`
    : `<div class="w-9 h-9 rounded-full bg-gradient-to-br from-purple-500 to-pink-500 flex items-center justify-center text-sm font-bold flex-shrink-0">${(post.user?.username ?? '?')[0].toUpperCase()}</div>`

  const likeIcon = post.is_liked
    ? `<svg class="w-6 h-6 fill-current text-pink-500" viewBox="0 0 24 24"><path d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/></svg>`
    : `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/></svg>`

  const bookmarkIcon = post.is_bookmarked
    ? `<svg class="w-6 h-6 fill-current" viewBox="0 0 24 24"><path d="M5 4a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 20V4z"/></svg>`
    : `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z"/></svg>`

  // Build image column: carousel if multiple images, single if one
  const allImages = post.images && post.images.length > 1
    ? post.images.map(i => i.image_path)
    : [post.image_path]
  const isCarousel = allImages.length > 1

  const imageSection = isCarousel
    ? `<div class="relative post-img-wrap" data-post-id="${post.id}">
        <div class="carousel-track overflow-hidden">
          <div class="carousel-slides flex transition-transform duration-300" style="width:${allImages.length * 100}%">
            ${allImages.map(src => `<div style="width:${100 / allImages.length}%"><img src="${src}" alt="post" class="w-full object-cover max-h-[600px]" loading="lazy" /></div>`).join('')}
          </div>
        </div>
        <button class="carousel-prev absolute left-2 top-1/2 -translate-y-1/2 bg-black/50 hover:bg-black/70 rounded-full w-8 h-8 flex items-center justify-center text-white transition z-10">‹</button>
        <button class="carousel-next absolute right-2 top-1/2 -translate-y-1/2 bg-black/50 hover:bg-black/70 rounded-full w-8 h-8 flex items-center justify-center text-white transition z-10">›</button>
        <div class="carousel-dots absolute bottom-2 left-1/2 -translate-x-1/2 flex gap-1">
          ${allImages.map((_, i) => `<span class="carousel-dot w-1.5 h-1.5 rounded-full ${i === 0 ? 'bg-white' : 'bg-white/40'}" data-idx="${i}"></span>`).join('')}
        </div>
      </div>`
    : `<div class="relative group cursor-pointer post-img-wrap" data-post-id="${post.id}">
        <img src="${post.image_path}" alt="post" class="w-full object-cover max-h-[600px]" loading="lazy" />
      </div>`

  return `
  <article class="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden" data-post-id="${post.id}">
    <div class="flex items-center gap-3 p-3 px-4">
      <a href="/profile/${post.user?.username ?? ''}" data-link class="flex-shrink-0">${avatar}</a>
      <div class="flex-1 min-w-0">
        <a href="/profile/${post.user?.username ?? ''}" data-link class="font-semibold text-sm hover:underline">${post.user?.username ?? 'unknown'}</a>
      </div>
      ${isCarousel ? `<span class="text-xs text-gray-500 flex-shrink-0">1/${allImages.length}</span>` : ''}
      <span class="text-xs text-gray-500 flex-shrink-0">${timeAgo(post.created_at)}</span>
    </div>
    ${imageSection}
    <div class="p-4 space-y-2">
      <div class="flex items-center gap-3">
        <button class="like-btn flex items-center gap-1.5 text-sm ${post.is_liked ? 'text-pink-500' : 'text-gray-300 hover:text-pink-400'} transition" data-post-id="${post.id}" data-liked="${post.is_liked ?? false}">
          ${likeIcon}
          <span class="like-count font-medium">${post.like_count ?? 0}</span>
        </button>
        <button class="comment-btn flex items-center gap-1.5 text-sm text-gray-300 hover:text-white transition" data-post-id="${post.id}">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/></svg>
          ${post.comment_count ? `<span class="text-sm font-medium">${post.comment_count}</span>` : ''}
        </button>
        <button class="bookmark-btn ml-auto text-sm ${post.is_bookmarked ? 'text-white' : 'text-gray-300 hover:text-white'} transition" data-post-id="${post.id}" data-bookmarked="${post.is_bookmarked ?? false}">
          ${bookmarkIcon}
        </button>
      </div>
      ${post.caption ? `<p class="text-sm leading-relaxed"><a href="/profile/${post.user?.username ?? ''}" data-link class="font-semibold hover:underline">${post.user?.username ?? ''}</a> ${mentionify(post.caption)}</p>` : ''}
    </div>
  </article>`
}

export function wirePostActions(root: HTMLElement, posts: Post[] = []) {
  const postMap = new Map(posts.map(p => [p.id, p]))

  // Carousel controls
  root.querySelectorAll<HTMLElement>('.carousel-track').forEach(track => {
    if (track.dataset.bound) return
    track.dataset.bound = '1'
    const article = track.closest('article')!
    const slides = track.querySelector<HTMLElement>('.carousel-slides')!
    const dots = article.querySelectorAll<HTMLElement>('.carousel-dot')
    let current = 0
    const total = dots.length

    const goTo = (idx: number) => {
      current = Math.max(0, Math.min(idx, total - 1))
      slides.style.transform = `translateX(-${current * (100 / total)}%)`
      dots.forEach((d, i) => {
        d.classList.toggle('bg-white', i === current)
        d.classList.toggle('bg-white/40', i !== current)
      })
      // Update counter in header
      const counter = article.querySelector<HTMLElement>('.text-xs.text-gray-500')
      if (counter && counter.textContent?.includes('/')) counter.textContent = `${current + 1}/${total}`
    }

    article.querySelector('.carousel-prev')?.addEventListener('click', (e) => {
      e.stopPropagation()
      goTo(current - 1)
    })
    article.querySelector('.carousel-next')?.addEventListener('click', (e) => {
      e.stopPropagation()
      goTo(current + 1)
    })
  })

  // Image click → lightbox modal; double-tap → like
  root.querySelectorAll<HTMLElement>('.post-img-wrap').forEach(el => {
    if (el.dataset.bound) return
    el.dataset.bound = '1'
    let lastTap = 0
    el.addEventListener('touchend', () => {
      const now = Date.now()
      if (now - lastTap < 300) quickLike(root, parseInt(el.dataset.postId!))
      lastTap = now
    })
    el.addEventListener('click', () => {
      const postID = parseInt(el.dataset.postId!)
      const post = postMap.get(postID)
      if (post) openPostLightbox(post, posts)
      else openPostLightbox({ id: postID } as Post, posts)
    })
  })

  // Comment button → lightbox
  root.querySelectorAll<HTMLButtonElement>('.comment-btn').forEach(btn => {
    if (btn.dataset.bound) return
    btn.dataset.bound = '1'
    btn.addEventListener('click', () => {
      const postID = parseInt(btn.dataset.postId!)
      const post = postMap.get(postID)
      if (post) openPostLightbox(post, posts)
      else openPostLightbox({ id: postID } as Post, posts)
    })
  })

  // Like button
  root.querySelectorAll<HTMLButtonElement>('.like-btn').forEach(btn => {
    if (btn.dataset.bound) return
    btn.dataset.bound = '1'
    btn.addEventListener('click', async () => {
      const postID = parseInt(btn.dataset.postId!)
      const isLiked = btn.dataset.liked === 'true'
      try {
        const res = isLiked ? await postsAPI.unlike(postID) : await postsAPI.like(postID)
        setLikeState(btn, !isLiked, res.like_count)
      } catch { /* ignore */ }
    })
  })

  // Bookmark button
  root.querySelectorAll<HTMLButtonElement>('.bookmark-btn').forEach(btn => {
    if (btn.dataset.bound) return
    btn.dataset.bound = '1'
    btn.addEventListener('click', async () => {
      const postID = parseInt(btn.dataset.postId!)
      const isBookmarked = btn.dataset.bookmarked === 'true'
      try {
        isBookmarked ? await postsAPI.unbookmark(postID) : await postsAPI.bookmark(postID)
        const newState = !isBookmarked
        btn.dataset.bookmarked = String(newState)
        btn.classList.toggle('text-white', newState)
        btn.innerHTML = newState
          ? `<svg class="w-6 h-6 fill-current" viewBox="0 0 24 24"><path d="M5 4a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 20V4z"/></svg>`
          : `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 5a2 2 0 012-2h10a2 2 0 012 2v16l-7-3.5L5 21V5z"/></svg>`
      } catch { /* ignore */ }
    })
  })
}

async function quickLike(root: HTMLElement, postID: number) {
  const btn = root.querySelector<HTMLButtonElement>(`.like-btn[data-post-id="${postID}"]`)
  if (!btn || btn.dataset.liked === 'true') return
  try {
    const res = await postsAPI.like(postID)
    setLikeState(btn, true, res.like_count)
  } catch { /* ignore */ }
}

function setLikeState(btn: HTMLButtonElement, liked: boolean, count: number) {
  btn.dataset.liked = String(liked)
  btn.querySelector('.like-count')!.textContent = String(count)
  btn.classList.toggle('text-pink-500', liked)
  btn.classList.toggle('text-gray-300', !liked)
  btn.innerHTML = liked
    ? `<svg class="w-6 h-6 fill-current text-pink-500" viewBox="0 0 24 24"><path d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/></svg><span class="like-count font-medium">${count}</span>`
    : `<svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4.318 6.318a4.5 4.5 0 000 6.364L12 20.364l7.682-7.682a4.5 4.5 0 00-6.364-6.364L12 7.636l-1.318-1.318a4.5 4.5 0 00-6.364 0z"/></svg><span class="like-count font-medium">${count}</span>`
}

// ─── Shared helpers ───────────────────────────────────────────────

export function skeleton(): string {
  return `
  <div class="bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden animate-pulse">
    <div class="flex items-center gap-3 p-4">
      <div class="w-9 h-9 rounded-full bg-gray-800"></div>
      <div class="h-3 bg-gray-800 rounded w-24"></div>
    </div>
    <div class="h-72 bg-gray-800"></div>
    <div class="p-4 space-y-2">
      <div class="h-3 bg-gray-800 rounded w-16"></div>
      <div class="h-3 bg-gray-800 rounded w-40"></div>
    </div>
  </div>`
}

export function emptyState(msg: string): string {
  return `<div class="text-center text-gray-500 py-16 text-sm">${msg}</div>`
}

export function errorState(msg: string): string {
  return `<div class="text-center text-red-400 py-16 text-sm">${msg}</div>`
}

export function avatar(user: { username: string; avatar_path?: string } | undefined, size = 9): string {
  // size is in Tailwind units (1 unit = 4px). Use inline styles so the value
  // isn't tree-shaken by Tailwind's static scanner.
  const px = size * 4
  const dim = `width:${px}px;height:${px}px;min-width:${px}px;`
  const fontSize = px <= 28 ? '10px' : px <= 40 ? '13px' : '16px'
  return user?.avatar_path
    ? `<img src="${user.avatar_path}" style="${dim}" class="rounded-full object-cover flex-shrink-0" />`
    : `<div style="${dim};font-size:${fontSize}" class="rounded-full bg-gradient-to-br from-purple-500 to-pink-500 flex items-center justify-center font-bold flex-shrink-0">${(user?.username ?? '?')[0].toUpperCase()}</div>`
}

export function timeAgo(iso: string): string {
  const diff = (Date.now() - new Date(iso).getTime()) / 1000
  if (diff < 60) return 'just now'
  if (diff < 3600) return `${Math.floor(diff / 60)}m`
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`
  if (diff < 86400 * 7) return `${Math.floor(diff / 86400)}d`
  return new Date(iso).toLocaleDateString()
}

export function escapeHTML(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

export function mentionify(text: string): string {
  return escapeHTML(text)
    .replace(/@(\w+)/g, '<span data-mention="$1" class="text-purple-400 font-medium hover:underline cursor-pointer">@$1</span>')
    .replace(/#(\w+)/g, '<span data-hashtag="$1" class="text-sky-400 font-medium hover:underline cursor-pointer">#$1</span>')
}
