import { users, bookmarks as bookmarksAPI, highlights as highlightsAPI, APIError } from './api.ts'
import { getUser } from './auth.ts'
import { navigate } from './router.ts'
import type { User, Post, Highlight } from './types.ts'
import { avatar, escapeHTML, skeleton, errorState } from './feed.ts'
import { openPostLightbox } from './post.ts'

export async function renderProfile(root: HTMLElement, params: Record<string, string>) {
  const username = params['username']
  const me = getUser()

  root.innerHTML = `
  <div class="max-w-3xl mx-auto px-4 py-6">
    <div id="profile-header" class="mb-4">${headerSkeleton()}</div>
    <div id="highlights-bar" class="mb-4 hidden"></div>
    <div id="profile-tabs" class="hidden border-b border-gray-800 flex gap-8 mb-4 text-sm font-semibold"></div>
    <div id="profile-grid"></div>
  </div>`

  try {
    const isOwnProfile = me?.username === username
    const [user, postsRes] = await Promise.all([
      users.get(username),
      users.posts(username),
    ])

    // ─── Profile Header ───────────────────────────────────────────
    // Use post_count from API if available, else fall back to posts length
    const postCount = user.post_count ?? postsRes.posts.length
    root.querySelector('#profile-header')!.innerHTML = buildHeader(user, postCount, isOwnProfile)
    wireHeader(root, user, postsRes.posts, isOwnProfile, me)

    // ─── Highlights Bar ───────────────────────────────────────────
    loadHighlights(root, username, isOwnProfile)

    // ─── Tabs ─────────────────────────────────────────────────────
    const tabsEl = root.querySelector<HTMLElement>('#profile-tabs')!
    tabsEl.classList.remove('hidden')
    tabsEl.innerHTML = `
      <button class="tab-btn pb-3 border-b-2 border-white text-white" data-tab="posts">Posts</button>
      ${isOwnProfile ? `<button class="tab-btn pb-3 border-b-2 border-transparent text-gray-500 hover:text-white transition" data-tab="saved">Saved</button>` : ''}
    `

    // ─── Grid ─────────────────────────────────────────────────────
    renderGrid(root, postsRes.posts, postsRes.posts, isOwnProfile, username)

    tabsEl.querySelectorAll<HTMLButtonElement>('.tab-btn').forEach(btn => {
      btn.addEventListener('click', async () => {
        tabsEl.querySelectorAll('.tab-btn').forEach(b => {
          b.classList.remove('border-white', 'text-white')
          b.classList.add('border-transparent', 'text-gray-500')
        })
        btn.classList.add('border-white', 'text-white')
        btn.classList.remove('border-transparent', 'text-gray-500')

        if (btn.dataset.tab === 'saved') {
          root.querySelector('#profile-grid')!.innerHTML = skeleton() + skeleton() + skeleton()
          try {
            const res = await bookmarksAPI.list()
            renderGrid(root, res.posts, postsRes.posts, isOwnProfile, username)
          } catch {
            root.querySelector('#profile-grid')!.innerHTML = errorState('Failed to load saved posts')
          }
        } else {
          renderGrid(root, postsRes.posts, postsRes.posts, isOwnProfile, username)
        }
      })
    })

  } catch {
    root.querySelector('#profile-header')!.innerHTML = errorState('Profile not found')
  }
}

async function loadHighlights(root: HTMLElement, username: string, isOwnProfile: boolean) {
  const bar = root.querySelector<HTMLElement>('#highlights-bar')!
  try {
    const { highlights } = await highlightsAPI.list(username)
    if (!highlights.length && !isOwnProfile) return
    bar.classList.remove('hidden')

    bar.innerHTML = `<div class="flex gap-4 overflow-x-auto pb-1 scrollbar-none" style="scrollbar-width:none"></div>`
    const row = bar.querySelector('div')!

    highlights.forEach(hl => {
      const btn = document.createElement('button')
      btn.className = 'flex flex-col items-center gap-1.5 flex-shrink-0 focus:outline-none'
      const imgSrc = hl.cover_image_path
      btn.innerHTML = `
        <div style="width:64px;height:64px;border-radius:50%;background:#1f2937;display:flex;align-items:center;justify-content:center;overflow:hidden;border:2px solid #374151;">
          ${imgSrc
            ? `<img src="${imgSrc}" style="width:64px;height:64px;object-fit:cover;border-radius:50%;" />`
            : `<div style="width:100%;height:100%;background:linear-gradient(135deg,#7c3aed,#db2777);border-radius:50%;"></div>`}
        </div>
        <span class="text-xs text-gray-400 truncate text-center" style="max-width:64px;">${escapeHTML(hl.title)}</span>`
      btn.addEventListener('click', () => openHighlightViewer(hl))
      row.appendChild(btn)
    })

    // Add "New" button for own profile
    if (isOwnProfile) {
      const addBtn = document.createElement('button')
      addBtn.className = 'flex flex-col items-center gap-1.5 flex-shrink-0 focus:outline-none'
      addBtn.innerHTML = `
        <div style="width:64px;height:64px;border-radius:50%;border:2px dashed #374151;display:flex;align-items:center;justify-content:center;">
          <svg class="w-6 h-6 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
        </div>
        <span class="text-xs text-gray-500" style="max-width:64px;">New</span>`
      addBtn.addEventListener('click', () => openNewHighlightModal())
      row.appendChild(addBtn)
    }
  } catch { /* non-critical */ }
}

