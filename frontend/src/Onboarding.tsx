import { useState } from 'react'
import { ArrowLeft, Check, Eye, EyeOff, KeyRound } from 'lucide-react'
import { desktop } from './api'
import type { Run, State } from './api'
import { Brand, Button, Field, Icon, IconButton } from './components/ui'

export function Splash() {
  return (
    <main className="onboarding">
      <Brand />
      <p role="status" className="muted">
        Opening your workspace…
      </p>
      <div className="startup-progress" aria-hidden="true" />
    </main>
  )
}
export function Nickname({
  state,
  busy,
  error,
  run,
}: {
  state: State
  busy: boolean
  error: string
  run: Run
}) {
  const [name, setName] = useState(state.preferences.nickname)
  return (
    <main className="onboarding">
      <section className="onboarding-form">
        <Brand />
        <p className="setup-step">
          01 <span /> Your workspace
        </p>
        <h1>What should I call you?</h1>
        <p className="muted">A name to make this space yours.</p>
        <form
          onSubmit={(event) => {
            event.preventDefault()
            void run(() => desktop().SaveNickname(name))
          }}
        >
          <Field
            label="Nickname"
            autoComplete="nickname"
            autoFocus
            value={name}
            onChange={(event) => setName(event.target.value)}
            required
            maxLength={40}
            disabled={busy}
            placeholder="Your nickname"
          />
          <p className="field-hint">1–40 characters. You can change this later.</p>
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          <Button type="submit" disabled={busy || !name.trim()} className="full-width">
            {busy ? 'Saving…' : 'Continue'}
          </Button>
        </form>
      </section>
    </main>
  )
}
export function MuseSetup({
  state,
  busy,
  error,
  apiKey,
  setApiKey,
  run,
}: {
  state: State
  busy: boolean
  error: string
  apiKey: string
  setApiKey(value: string): void
  run: Run
}) {
  const [visible, setVisible] = useState(false)
  return (
    <main className="onboarding">
      <section className="onboarding-form">
        <Brand />
        <p className="setup-step">
          02 <span /> Connect your AI
        </p>
        <h1>{state.preferences.stage === 'done' ? 'Reconnect Meta Muse' : 'Connect Meta Muse'}</h1>
        <p className="muted">
          Hi {state.preferences.nickname}. Bring your own API key to start planning with Muse.
        </p>
        {state.connectionIssue && (
          <p role="status" className="verification-note">
            {state.connectionIssue}
          </p>
        )}
        <div className="provider">
          <span className="provider-icon">
            <Icon icon={KeyRound} size={22} />
          </span>
          <div>
            <strong>Meta Muse</strong>
            <p>Pick Muse Spark, Ember, or Wonder for each chat after you connect.</p>
            <p className="field-hint">
              Contributor models are discounted and let Meta use submitted content and responses to
              train its models. Standard models do not.
            </p>
          </div>
          <Icon icon={Check} />
        </div>
        <form
          onSubmit={async (event) => {
            event.preventDefault()
            if (await run(() => desktop().ConnectMuse(apiKey))) setApiKey('')
          }}
        >
          <div className="key-field">
            <Field
              label="API key"
              type={visible ? 'text' : 'password'}
              autoComplete="off"
              spellCheck={false}
              value={apiKey}
              onChange={(event) => setApiKey(event.target.value)}
              disabled={busy}
              required
              placeholder="Paste your Meta API key"
            />
            <IconButton
              icon={visible ? EyeOff : Eye}
              label={visible ? 'Hide key' : 'Show key'}
              onClick={() => setVisible(!visible)}
            />
          </div>
          <p className="field-hint">Your key is saved securely in your computer’s keychain.</p>
          <button
            type="button"
            className="text-link"
            onClick={() => void desktop().OpenMuseGuide()}
          >
            How to get a Meta API key
          </button>
          <p className="verification-note">
            Connecting sends a short request to Meta and may use your API credits.
          </p>
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          <Button className="full-width" type="submit" disabled={busy || !apiKey.trim()}>
            {busy ? 'Connecting…' : 'Connect and continue'}
          </Button>
          <Button
            className="back-button"
            variant="ghost"
            disabled={busy}
            onClick={() => void run(() => desktop().Back())}
          >
            <Icon icon={ArrowLeft} />
            Back
          </Button>
        </form>
      </section>
    </main>
  )
}
