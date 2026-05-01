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
  dispatch(path)
}

function dispatch(path: string) {
  for (const route of routes) {
    const m = path.match(route.pattern)
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
  const anchor = (e.target as Element).closest('a[data-link]')
  if (!anchor) return
  e.preventDefault()
  navigate((anchor as HTMLAnchorElement).href.replace(location.origin, ''))
})

export function start() {
  dispatch(location.pathname)
}

export function currentPath(): string {
  return location.pathname
}