function openHighlightViewer(hl: Highlight) {
  if (!hl.stories?.length) return
  const stories = hl.stories
  let idx = 0

  const overlay = document.createElement('div')
  overlay.className = 'fixed inset-0 bg-black z-50 flex items-center justify-center'
  overlay.innerHTML = `
    <div class="relative w-full max-w-sm h-full max-h-[95vh] flex flex-col">
      <div class="flex items-center gap-3 px-3 py-2 z-10">
        <div class="w-8 h-8 rounded-full bg-purple-600 flex items-center justify-center text-xs font-bold">${escapeHTML(hl.title[0].toUpperCase())}</div>
        <span class="font-semibold text-sm">${escapeHTML(hl.title)}</span>
        <div class="flex-1 ml-2 flex gap-1" id="hl-dots"></div>
      </div>
      <div class="flex-1 flex items-center justify-center overflow-hidden rounded-2xl bg-gray-900">
        <img id="hl-img" class="w-full h-full object-contain" />
      </div>
      <div class="absolute inset-0 flex" style="top:3rem;">
        <div id="hl-prev" class="flex-1 cursor-pointer"></div>
        <div id="hl-next" class="flex-1 cursor-pointer"></div>
      </div>
      <button id="hl-close" class="absolute top-3 right-3 text-white text-2xl z-20">✕</button>
    </div>`

  document.body.appendChild(overlay)

  const show = (i: number) => {
    idx = Math.max(0, Math.min(i, stories.length - 1))
    ;(overlay.querySelector<HTMLImageElement>('#hl-img')!).src = stories[idx].image_path
    overlay.querySelector('#hl-dots')!.innerHTML = stories.map((_, j) =>
      `<div class="flex-1 h-0.5 rounded-full ${j <= idx ? 'bg-white' : 'bg-gray-600'}"></div>`
    ).join('')
  }

  overlay.querySelector('#hl-close')!.addEventListener('click', () => overlay.remove())
  overlay.querySelector('#hl-prev')!.addEventListener('click', () => { if (idx > 0) show(idx - 1); else overlay.remove() })
  overlay.querySelector('#hl-next')!.addEventListener('click', () => { if (idx < stories.length - 1) show(idx + 1); else overlay.remove() })
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })
  document.addEventListener('keydown', function esc(e) {
    if (e.key === 'Escape') { overlay.remove(); document.removeEventListener('keydown', esc) }
  })

  show(0)
}

function openNewHighlightModal() {
  const overlay = document.createElement('div')
  overlay.className = 'fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4'
  overlay.innerHTML = `
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-5 w-full max-w-sm space-y-3">
      <div class="flex items-center justify-between">
        <h3 class="font-semibold">New Highlight</h3>
        <button id="nh-close" class="text-gray-400 hover:text-white">✕</button>
      </div>
      <p class="text-xs text-gray-500">Give your highlight a name. You can add stories to it from the story viewer.</p>
      <input id="nh-title" type="text" placeholder="e.g. Travel, Food, Friends..." maxlength="100"
        class="w-full bg-gray-800 border border-gray-700 rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-purple-500 transition" />
      <div id="nh-error" class="hidden text-red-400 text-sm"></div>
      <div class="flex gap-3">
        <button id="nh-cancel" class="flex-1 border border-gray-700 rounded-xl py-2.5 text-sm hover:bg-gray-800 transition">Cancel</button>
        <button id="nh-save" class="flex-1 bg-purple-600 hover:bg-purple-700 rounded-xl py-2.5 text-sm font-semibold transition">Create</button>
      </div>
    </div>`
  document.body.appendChild(overlay)

  const close = () => overlay.remove()
  overlay.querySelector('#nh-close')!.addEventListener('click', close)
  overlay.querySelector('#nh-cancel')!.addEventListener('click', close)
  overlay.addEventListener('click', (e) => { if (e.target === overlay) close() })

  overlay.querySelector('#nh-save')!.addEventListener('click', async () => {
    const title = (overlay.querySelector<HTMLInputElement>('#nh-title')!).value.trim()
    const errEl = overlay.querySelector<HTMLElement>('#nh-error')!
    if (!title) { errEl.textContent = 'Title is required'; errEl.classList.remove('hidden'); return }
    const btn = overlay.querySelector<HTMLButtonElement>('#nh-save')!
    btn.disabled = true; btn.textContent = 'Creating...'
    try {
      await highlightsAPI.create(title)
      close()
      window.location.reload()
    } catch (e: any) {
      errEl.textContent = e?.message ?? 'Failed'
      errEl.classList.remove('hidden')
      btn.disabled = false; btn.textContent = 'Create'
    }
  })
}

