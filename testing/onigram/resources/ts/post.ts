import { posts as postsAPI, APIError } from './api.ts'
import { getUser } from './auth.ts'
import { navigate } from './router.ts'
import { renderNavBar, wirePostActions, renderPostCard } from './feed.ts'

export async function renderPost(root: HTMLElement, params: Record<string, string>) {
  const postId = parseInt(params['id'])
  const me = getUser()

  root.innerHTML = `
  <div class="max-w-lg mx-auto px-4 py-6 space-y-4">
    ${renderNavBar(me?.username ?? '')}
    <div id="post-content" class="text-center text-gray-500 py-12">Loading...</div>
  </div>`

  try {
    const [post, commentsRes] = await Promise.all([
      postsAPI.get(postId),
      postsAPI.comments(postId),
    ])

    root.querySelector('#post-content')!.innerHTML = `
      ${renderPostCard(post)}
      <div class="bg-gray-900 border border-gray-800 rounded-2xl p-4 space-y-4">
        <h3 class="font-semibold text-sm text-gray-300">${commentsRes.comments.length} comment${commentsRes.comments.length !== 1 ? 's' : ''}</h3>
        <div id="comments-list" class="space-y-3">
          ${commentsRes.comments.map(c => `
            <div class="flex gap-3 items-start">
              ${c.user?.avatar_path
                ? `<img src="${c.user.avatar_path}" class="w-8 h-8 rounded-full object-cover flex-shrink-0" />`
                : `<div class="w-8 h-8 rounded-full bg-purple-700 flex items-center justify-center text-xs font-bold flex-shrink-0">${(c.user?.username ?? '?')[0].toUpperCase()}</div>`}
              <div>
                <span class="font-semibold text-sm">${c.user?.username ?? 'unknown'}</span>
                <span class="text-sm text-gray-300 ml-2">${c.body}</span>
              </div>
            </div>
          `).join('')}
        </div>
        ${me ? `
        <div class="flex gap-2">
          <input id="comment-input" type="text" placeholder="Add a comment..." class="flex-1 bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-purple-500" />
          <button id="comment-btn" class="bg-purple-600 hover:bg-purple-700 px-4 rounded-lg text-sm font-semibold transition">Post</button>
        </div>` : `<p class="text-sm text-gray-500 text-center"><a href="/login" data-link class="text-purple-400 hover:underline">Sign in</a> to comment</p>`}
      </div>`

    wirePostActions(root)

    root.querySelector('#comment-btn')?.addEventListener('click', async () => {
      const input = root.querySelector<HTMLInputElement>('#comment-input')!
      const body = input.value.trim()
      if (!body) return
      try {
        const comment = await postsAPI.addComment(postId, body)
        const list = root.querySelector('#comments-list')!
        const avatar = comment.user?.avatar_path
          ? `<img src="${comment.user.avatar_path}" class="w-8 h-8 rounded-full object-cover flex-shrink-0" />`
          : `<div class="w-8 h-8 rounded-full bg-purple-700 flex items-center justify-center text-xs font-bold flex-shrink-0">${(comment.user?.username ?? me!.username)[0].toUpperCase()}</div>`
        list.insertAdjacentHTML('beforeend', `
          <div class="flex gap-3 items-start">
            ${avatar}
            <div><span class="font-semibold text-sm">${comment.user?.username ?? me!.username}</span>
            <span class="text-sm text-gray-300 ml-2">${comment.body}</span></div>
          </div>`)
        input.value = ''
      } catch (e) {
        alert(e instanceof APIError ? e.message : 'Failed to post comment')
      }
    })

  } catch {
    root.querySelector('#post-content')!.innerHTML = '<div class="text-center text-red-400 py-12">Post not found</div>'
  }
}
