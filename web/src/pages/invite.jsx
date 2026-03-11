import { useState, useEffect } from 'preact/hooks'
import { apiGet, apiPost } from '../api.js'

export function InviteAccept({ token }) {
  const [userInfo, setUserInfo] = useState(null)
  const [error, setError] = useState(null)
  const [loading, setLoading] = useState(true)

  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [formErrors, setFormErrors] = useState({})
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    async function check() {
      const { data, error } = await apiGet(`/auth/invite/${token}`)
      if (error) {
        setError(error)
      } else {
        setUserInfo(data)
      }
      setLoading(false)
    }
    check()
  }, [token])

  const handleSubmit = async () => {
    const errs = {}
    if (!password) errs.password = 'Password is required'
    else if (password.length < 8) errs.password = 'Must be at least 8 characters'
    if (password !== confirm) errs.confirm = 'Passwords do not match'
    setFormErrors(errs)
    if (Object.keys(errs).length > 0) return

    setSaving(true)
    const { error } = await apiPost(`/auth/invite/${token}`, { password })
    setSaving(false)
    if (error) {
      setError(error)
      return
    }
    // Auto-logged in by backend — redirect to home
    window.location.href = '/'
  }

  if (loading) {
    return (
      <div class="auth-page">
        <div class="auth-card">
          <div class="loading">Verifying invite...</div>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div class="auth-page">
        <div class="auth-card">
          <h1 class="auth-title">PageFire</h1>
          <div class="form-error" style="margin-bottom: 12px">{error}</div>
          <p class="text-muted">This invite link may be expired or already used. Contact your admin for a new one.</p>
        </div>
      </div>
    )
  }

  return (
    <div class="auth-page">
      <div class="auth-card">
        <h1 class="auth-title">PageFire</h1>
        <p class="text-muted" style="margin-bottom: 16px">
          Welcome, <strong>{userInfo.name}</strong>! Set your password to activate your account.
        </p>

        <div class="form-field">
          <label class="form-label">Email</label>
          <input class="form-control" type="email" value={userInfo.email} disabled />
        </div>

        <div class="form-field">
          <label class="form-label">Password</label>
          <input
            class={`form-control${formErrors.password ? ' form-control-error' : ''}`}
            type="password"
            value={password}
            onInput={(e) => setPassword(e.target.value)}
            placeholder="Min 8 characters"
          />
          {formErrors.password && <span class="form-error">{formErrors.password}</span>}
        </div>

        <div class="form-field">
          <label class="form-label">Confirm Password</label>
          <input
            class={`form-control${formErrors.confirm ? ' form-control-error' : ''}`}
            type="password"
            value={confirm}
            onInput={(e) => setConfirm(e.target.value)}
            placeholder="Repeat password"
          />
          {formErrors.confirm && <span class="form-error">{formErrors.confirm}</span>}
        </div>

        <button class="btn btn-primary" style="width: 100%; margin-top: 12px" onClick={handleSubmit} disabled={saving}>
          {saving ? 'Setting up...' : 'Set Password & Sign In'}
        </button>
      </div>
    </div>
  )
}