function buildHeader(user: User, postCount: number, isOwnProfile: boolean): string {
  const av = user.avatar_path
    ? `<img src="${user.avatar_path}" class="w-24 h-24 md:w-36 md:h-36 rounded-full object-cover ring-4 ring-gray-800" />`
    : `<div class="w-24 h-24 md:w-36 md:h-36 rounded-full bg-gradient-to-br from-purple-500 to-pink-500 flex items-center justify-center text-4xl font-bold ring-4 ring-gray-800">${user.username[0].toUpperCase()}</div>`

  const actionBtn = isOwnProfile
    ? `<button id="edit-profile-btn" class="border border-gray-600 px-5 py-1.5 rounded-xl text-sm font-semibold hover:bg-gray-800 transition">Edit profile</button>`
    : user.is_following
      ? `<button id="follow-btn" data-following="true" class="bg-gray-700 px-5 py-1.5 rounded-xl text-sm font-semibold hover:bg-gray-600 transition">Following</button>`
      : `<button id="follow-btn" data-following="false" class="bg-purple-600 px-5 py-1.5 rounded-xl text-sm font-semibold hover:bg-purple-700 transition">Follow</button>`

  const dmBtn = !isOwnProfile
    ? `<button id="dm-btn" class="border border-gray-600 px-5 py-1.5 rounded-xl text-sm font-semibold hover:bg-gray-800 transition ml-2">Message</button>`
    : ''

  return `
  <div class="flex items-start gap-8 md:gap-16">
    <div class="relative flex-shrink-0">
      ${av}
      ${isOwnProfile ? `<button id="avatar-btn" class="absolute inset-0 rounded-full flex items-center justify-center bg-black/0 hover:bg-black/40 transition">
        <span class="opacity-0 hover:opacity-100 text-white text-xs font-semibold">Change</span>
      </button>
      <input type="file" id="avatar-file" accept="image/*" class="hidden" />` : ''}
    </div>
    <div class="flex-1 space-y-4">
      <div class="flex flex-wrap items-center gap-3">
        <h1 class="text-xl font-bold">${escapeHTML(user.username)}</h1>
        ${actionBtn}${dmBtn}
      </div>
      <div class="flex gap-8 text-sm">
        <span><strong>${postCount}</strong> posts</span>
        <button id="followers-btn" class="hover:text-gray-300 transition"><strong>${user.follower_count ?? 0}</strong> followers</button>
        <button id="following-btn" class="hover:text-gray-300 transition"><strong>${user.following_count ?? 0}</strong> following</button>
      </div>
      ${user.bio ? `<p class="text-sm text-gray-200 leading-relaxed">${escapeHTML(user.bio)}</p>` : ''}
      ${user.website ? `<a href="${escapeHTML(user.website)}" target="_blank" rel="noopener" class="text-sm text-purple-400 hover:underline">${escapeHTML(user.website)}</a>` : ''}
    </div>
  </div>`
}

