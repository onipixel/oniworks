import { stories as storiesAPI, highlights as highlightsAPI } from './api.ts'
import { getUser } from './auth.ts'
import type { Story, StoryGroup } from './types.ts'

// ─── Story Bar ────────────────────────────────────────────────────

export async function renderStoryBar(root: HTMLElement) {
  const me = getUser()
  root.innerHTML = `<div class="flex gap-3 overflow-x-auto pb-2 scrollbar-none"></div>`
  const bar = root.querySelector('div')!

  try {
    const { stories } = await storiesAPI.feed()
    // Group by user
    const groups: Map<number, StoryGroup> = new Map()

    // Always put "Your story" first
    if (me) {
      groups.set(me.id, { user: me as any, stories: [], hasUnseen: false })
    }

    for (const s of stories) {
      const uid = s.user_id
      if (!groups.has(uid)) {
        groups.set(uid, { user: s.user as any, stories: [], hasUnseen: false })
      }
      const g = groups.get(uid)!
      g.stories.push(s)
      if (!s.viewed) g.hasUnseen = true
    }

    groups.forEach((group, uid) => {
      const isMe = uid === me?.id
      const noStory = isMe && group.stories.length === 0

      // Gradient ring: use a wrapper div with gradient bg + 2px inner gap
      const ringStyle = noStory
        ? 'background:#374151;'  // gray-700 for "no story yet"
        : group.hasUnseen
          ? 'background:linear-gradient(135deg,#f97316,#ec4899,#a855f7);'
          : 'background:#4b5563;' // gray-600 for already-seen

      const avInner = group.user?.avatar_path
        ? `<img src="${group.user.avatar_path}" style="width:52px;height:52px;object-fit:cover;" class="rounded-full" />`
        : `<div style="width:52px;height:52px;font-size:18px;" class="rounded-full bg-gradient-to-br from-purple-500 to-pink-500 flex items-center justify-center font-bold">${(group.user?.username ?? '?')[0].toUpperCase()}</div>`

      const label = isMe ? 'Your story' : (group.user?.username ?? '')

      const btn = document.createElement('button')
      btn.className = 'flex flex-col items-center gap-1.5 flex-shrink-0 focus:outline-none'
      btn.innerHTML = `
        <div class="relative" style="width:64px;height:64px;">
          <!-- gradient ring -->
          <div style="${ringStyle}width:64px;height:64px;border-radius:50%;display:flex;align-items:center;justify-content:center;">
            <!-- dark gap -->
            <div style="width:58px;height:58px;border-radius:50%;background:#030712;display:flex;align-items:center;justify-content:center;">
              ${avInner}
            </div>
          </div>
          ${noStory ? `<div style="position:absolute;bottom:1px;right:1px;width:20px;height:20px;background:#9333ea;border-radius:50%;display:flex;align-items:center;justify-content:center;border:2px solid #030712;font-size:13px;font-weight:bold;">+</div>` : ''}
        </div>
        <span class="text-xs text-gray-400 truncate text-center" style="max-width:64px;">${label}</span>`

      btn.addEventListener('click', () => {
        if (noStory) {
          openStoryCreator()
        } else {
          openStoryViewer(Array.from(groups.values()).flatMap(g => g.stories), group.stories[0]?.id)
        }
      })
      bar.appendChild(btn)
    })
  } catch { /* story bar is non-critical */ }
}

// ─── Story Viewer ─────────────────────────────────────────────────

