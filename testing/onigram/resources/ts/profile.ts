import { users, APIError } from './api.ts'
import { getUser } from './auth.ts'
import { navigate } from './router.ts'
import { renderNavBar, renderPostCard, wirePostActions } from './feed.ts'

export async function renderProfile(root: HTMLElement, params: Record<string, string>) {
  const username = params['username']
  const me = getUser()
  root.innerHTML = `
  <div class="max-w-lg mx-auto px-4 py-6 space-y-6">
    ${renderNavBar(me?.username ?? '')}
    <div id="profile-content" class="text-center text-gray-500 py-12">Loading...</div>
  </div>`

  try {
    const [user, postsRes] = await Promise.all([
      users.get(username),
      users.posts(username),
    ])
    const isOwnProfile = me?.username === username

    root.querySelector('#profile-content')!.innerHTML = `
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6 space-y-4">
      <div class="flex items-center gap-6">
        ${user.avatar_path
          ? `<img src="${user.avatar_path}" class="w-20 h-20 rounded-full object-cover" />`
          : `<div class="w-20 h-20 rounded-full bg-purple-700 flex items-center justify-center text-2xl font-bold">${username[0].toUpperCase()}</div>`}
        <div class="flex-1 space-y-2">
          <div class="flex items-center gap-3">
            <h2 class="text-xl font-bold">${user.username}</h2>
            ${isOwnProfile
              ? `<button id="edit-profile-btn" class="border border-gray-600 px-4 py-1 rounded-lg text-sm hover:bg-gray-800 transition">Edit</button>`
              : user.is_following
                ? `<button id="follow-btn" data-following="true" class="bg-gray-700 px-4 py-1 rounded-lg text-sm hover:bg-gray-600 transition">Following</button>`
                : `<button id="follow-btn" data-following="false" class="bg-purple-600 px-4 py-1 rounded-lg text-sm hover:bg-purple-700 transition">Follow</button>`}
          </div>
          <div class="flex gap-6 text-sm text-gray-300">
            <span><strong>${postsRes.posts.length}</strong> posts</span>
            <button id="followers-btn" class="hover:text-white"><strong>${user.follower_count ?? 0}</strong> followers</button>
            <button id="following-btn" class="hover:text-white"><strong>${user.following_count ?? 0}</strong> following</button>
          </div>
          ${user.bio ? `<p class="text-sm text-gray-300">${user.bio}</p>` : ''}
        </div>
      </div>
      ${isOwnProfile ? `
      <div id="avatar-upload" class="border border-dashed border-gray-700 rounded-xl p-3 text-sm text-center text-gray-500 hover:border-gray-500 cursor-pointer">
        Click to update avatar
        <input type="file" id="avatar-file" accept="image/*" class="hidden" />
      </div>` : ''}
    </div>

    <div id="posts-grid" class="space-y-4">
      ${postsRes.posts.length === 0 ? '<div class="text-center text-gray-500 py-8">No posts yet</div>' : ''}
    </div>`

    postsRes.posts.forEach(p => {
      root.querySelector('#posts-grid')!.insertAdjacentHTML('beforeend', renderPostCard(p))
    })
    wirePostActions(root)

    // Follow / unfollow
    root.querySelector<HTMLButtonElement>('#follow-btn')?.addEventListener('click', async (e) => {
      const btn = e.currentTarget as HTMLButtonElement
      const isFollowing = btn.dataset.following === 'true'
      try {
        if (isFollowing) {
          await users.unfollow(username)
          btn.textContent = 'Follow'
          btn.className = 'bg-purple-600 px-4 py-1 rounded-lg text-sm hover:bg-purple-700 transition'
          btn.dataset.following = 'false'
        } else {
          await users.follow(username)
          btn.textContent = 'Following'
          btn.className = 'bg-gray-700 px-4 py-1 rounded-lg text-sm hover:bg-gray-600 transition'
          btn.dataset.following = 'true'
        }
      } catch { /* ignore */ }
    })

    // Avatar upload
    const avatarUploadEl = root.querySelector<HTMLDivElement>('#avatar-upload')
    if (avatarUploadEl) {
      avatarUploadEl.addEventListener('click', () => {
        root.querySelector<HTMLInputElement>('#avatar-file')!.click()
      })
      root.querySelector<HTMLInputElement>('#avatar-file')!.addEventListener('change', async (e) => {
        const file = (e.target as HTMLInputElement).files?.[0]
        if (!file) return
        const form = new FormData()
        form.append('avatar', file)
        try {
          await users.updateAvatar(form)
          navigate(`/profile/${username}`)
        } catch (err) {
          alert(err instanceof APIError ? err.message : 'Upload failed')
        }
      })
    }

    root.querySelector('#followers-btn')?.addEventListener('click', () => navigate(`/profile/${username}/followers`))
    root.querySelector('#following-btn')?.addEventListener('click', () => navigate(`/profile/${username}/following`))

  } catch {
    root.querySelector('#profile-content')!.innerHTML = '<div class="text-center text-red-400 py-12">Profile not found</div>'
  }
}
