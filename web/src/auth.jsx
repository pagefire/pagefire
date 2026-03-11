import { createContext } from 'preact'
import { useState, useEffect, useContext, useCallback } from 'preact/hooks'
import { apiPost, apiGet } from './api.js'

const AuthContext = createContext(null)

export function useAuth() {
  return useContext(AuthContext)
}

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)

  const checkAuth = useCallback(async () => {
    const { data } = await apiGet('/auth/me')
    setUser(data)
    setLoading(false)
  }, [])

  useEffect(() => {
    checkAuth()

    // Listen for 401 responses from API calls
    const handleUnauthorized = () => {
      setUser(null)
      setLoading(false)
    }
    window.addEventListener('pagefire:unauthorized', handleUnauthorized)
    return () => window.removeEventListener('pagefire:unauthorized', handleUnauthorized)
  }, [checkAuth])

  const login = async (email, password) => {
    const { data, error } = await apiPost('/auth/login', { email, password })
    if (data) {
      setUser(data)
      return { error: null }
    }
    return { error: error || 'Login failed' }
  }

  const logout = async () => {
    await apiPost('/auth/logout')
    setUser(null)
  }

  return (
    <AuthContext.Provider value={{ user, loading, login, logout, checkAuth }}>
      {children}
    </AuthContext.Provider>
  )
}

export function LoginGate({ children }) {
  const { user, loading, login } = useAuth()
  const [setupRequired, setSetupRequired] = useState(null)
  const [checking, setChecking] = useState(true)

  useEffect(() => {
    const checkSetup = async () => {
      const { data } = await apiGet('/auth/setup')
      setSetupRequired(data?.setup_required ?? false)
      setChecking(false)
    }
    if (!user && !loading) {
      checkSetup()
    } else {
      setChecking(false)
    }
  }, [user, loading])

  if (loading || checking) {
    return <div class="login-gate"><div class="login-card"><div class="login-logo">PageFire</div><p class="text-muted">Loading...</p></div></div>
  }

  if (user) return children

  if (setupRequired) {
    return <SetupForm />
  }

  return <LoginForm login={login} />
}

function LoginForm({ login }) {
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!email.trim()) { setError('Email is required'); return }
    if (!password) { setError('Password is required'); return }

    setLoading(true)
    setError('')

    const { error: err } = await login(email.trim(), password)
    if (err) {
      setError(err)
      setLoading(false)
    }
    // On success, login() sets user in context → LoginGate re-renders to show children
  }

  return (
    <div class="login-gate">
      <form class="login-card" onSubmit={handleSubmit}>
        <div class="login-logo">PageFire</div>
        <p class="login-subtitle">Sign in to your account</p>
        <input
          type="email"
          class="login-input"
          placeholder="Email"
          value={email}
          onInput={(e) => setEmail(e.target.value)}
          autoFocus
        />
        <input
          type="password"
          class="login-input"
          placeholder="Password"
          value={password}
          onInput={(e) => setPassword(e.target.value)}
        />
        {error && <p class="login-error">{error}</p>}
        <button class="login-button" type="submit" disabled={loading}>
          {loading ? 'Signing in...' : 'Sign In'}
        </button>
      </form>
    </div>
  )
}

function SetupForm() {
  const { checkAuth, login } = useAuth()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const validateEmail = (v) => /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v)

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!name.trim()) { setError('Name is required'); return }
    if (!email.trim()) { setError('Email is required'); return }
    if (!validateEmail(email.trim())) { setError('Invalid email address'); return }
    if (!password) { setError('Password is required'); return }
    if (password.length < 8) { setError('Password must be at least 8 characters'); return }
    if (password !== confirm) { setError('Passwords do not match'); return }

    setLoading(true)
    setError('')

    const { error: err } = await apiPost('/auth/setup', {
      name: name.trim(),
      email: email.trim(),
      password,
    })
    if (err) {
      setError(err)
      setLoading(false)
      return
    }

    // Setup auto-logs in via session cookie — reload page to pick up session
    window.location.href = '/'
  }

  return (
    <div class="login-gate">
      <form class="login-card" onSubmit={handleSubmit}>
        <div class="login-logo">PageFire</div>
        <p class="login-subtitle">Create your admin account</p>
        <input
          type="text"
          class="login-input"
          placeholder="Name"
          value={name}
          onInput={(e) => setName(e.target.value)}
          autoFocus
        />
        <input
          type="email"
          class="login-input"
          placeholder="Email"
          value={email}
          onInput={(e) => setEmail(e.target.value)}
        />
        <input
          type="password"
          class="login-input"
          placeholder="Password (min 8 characters)"
          value={password}
          onInput={(e) => setPassword(e.target.value)}
        />
        <input
          type="password"
          class="login-input"
          placeholder="Confirm password"
          value={confirm}
          onInput={(e) => setConfirm(e.target.value)}
        />
        {error && <p class="login-error">{error}</p>}
        <button class="login-button" type="submit" disabled={loading}>
          {loading ? 'Creating account...' : 'Create Admin Account'}
        </button>
      </form>
    </div>
  )
}