function openStoryViewer(allStories: Story[], startID?: number) {
  let idx = startID ? allStories.findIndex(s => s.id === startID) : 0
  if (idx < 0) idx = 0

  const overlay = document.createElement('div')
  overlay.className = 'fixed inset-0 bg-black z-50 flex items-center justify-center'
  overlay.innerHTML = `
    <div class="relative w-full max-w-sm h-full max-h-[95vh] flex flex-col">
      <!-- progress bars -->
      <div id="story-progress" class="flex gap-1 p-2 z-10"></div>
      <!-- header -->
      <div id="story-header" class="flex items-center gap-3 px-3 pb-2 z-10"></div>
      <!-- image -->
      <div class="flex-1 flex items-center justify-center overflow-hidden rounded-2xl bg-gray-900">
        <img id="story-img" class="w-full h-full object-contain" />
      </div>
      <!-- nav -->
      <div class="absolute inset-0 flex">
        <div id="story-prev" class="flex-1 cursor-pointer"></div>
        <div id="story-next" class="flex-1 cursor-pointer"></div>
      </div>
      <!-- close -->
      <button id="story-close" class="absolute top-4 right-4 text-white text-2xl z-20">✕</button>
      <!-- save to highlight (own stories only) -->
      <button id="story-highlight-btn" class="hidden absolute bottom-4 left-1/2 -translate-x-1/2 bg-white/10 hover:bg-white/20 backdrop-blur text-white text-xs font-semibold px-4 py-2 rounded-full z-20 transition">♡ Add to Highlight</button>
    </div>`

  document.body.appendChild(overlay)

  let timer: ReturnType<typeof setTimeout>

  function show(i: number) {
    if (i < 0 || i >= allStories.length) { close(); return }
    idx = i
    const story = allStories[i]

    ;(overlay.querySelector<HTMLImageElement>('#story-img')!).src = story.image_path

    // Header
    const av = story.user?.avatar_path
      ? `<img src="${story.user.avatar_path}" class="w-8 h-8 rounded-full object-cover" />`
      : `<div class="w-8 h-8 rounded-full bg-purple-600 flex items-center justify-center text-xs font-bold">${(story.user?.username ?? '?')[0].toUpperCase()}</div>`
    overlay.querySelector('#story-header')!.innerHTML = `${av}<span class="font-semibold text-sm">${story.user?.username ?? ''}</span>`

    // Progress bars
    const prog = overlay.querySelector('#story-progress')!
    prog.innerHTML = allStories.map((_, j) => `<div class="flex-1 h-0.5 rounded-full overflow-hidden bg-gray-600"><div class="h-full bg-white transition-all duration-5000 ${j < i ? 'w-full' : j === i ? 'progress-active w-0' : 'w-0'}"></div></div>`).join('')

    // Animate current bar
    requestAnimationFrame(() => {
      const bar = prog.querySelectorAll<HTMLDivElement>('.progress-active')[0]
      if (bar) {
        bar.style.transition = 'width 5s linear'
        bar.style.width = '100%'
      }
    })

    // Mark viewed
    storiesAPI.markViewed(story.id).catch(() => {})

    // Show "Add to Highlight" for own stories
    const hlBtn = overlay.querySelector<HTMLButtonElement>('#story-highlight-btn')!
    const me = getUser()
    if (me && story.user_id === me.id) {
      hlBtn.classList.remove('hidden')
      hlBtn.onclick = () => openAddToHighlightModal(story.id)
    } else {
      hlBtn.classList.add('hidden')
    }

    clearTimeout(timer)
    timer = setTimeout(() => show(i + 1), 5000)
  }

  function close() {
    clearTimeout(timer)
    overlay.remove()
  }

  overlay.querySelector('#story-close')!.addEventListener('click', close)
  overlay.querySelector('#story-prev')!.addEventListener('click', () => show(idx - 1))
  overlay.querySelector('#story-next')!.addEventListener('click', () => show(idx + 1))
  overlay.addEventListener('click', (e) => { if (e.target === overlay) close() })

  show(idx)
}

// ─── Story Creator ────────────────────────────────────────────────

