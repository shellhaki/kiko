import { useState } from 'react'
import { Brand, Button, Field } from './components/ui'
import { webLogin } from './web-api'
export function WebLogin({ onLogin }: { onLogin(): void }) {
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  return (
    <main className="onboarding">
      <Brand />
      <h1>Your personal workspace</h1>
      <p className="muted">Sign in with the login token configured on your Kiko server.</p>
      <form
        onSubmit={async (event) => {
          event.preventDefault()
          const token = String(new FormData(event.currentTarget).get('token'))
          setBusy(true)
          setError('')
          try {
            await webLogin(token)
            onLogin()
          } catch (problem) {
            setError(String(problem))
          } finally {
            setBusy(false)
          }
        }}
      >
        <Field
          label="Server login token"
          name="token"
          type="password"
          autoComplete="current-password"
          required
          disabled={busy}
        />
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <Button type="submit" disabled={busy}>
          {busy ? 'Signing in…' : 'Sign in'}
        </Button>
      </form>
    </main>
  )
}
