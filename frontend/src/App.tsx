import { WebLogin } from './WebLogin'
import { WebAuthenticationError } from './web-api'
import { useCallback, useEffect, useState } from 'react'
import type { Run, State } from './api'
import { desktop } from './api'
import { Brand, Button, UIProvider } from './components/ui'
import { MuseSetup, Nickname, Splash } from './Onboarding'
import { Workspace } from './Workspace'
import { flushDocument } from './document-session'

export default function App({ splashDuration = 3000 }: { splashDuration?: number } = {}) {
  const [splashFinished, setSplashFinished] = useState(splashDuration === 0)
  useEffect(() => {
    const timer = setTimeout(() => setSplashFinished(true), splashDuration)
    return () => clearTimeout(timer)
  }, [splashDuration])
  const [state, setState] = useState<State | null>(null)
  const [loading, setLoading] = useState(true)
  const [needsLogin, setNeedsLogin] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [apiKey, setApiKey] = useState('')
  const boot = useCallback(() => {
    return Promise.resolve()
      .then(() => desktop().Bootstrap())
      .then(setState)
      .catch((error) => {
        setNeedsLogin(error instanceof WebAuthenticationError)
        setError(String(error instanceof Error ? error.message : error))
      })
      .finally(() => setLoading(false))
  }, [])
  useEffect(() => {
    void boot()
  }, [boot])
  const run: Run = async (operation) => {
    setBusy(true)
    setError('')
    try {
      await flushDocument()
      setState(await operation())
      return true
    } catch (error) {
      setError(String(error instanceof Error ? error.message : error))
      return false
    } finally {
      setBusy(false)
    }
  }
  useEffect(() => {
    document.documentElement.dataset.theme = state?.preferences.theme ?? 'dark'
  }, [state?.preferences.theme])
  return (
    <UIProvider>
      {loading || !splashFinished ? (
        <Splash />
      ) : !state && window.kikoWeb && needsLogin ? (
        <WebLogin
          onLogin={() => {
            setLoading(true)
            setError('')
            void boot()
          }}
        />
      ) : !state ? (
        <main className="onboarding">
          <Brand />
          <h1>Let’s try that again.</h1>
          <p role="alert" className="error">
            {error}
          </p>
          <Button
            onClick={() => {
              setLoading(true)
              setError('')
              void boot()
            }}
          >
            Retry startup
          </Button>
        </main>
      ) : state.preferences.resetInProgress ? (
        <main className="onboarding">
          <Brand />
          <h1>Finish resetting Kiko</h1>
          <p>Reset was interrupted. Retry to remove the saved key and local data.</p>
          {error && <p role="alert">{error}</p>}
          <Button disabled={busy} onClick={() => void run(() => desktop().ResetData())}>
            {busy ? 'Resetting…' : 'Retry reset'}
          </Button>
        </main>
      ) : state.preferences.stage === 'nickname' ? (
        <Nickname state={state} busy={busy} error={error} run={run} />
      ) : state.preferences.stage !== 'done' ? (
        <MuseSetup
          state={state}
          busy={busy}
          error={error}
          apiKey={apiKey}
          setApiKey={setApiKey}
          run={run}
        />
      ) : (
        <Workspace state={state} busy={busy} error={error} run={run} onState={setState} />
      )}
    </UIProvider>
  )
}
