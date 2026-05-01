import { posts as postsAPI, users as usersAPI } from './api.ts'
import { navigate } from './router.ts'
import type { Post } from './types.ts'
import { avatar, escapeHTML } from './feed.ts'
import { openPostLightbox } from './post.ts'

export async function renderExplore(root: HTMLElement) {
  // Pick up ?tag= from URL so clicking a hashtag pre-fills search
  const tagParam = new URLSearchParams(location.search).get('tag') ?? ''

  root.innerHTML = `
  <div class="max-w-4xl mx-auto px-4 py-6">
    <div class="relative mb-6">
      <svg class="absolute left-4 top-1/2 -translate-y-1/2 w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
      <input id="search-input" type="text" placeholder="Search people or #hashtags..." value="${escapeHTML(tagParam ? '#' + tagParam : '')}" class="w-full bg-gray-900 border border-gray-800 rounded-2xl pl-12 pr-4 py-3 text-sm focus:outline-none focus:border-purple-500 transition" />
    </div>
    <div id="search-results" class="hidden mb-6 bg-gray-900 border border-gray-800 rounded-2xl overflow-hidden"></div>
    <div id="explore-grid" class="grid grid-cols-3 gap-1 md:gap-2">
      ${gridSkeleton()}
    </div>
    <div id="load-more-wrap" class="hidden text-center py-6">
      <button id="load-more-btn" class="text-purple-400 hover:text-purple-300 text-sm">Load more</button>
    </div>
  </div>`

  let page = 1
  loadGrid(1)

  // Search
  let searchTimer: ReturnType<typeof setTimeout>
  const searchInput = root.querySelector<HTMLInputElement>('#search-input')!
  searchInput.addEventListener('input', (e) => {
    clearTimeout(searchTimer)
    const q = (e.target as HTMLInputElement).value.trim()
    const resultsEl = root.querySelector<HTMLDivElement>('#search-results')!
    if (q.length < 2) { resultsEl.classList.add('hidden'); return }
    searchTimer = setTimeout(() => doSearch(q.replace(/^#/, ''), resultsEl), 300)
  })

  root.querySelector('#load-more-btn')?.addEventListener('click', () => {
    page++
    loadGrid(page)
  })

  async function loadGrid(p: number) {
    const grid = root.querySelector<HTMLDivElement>('#explore-grid')!
    if (p === 1) grid.innerHTML = gridSkeleton()
    try {
      const { posts } = await postsAPI.explore(p)
      if (p === 1) grid.innerHTML = ''
      if (posts.length === 0 && p === 1) {
        grid.innerHTML = `<div class="col-span-3 text-center text-gray-500 py-16 text-sm">No posts yet</div>`
        return
      }
      posts.forEach(post => {
        const cell = document.createElement('div')
        cell.className = 'relative aspect-square overflow-hidden bg-gray-900 cursor-pointer group'
        cell.innerHTML = `
          <img src="${post.image_path}" class="w-full h-full object-cover group-hover:scale-105 transition duration-300" loading="lazy" />
          <div class="absolute inset-0 bg-black/0 group-hover:bg-black/40 transition flex items-center justify-center opacity-0 group-hover:opacity-100">
            <div class="flex items-center gap-3 text-white text-sm font-semibold">
              <span>♥ ${post.like_count ?? 0}</span>
            </div>
          </div>`
        cell.addEventListener('click', () => openPostLightbox(post, posts))
        grid.appendChild(cell)
      })
      root.querySelector('#load-more-wrap')!.classList.toggle('hidden', posts.length < 30)
    } catch {
      if (p === 1) grid.innerHTML = `<div class="col-span-3 text-center text-red-400 py-16 text-sm">Failed to load</div>`
    }
  }
}

async function doSearch(q: string, resultsEl: HTMLDivElement) {
  resultsEl.innerHTML = `<div class="px-4 py-3 text-gray-500 text-sm">Searching...</div>`
  resultsEl.classList.remove('hidden')
  try {
    const { users } = await usersAPI.search(q)
    if (users.length === 0) {
      resultsEl.innerHTML = `<div class="px-4 py-3 text-gray-500 text-sm">No results for "${escapeHTML(q)}"</div>`
      return
    }
    resultsEl.innerHTML = users.map(u => `
      <button class="w-full flex items-center gap-3 px-4 py-3 hover:bg-gray-800 transition text-left user-result" data-username="${u.username}">
        ${avatar(u, 10)}
        <div>
          <div class="font-semibold text-sm">${escapeHTML(u.username)}</div>
          ${u.bio ? `<div class="text-xs text-gray-400 truncate max-w-xs">${escapeHTML(u.bio)}</div>` : ''}
        </div>
      </button>`).join('')

    resultsEl.querySelectorAll<HTMLButtonElement>('.user-result').forEach(btn => {
      btn.addEventListener('click', () => {
        resultsEl.classList.add('hidden')
        navigate(`/profile/${btn.dataset.username}`)
      })
    })
  } catch {
    resultsEl.innerHTML = `<div class="px-4 py-3 text-red-400 text-sm">Search failed</div>`
  }
}

function gridSkeleton(): string {
  return Array(9).fill(0).map(() =>
    `<div class="aspect-square bg-gray-900 animate-pulse rounded-sm"></div>`
  ).join('')
}