function wireHeader(root: HTMLElement, user: User, _posts: unknown[], _isOwnProfile: boolean, me: User | null) {
  root.querySelector('#avatar-btn')?.addEventListener('click', () => {
    root.querySelector<HTMLInputElement>('#avatar-file')!.click()
  })
  root.querySelector<HTMLInputElement>('#avatar-file')?.addEventListener('change', async (e) => {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file) return
    const form = new FormData()
    form.append('avatar', file)
    try {
      await users.updateAvatar(form)
      navigate(`/profile/${user.username}`)
    } catch (err) {
      alert(err instanceof APIError ? err.message : 'Upload failed')
    }
  })

  root.querySelector('#edit-profile-btn')?.addEventListener('click', () =>
    openEditModal(root, user, me!))

  root.querySelector('#dm-btn')?.addEventListener('click', () =>
    navigate(`/messages/${user.username}`))

  root.querySelector<HTMLButtonElement>('#follow-btn')?.addEventListener('click', async (e) => {
    const btn = e.currentTarget as HTMLButtonElement
    const isFollowing = btn.dataset.following === 'true'
    try {
      if (isFollowing) {
        await users.unfollow(user.username)
        btn.textContent = 'Follow'
        btn.className = 'bg-purple-600 px-5 py-1.5 rounded-xl text-sm font-semibold hover:bg-purple-700 transition'
        btn.dataset.following = 'false'
      } else {
        await users.follow(user.username)
        btn.textContent = 'Following'
        btn.className = 'bg-gray-700 px-5 py-1.5 rounded-xl text-sm font-semibold hover:bg-gray-600 transition'
        btn.dataset.following = 'true'
      }
    } catch { /* ignore */ }
  })

  root.querySelector('#followers-btn')?.addEventListener('click', () => openUserListModal('Followers', user.username, 'followers'))
  root.querySelector('#following-btn')?.addEventListener('click', () => openUserListModal('Following', user.username, 'following'))
}

function renderGrid(root: HTMLElement, posts: Post[], _all: Post[], isOwnProfile: boolean, username: string) {
  const grid = root.querySelector<HTMLElement>('#profile-grid')!
  if (posts.length === 0) {
    grid.innerHTML = `<div class="text-center text-gray-500 py-16 text-sm">No posts yet</div>`
    return
  }
  grid.innerHTML = `<div class="grid grid-cols-3 gap-1"></div>`
  const container = grid.querySelector('div')!
  posts.forEach(post => {
    const imgSrc = post.images && post.images.length > 0 ? post.images[0].image_path : post.image_path
    const cell = document.createElement('div')
    cell.className = 'relative aspect-square overflow-hidden bg-gray-900 cursor-pointer group'
    cell.innerHTML = `
      <img src="${imgSrc}" class="w-full h-full object-cover group-hover:scale-105 transition duration-300" loading="lazy" />
      ${post.images && post.images.length > 1
        ? `<div class="absolute top-2 right-2 bg-black/50 rounded-full p-1"><svg class="w-3 h-3 text-white" fill="currentColor" viewBox="0 0 24 24"><path d="M3 3h8v8H3zm10 0h8v8h-8zM3 13h8v8H3z"/></svg></div>`
        : ''}
      <div class="absolute inset-0 bg-black/0 group-hover:bg-black/40 transition flex items-center justify-center opacity-0 group-hover:opacity-100">
        <div class="flex items-center gap-3 text-white text-sm font-semibold">
          <span>♥ ${post.like_count ?? 0}</span>
          ${post.comment_count ? `<span>💬 ${post.comment_count}</span>` : ''}
        </div>
      </div>`
    cell.addEventListener('click', () => openPostLightbox(post, posts))
    container.appendChild(cell)
  })
}

// ─── Edit Profile Modal ───────────────────────────────────────────

