import { auth, APIError } from './api.ts'
import { navigate } from './router.ts'
import type { User } from './types.ts'

let currentUser: User | null = null

export function getUser(): User | null { return currentUser }
export function setUser(u: User | null) { currentUser = u }

export async function loadUser(): Promise<User | null> {
  const tok = localStorage.getItem('og_token')
  if (!tok) return null
  try {
    currentUser = await auth.me()
    return currentUser
  } catch {
    localStorage.removeItem('og_token')
    currentUser = null
    return null
  }
}

export function logout() {
  localStorage.removeItem('og_token')
  currentUser = null
  location.href = '/login'
}

export function requireAuth(): boolean {
  if (!currentUser) {
    navigate('/login')
    return false
  }
  return true
}

// ─── Login Page ───────────────────────────────────────────────────

export function renderLogin(root: HTMLElement) {
  root.innerHTML = `
  <div class="flex items-center justify-center min-h-screen px-4">
    <div class="w-full max-w-sm">
      <h1 class="text-3xl font-bold text-center mb-8 bg-gradient-to-r from-purple-400 to-pink-400 bg-clip-text text-transparent">OniGram</h1>
      <div class="bg-gray-900 border border-gray-800 rounded-2xl p-8 space-y-4">
        <h2 class="text-xl font-semibold text-center">Sign in</h2>
        <div id="auth-error" class="hidden bg-red-900/40 border border-red-700 text-red-300 rounded-lg px-4 py-2 text-sm"></div>
        <input id="email" type="email" placeholder="Email" class="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-sm focus:outline-none focus:border-purple-500" />
        <input id="password" type="password" placeholder="Password" class="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-sm focus:outline-none focus:border-purple-500" />
        <button id="login-btn" class="w-full bg-purple-600 hover:bg-purple-700 text-white font-semibold py-3 rounded-lg transition">Sign in</button>
        <p class="text-center text-sm text-gray-400">Don't have an account? <a href="/register" data-link class="text-purple-400 hover:underline">Sign up</a></p>
      </div>
    </div>
  </div>`

  const emailEl = root.querySelector<HTMLInputElement>('#email')!
  const passwordEl = root.querySelector<HTMLInputElement>('#password')!
  const errorEl = root.querySelector<HTMLDivElement>('#auth-error')!

  root.querySelector('#login-btn')!.addEventListener('click', async () => {
    errorEl.classList.add('hidden')
    try {
      const res = await auth.login(emailEl.value, passwordEl.value)
      localStorage.setItem('og_token', res.token)
      currentUser = res.user
      navigate('/feed')
    } catch (e) {
      const msg = e instanceof APIError ? e.message : 'Login failed'
      errorEl.textContent = msg
      errorEl.classList.remove('hidden')
    }
  })
}

// ─── Register Page ────────────────────────────────────────────────

export function renderRegister(root: HTMLElement) {
  root.innerHTML = `
  <div class="flex items-center justify-center min-h-screen px-4">
    <div class="w-full max-w-sm">
      <h1 class="text-3xl font-bold text-center mb-8 bg-gradient-to-r from-purple-400 to-pink-400 bg-clip-text text-transparent">OniGram</h1>
      <div class="bg-gray-900 border border-gray-800 rounded-2xl p-8 space-y-4">
        <h2 class="text-xl font-semibold text-center">Create account</h2>
        <div id="reg-error" class="hidden bg-red-900/40 border border-red-700 text-red-300 rounded-lg px-4 py-2 text-sm"></div>
        <input id="username" type="text" placeholder="Username" class="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-sm focus:outline-none focus:border-purple-500" />
        <input id="email" type="email" placeholder="Email" class="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-sm focus:outline-none focus:border-purple-500" />
        <input id="password" type="password" placeholder="Password (min 8 chars)" class="w-full bg-gray-800 border border-gray-700 rounded-lg px-4 py-3 text-sm focus:outline-none focus:border-purple-500" />
        <button id="reg-btn" class="w-full bg-purple-600 hover:bg-purple-700 text-white font-semibold py-3 rounded-lg transition">Sign up</button>
        <p class="text-center text-sm text-gray-400">Already have an account? <a href="/login" data-link class="text-purple-400 hover:underline">Sign in</a></p>
      </div>
    </div>
  </div>`

  const usernameEl = root.querySelector<HTMLInputElement>('#username')!
  const emailEl = root.querySelector<HTMLInputElement>('#email')!
  const passwordEl = root.querySelector<HTMLInputElement>('#password')!
  const errorEl = root.querySelector<HTMLDivElement>('#reg-error')!

  root.querySelector('#reg-btn')!.addEventListener('click', async () => {
    errorEl.classList.add('hidden')
    try {
      const res = await auth.register(usernameEl.value, emailEl.value, passwordEl.value)
      localStorage.setItem('og_token', res.token)
      currentUser = res.user
      navigate('/feed')
    } catch (e) {
      const msg = e instanceof APIError ? e.message : 'Registration failed'
      errorEl.textContent = msg
      errorEl.classList.remove('hidden')
    }
  })
}
