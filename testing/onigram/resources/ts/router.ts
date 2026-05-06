// Minimal client-side router for OniGram SPA.

type RouteHandler = (params: Record<string, string>) => void

interface Route {
  pattern: RegExp
  keys: string[]
  handler: RouteHandler
}

const routes: Route[] = []

export function on(path: string, handler: RouteHandler) {
  const keys: string[] = []
  const pattern = new RegExp(
    '^' + path.replace(/:(\w+)/g, (_m, k) => { keys.push(k); return '([^/]+)' }) + '/?$'
  )
  routes.push({ pattern, keys, handler })
}

export function navigate(path: string) {
  history.pushState({}, '', path)
  // Let any open overlay (lightbox, modals) know navigation happened
  document.dispatchEvent(new CustomEvent('oni:navigate', { detail: { path } }))
  dispatch(path)
}

function dispatch(path: string) {
  const pathname = path.split('?')[0]
  for (const route of routes) {
    const m = pathname.match(route.pattern)
    if (m) {
      const params: Record<string, string> = {}
      route.keys.forEach((k, i) => { params[k] = m[i + 1] })
      route.handler(params)
      return
    }
  }
}

window.addEventListener('popstate', () => dispatch(location.pathname))

document.addEventListener('click', (e) => {
  // Hashtag links: data-hashtag="tag"
  const htEl = (e.target as Element).closest('[data-hashtag]') as HTMLElement | null
  if (htEl) {
    e.preventDefault()
    navigate('/explore?tag=' + encodeURIComponent(htEl.dataset.hashtag!))
    return
  }

  // Mention links: data-mention="username"
  const mnEl = (e.target as Element).closest('[data-mention]') as HTMLElement | null
  if (mnEl) {
    e.preventDefault()
    navigate('/profile/' + mnEl.dataset.mention!)
    return
  }

  // Regular SPA links
  const anchor = (e.target as Element).closest('a[data-link]')
  if (!anchor) return
  e.preventDefault()
  const url = new URL((anchor as HTMLAnchorElement).href)
  navigate(url.pathname + url.search)
})

export function start() {
  dispatch(location.pathname)
}

export function currentPath(): string {
  return location.pathname
}