function openEditModal(_root: HTMLElement, user: User, _me: User) {
  const overlay = document.createElement('div')
  overlay.className = 'fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4'
  overlay.innerHTML = `
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-md space-y-4">
      <div class="flex items-center justify-between">
        <h3 class="font-semibold text-lg">Edit Profile</h3>
        <button id="edit-close" class="text-gray-400 hover:text-white">✕</button>
      </div>
      <div class="space-y-3">
        <div>
          <label class="text-xs text-gray-400 mb-1 block">Username</label>
          <input id="edit-username" type="text" value="${escapeHTML(user.username)}" class="w-full bg-gray-800 border border-gray-700 rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-purple-500 transition" />
        </div>
        <div>
          <label class="text-xs text-gray-400 mb-1 block">Bio</label>
          <textarea id="edit-bio" class="w-full bg-gray-800 border border-gray-700 rounded-xl px-4 py-2.5 text-sm resize-none h-20 focus:outline-none focus:border-purple-500 transition">${escapeHTML(user.bio ?? '')}</textarea>
        </div>
        <div>
          <label class="text-xs text-gray-400 mb-1 block">Website</label>
          <input id="edit-website" type="url" value="${escapeHTML(user.website ?? '')}" placeholder="https://yoursite.com" class="w-full bg-gray-800 border border-gray-700 rounded-xl px-4 py-2.5 text-sm focus:outline-none focus:border-purple-500 transition" />
        </div>
      </div>
      <div id="edit-error" class="hidden text-red-400 text-sm"></div>
      <div class="flex gap-3">
        <button id="edit-cancel" class="flex-1 border border-gray-700 rounded-xl py-2.5 text-sm hover:bg-gray-800 transition">Cancel</button>
        <button id="edit-save" class="flex-1 bg-purple-600 hover:bg-purple-700 rounded-xl py-2.5 text-sm font-semibold transition">Save</button>
      </div>
    </div>`

  document.body.appendChild(overlay)

  const close = () => overlay.remove()
  overlay.querySelector('#edit-close')!.addEventListener('click', close)
  overlay.querySelector('#edit-cancel')!.addEventListener('click', close)

  overlay.querySelector('#edit-save')!.addEventListener('click', async () => {
    const username = (overlay.querySelector<HTMLInputElement>('#edit-username')!).value.trim()
    const bio = (overlay.querySelector<HTMLTextAreaElement>('#edit-bio')!).value.trim()
    const website = (overlay.querySelector<HTMLInputElement>('#edit-website')!).value.trim()
    const errEl = overlay.querySelector<HTMLDivElement>('#edit-error')!
    const btn = overlay.querySelector<HTMLButtonElement>('#edit-save')!

    btn.disabled = true
    btn.textContent = 'Saving...'
    try {
      await users.updateProfile({ username, bio, website })
      close()
      navigate(`/profile/${username}`)
    } catch (e: any) {
      errEl.textContent = e?.message ?? 'Update failed'
      errEl.classList.remove('hidden')
      btn.disabled = false
      btn.textContent = 'Save'
    }
  })
}

// ─── User List Modal ──────────────────────────────────────────────

async function openUserListModal(title: string, username: string, type: 'followers' | 'following') {
  const overlay = document.createElement('div')
  overlay.className = 'fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4'
  overlay.innerHTML = `
    <div class="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-sm max-h-[80vh] flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-gray-800">
        <h3 class="font-semibold">${title}</h3>
        <button id="ul-close" class="text-gray-400 hover:text-white">✕</button>
      </div>
      <div id="ul-list" class="overflow-y-auto flex-1 p-2">
        <div class="text-center text-gray-500 py-8 text-sm">Loading...</div>
      </div>
    </div>`

  document.body.appendChild(overlay)
  overlay.querySelector('#ul-close')!.addEventListener('click', () => overlay.remove())
  overlay.addEventListener('click', (e) => { if (e.target === overlay) overlay.remove() })

  try {
    const res = type === 'followers' ? await users.followers(username) : await users.following(username)
    const list = overlay.querySelector('#ul-list')!
    if (res.users.length === 0) {
      list.innerHTML = `<div class="text-center text-gray-500 py-8 text-sm">No ${title.toLowerCase()} yet</div>`
      return
    }
    list.innerHTML = res.users.map(u => `
      <button class="w-full flex items-center gap-3 p-3 hover:bg-gray-800 rounded-xl transition text-left go-profile" data-username="${u.username}">
        ${avatar(u, 10)}
        <div>
          <div class="font-semibold text-sm">${escapeHTML(u.username)}</div>
          ${u.bio ? `<div class="text-xs text-gray-400 truncate max-w-xs">${escapeHTML(u.bio)}</div>` : ''}
        </div>
      </button>`).join('')

    list.querySelectorAll<HTMLButtonElement>('.go-profile').forEach(btn => {
      btn.addEventListener('click', () => {
        overlay.remove()
        navigate(`/profile/${btn.dataset.username}`)
      })
    })
  } catch {
    overlay.querySelector('#ul-list')!.innerHTML = `<div class="text-center text-red-400 py-8 text-sm">Failed to load</div>`
  }
}

function headerSkeleton(): string {
  return `<div class="flex items-start gap-8 animate-pulse">
    <div class="w-24 h-24 rounded-full bg-gray-800 flex-shrink-0"></div>
    <div class="flex-1 space-y-3">
      <div class="h-5 bg-gray-800 rounded w-32"></div>
      <div class="flex gap-6"><div class="h-3 bg-gray-800 rounded w-16"></div><div class="h-3 bg-gray-800 rounded w-20"></div></div>
      <div class="h-3 bg-gray-800 rounded w-48"></div>
    </div>
  </div>`
}