function openStoryCreator() {
  const overlay = document.createElement('div')
  overlay.className = 'fixed inset-0 bg-black/70 z-50 flex items-center justify-center p-4'
  overlay.innerHTML = `
    <div class="bg-gray-900 border border-gray-800 rounded-2xl p-6 w-full max-w-sm space-y-4">
      <h3 class="font-semibold text-lg">Add to Story</h3>
      <input type="file" id="story-file" accept="image/*" class="block w-full text-sm text-gray-400 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:bg-purple-700 file:text-white file:border-0 hover:file:bg-purple-600 cursor-pointer" />
      <img id="story-preview" class="hidden rounded-xl w-full object-cover max-h-64" />
      <div id="story-error" class="hidden text-red-400 text-sm"></div>
      <div class="flex gap-3">
        <button id="story-cancel" class="flex-1 border border-gray-700 rounded-xl py-2.5 text-sm hover:bg-gray-800 transition">Cancel</button>
        <button id="story-share" class="flex-1 bg-purple-600 hover:bg-purple-700 rounded-xl py-2.5 text-sm font-semibold transition">Share</button>
      </div>
    </div>`

  document.body.appendChild(overlay)

  overlay.querySelector('#story-file')!.addEventListener('change', (e) => {
    const file = (e.target as HTMLInputElement).files?.[0]
    if (!file) return
    const preview = overlay.querySelector<HTMLImageElement>('#story-preview')!
    preview.src = URL.createObjectURL(file)
    preview.classList.remove('hidden')
  })

  const close = () => overlay.remove()
  overlay.querySelector('#story-cancel')!.addEventListener('click', close)

  overlay.querySelector('#story-share')!.addEventListener('click', async () => {
    const file = (overlay.querySelector<HTMLInputElement>('#story-file')!).files?.[0]
    const errEl = overlay.querySelector<HTMLDivElement>('#story-error')!
    if (!file) { errEl.textContent = 'Select an image first'; errEl.classList.remove('hidden'); return }

    const form = new FormData()
    form.append('image', file)
    const btn = overlay.querySelector<HTMLButtonElement>('#story-share')!
    btn.disabled = true
    btn.textContent = 'Sharing...'
    try {
      await storiesAPI.create(form)
      close()
      window.location.reload()
    } catch (e: any) {
      errEl.textContent = e?.message ?? 'Upload failed'
      errEl.classList.remove('hidden')
      btn.disabled = false
      btn.textContent = 'Share'
    }
  })
}

// ─── Add to Highlight ─────────────────────────────────────────────

async function openAddToHighlightModal(storyId: number) {
  const me = getUser()
  if (!me) return

  const overlay = document.createElement('div')
  overlay.className = 'fixed inset-0 bg-black/70 z-[60] flex items-end md:items-center justify-center p-4'
  overlay.innerHTML = `
    <div class="bg-gray-900 border border-gray-800 rounded-2xl w-full max-w-sm max-h-[70vh] flex flex-col">
      <div class="flex items-center justify-between p-4 border-b border-gray-800">
        <h3 class="font-semibold">Save to Highlight</h3>
        <button id="ath-close" class="text-gray-400 hover:text-white">✕</button>
      </div>
      <div id="ath-list" class="overflow-y-auto flex-1 p-3 space-y-2">
        <div class="text-center text-gray-500 py-4 text-sm">Loading highlights...</div>
      </div>
      <div class="p-3 border-t border-gray-800">
        <button id="ath-new" class="w-full flex items-center gap-3 px-4 py-2.5 border border-dashed border-gray-700 rounded-xl hover:border-purple-500 hover:text-purple-400 transition text-sm text-gray-400">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v16m8-8H4"/></svg>
          New Highlight
        </button>
      </div>
    </div>`
  document.body.appendChild(overlay)

  const close = () => overlay.remove()
  overlay.querySelector('#ath-close')!.addEventListener('click', close)
  overlay.addEventListener('click', (e) => { if (e.target === overlay) close() })

  // Load existing highlights
  try {
    const { highlights } = await highlightsAPI.list(me.username)
    const list = overlay.querySelector('#ath-list')!
    if (highlights.length === 0) {
      list.innerHTML = `<p class="text-center text-gray-500 text-sm py-4">No highlights yet. Create one below.</p>`
    } else {
      list.innerHTML = highlights.map(hl => `
        <button class="ath-pick w-full flex items-center gap-3 px-3 py-2.5 rounded-xl hover:bg-gray-800 transition text-left" data-id="${hl.id}">
          <div style="width:44px;height:44px;border-radius:50%;overflow:hidden;flex-shrink:0;background:#374151;">
            ${hl.cover_image_path ? `<img src="${hl.cover_image_path}" style="width:44px;height:44px;object-fit:cover;" />` : ''}
          </div>
          <span class="text-sm font-medium">${hl.title}</span>
        </button>`).join('')
      list.querySelectorAll<HTMLButtonElement>('.ath-pick').forEach(btn => {
        btn.addEventListener('click', async () => {
          await highlightsAPI.addStory(parseInt(btn.dataset.id!), storyId)
          close()
        })
      })
    }
  } catch { /* ignore */ }

  overlay.querySelector('#ath-new')!.addEventListener('click', async () => {
    const title = prompt('Highlight name:')?.trim()
    if (!title) return
    try {
      await highlightsAPI.create(title, storyId)
      close()
    } catch (e: any) {
      alert(e?.message ?? 'Failed to create highlight')
    }
  })
}
