import { users, bookmarks as bookmarksAPI, APIError } from './api.ts'
import { getUser } from './auth.ts'
import { navigate } from './router.ts'
import type { User, Post } from './types.ts'
import { avatar, escapeHTML, skeleton, errorState } from './feed.ts'
import { openPostLightbox } from './post.ts'

export async function renderProfile(root: HTMLElement, params: Record<string, string>) {
  const username = params['username']
  const me = getUser()

  root.innerHTML = `
  <div class="max-w-3xl mx-auto px-4 py-6">
    <div id="profile-header" class="mb-6">${headerSkeleton()}</div>
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
    root.querySelector('#profile-header')!.innerHTML = buildHeader(user, postsRes.posts.length, isOwnProfile)
    wireHeader(root, user, postsRes.posts, isOwnProfile, me)

    // ─── Tabs ─────────────────────────────────────────────────────
    const tabsEl = root.querySelector<HTMLElement>('#profile-tabs')!
    tabsEl.classList.remove('hidden')
    tabsEl.innerHTML = `
      <button class="tab-btn pb-3 border-b-2 border-white text-white" data-tab="posts">Posts</button>
      ${isOwnProfile ? `<button class="tab-btn pb-3 border-b-2 border-transparent text-gray-500 hover:text-white transition" data-tab="saved">Saved</button>` : ''}
    `

    // ─── Grid ─────────────────────────────────────────────────────
    renderGrid(root, postsRes.posts, postsRes.posts)

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
            renderGrid(root, res.posts, postsRes.posts)
          } catch {
            root.querySelector('#profile-grid')!.innerHTML = errorState('Failed to load saved posts')
          }
        } else {
          renderGrid(root, postsRes.posts, postsRes.posts)
        }
      })
    })

  } catch {
    root.querySelector('#profile-header')!.innerHTML = errorState('Profile not found')
  }
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
  // Avatar change
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

  // Edit profile
  root.querySelector('#edit-profile-btn')?.addEventListener('click', () =>
    openEditModal(root, user, me!))

  // DM button
  root.querySelector('#dm-btn')?.addEventListener('click', () =>
    navigate(`/messages/${user.username}`))

  // Follow / unfollow
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

  // Followers/Following modals
  root.querySelector('#followers-btn')?.addEventListener('click', () => openUserListModal('Followers', user.username, 'followers'))
  root.querySelector('#following-btn')?.addEventListener('click', () => openUserListModal('Following', user.username, 'following'))
}

function renderGrid(root: HTMLElement, posts: Post[], _all: Post[]) {
  const grid = root.querySelector<HTMLElement>('#profile-grid')!
  if (posts.length === 0) {
    grid.innerHTML = `<div class="text-center text-gray-500 py-16 text-sm">No posts yet</div>`
    return
  }
  grid.innerHTML = `<div class="grid grid-cols-3 gap-1"></div>`
  const container = grid.querySelector('div')!
  posts.forEach(post => {
    const cell = document.createElement('div')
    cell.className = 'relative aspect-square overflow-hidden bg-gray-900 cursor-pointer group'
    cell.innerHTML = `
      <img src="${post.image_path}" class="w-full h-full object-cover group-hover:scale-105 transition duration-300" loading="lazy" />
      <div class="absolute inset-0 bg-black/0 group-hover:bg-black/40 transition flex items-center justify-center opacity-0 group-hover:opacity-100">
        <span class="text-white text-sm font-semibold">♥ ${post.like_count ?? 0}</span>
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

// ─── User List Modal (followers / following) ──────────────────────

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
