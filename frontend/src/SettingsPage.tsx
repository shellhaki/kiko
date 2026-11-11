import { useRef, useState } from 'react'
import { ArrowLeft, Database, Globe, KeyRound, SlidersHorizontal } from 'lucide-react'
import type { Run, State } from './api'
import { desktop } from './api'
import { Brand, Button, Field, Icon } from './components/ui'

export function SettingsPage({
  state,
  busy,
  error,
  run,
  section,
  onSectionChange,
  onBack,
  onReset,
}: {
  state: State
  busy: boolean
  error: string
  run: Run
  section: string
  onSectionChange(section: string): void
  onBack(): void
  onReset(): void
}) {
  const { preferences } = state
  const width = preferences.sidebarWidth
  const content = useRef<HTMLDivElement>(null)
  const appearance = (open: boolean, size = width, theme = preferences.theme) =>
    run(() => desktop().SaveAppearance(theme, open, size))
  const [replacementKey, setReplacementKey] = useState('')
  const [researchKey, setResearchKey] = useState('')
  const [showKey, setShowKey] = useState(false)
  return (
    <section className="settings-page" aria-label="Settings">
      <header className="settings-header">
        <Button
          variant="ghost"
          className="icon-button"
          aria-label="Back"
          autoFocus
          onClick={onBack}
        >
          <Icon icon={ArrowLeft} size={18} />
        </Button>
        <div>
          <h1>Settings</h1>
          <p className="muted">Preferences, connections, and local data.</p>
        </div>
      </header>
      <div className="settings-layout">
        <nav className="settings-navigation" aria-label="Settings sections">
          {[
            { name: 'General', icon: SlidersHorizontal },
            { name: 'API key', icon: KeyRound },
            { name: 'Research', icon: Globe },
            { name: 'Data', icon: Database },
          ].map(({ name, icon }) => (
            <button
              key={name}
              aria-current={section === name ? 'page' : undefined}
              onClick={() => {
                onSectionChange(name)
                if (content.current) content.current.scrollTop = 0
              }}
            >
              <Icon icon={icon} size={17} />
              {name}
            </button>
          ))}
          <div className="settings-about">
            <Brand compact />
            <span>Kiko</span>
          </div>
        </nav>
        <div className="settings-content" ref={content}>
          <section hidden={section !== 'General'} aria-label="General settings">
            <h2>General</h2>
            <p className="settings-intro">Your name, appearance, and editing preferences.</p>
            {window.kikoWeb && (
              <Button
                variant="ghost"
                onClick={() =>
                  void fetch('/api/logout', { method: 'POST' }).then(() => window.location.reload())
                }
              >
                Sign out of web workspace
              </Button>
            )}
            <form
              onSubmit={async (event) => {
                event.preventDefault()
                const data = new FormData(event.currentTarget)
                await run(() => desktop().SaveNickname(String(data.get('nickname'))))
              }}
            >
              <Field
                label="Nickname"
                name="nickname"
                defaultValue={preferences.nickname}
                required
                disabled={busy}
              />
              <Button type="submit" variant="secondary" disabled={busy}>
                Save nickname
              </Button>
            </form>
            <fieldset className="theme-picker">
              <legend>Appearance</legend>
              {(['dark', 'light', 'system'] as const).map((theme) => (
                <label key={theme} className={`theme-option ${theme}`}>
                  <span className="theme-preview" aria-hidden="true">
                    <i />
                    <i />
                    <i />
                  </span>
                  <span>
                    <input
                      type="radio"
                      name="theme"
                      value={theme}
                      checked={preferences.theme === theme}
                      disabled={busy}
                      onChange={() => void appearance(preferences.sidebarOpen, width, theme)}
                    />
                    {theme[0].toUpperCase() + theme.slice(1)}
                  </span>
                </label>
              ))}
            </fieldset>
          </section>
          <section hidden={section !== 'API key'} aria-label="API key settings">
            <h2>API key</h2>
            <p className="settings-intro">
              Kiko connects to Meta Muse. Choose the model for each chat from its composer.
            </p>
            <div className="connection-summary">
              <Icon icon={KeyRound} />
              <div>
                <strong>Meta Muse</strong>
                <p className="muted">{state.connected ? 'Connected' : 'Not connected'}</p>
              </div>
            </div>
            <p className="field-hint">
              {window.kikoWeb
                ? 'Keys are encrypted on this Kiko server. Saved keys are never sent back to your browser.'
                : 'Your key is kept in your operating system’s secure keychain.'}
            </p>
            <form
              className="key-settings"
              onSubmit={async (event) => {
                event.preventDefault()
                if (await run(() => desktop().ConnectMuse(replacementKey))) setReplacementKey('')
              }}
            >
              <Field
                label={state.connected ? 'Replace API key' : 'API key'}
                type={showKey ? 'text' : 'password'}
                value={replacementKey}
                onChange={(event) => setReplacementKey(event.target.value)}
                autoComplete="off"
                disabled={busy}
              />
              <Button variant="ghost" onClick={() => setShowKey(!showKey)}>
                {showKey ? 'Hide key' : 'Show key'}
              </Button>
              <p className="field-hint">
                Verifying sends a short request to Meta and may use your API credits. Your existing
                key stays until the replacement is verified.
              </p>
              <Button type="submit" variant="secondary" disabled={busy || !replacementKey.trim()}>
                Verify and save key
              </Button>
              {state.connected && (
                <Button
                  variant="ghost"
                  disabled={busy}
                  onClick={() => void run(() => desktop().RemoveMuseKey())}
                >
                  Remove saved key
                </Button>
              )}
            </form>
            <p className="field-hint">
              Contributor models offer discounted usage and allow Meta to use submitted content and
              responses for model training. Standard models do not.
            </p>
          </section>
          <section
            hidden={section !== 'Research'}
            aria-label="Research settings"
            className="key-settings"
          >
            <h2>Web research</h2>
            <p className="field-hint">
              Optional Tavily Basic search and page extraction. Calls use your Tavily credits;
              source excerpts are sent to your selected model. At most five research calls per turn.
            </p>
            <p>
              {preferences.tavilyEnabled ? 'Tavily key saved' : 'Not configured'} ·{' '}
              {
                state.messages
                  .flatMap((message) => message.steps ?? [])
                  .filter((step) => step.tool === 'web_search' || step.tool === 'web_fetch').length
              }{' '}
              calls in this chat
            </p>
            <form
              onSubmit={async (event) => {
                event.preventDefault()
                if (await run(() => desktop().ConfigureResearch(researchKey))) setResearchKey('')
              }}
            >
              <Field
                label="Tavily API key"
                type="password"
                value={researchKey}
                onChange={(event) => setResearchKey(event.target.value)}
                autoComplete="off"
              />
              <Button type="submit" variant="secondary" disabled={busy || !researchKey.trim()}>
                Save research key
              </Button>
              {preferences.tavilyEnabled && (
                <Button
                  variant="ghost"
                  disabled={busy}
                  onClick={() => void run(() => desktop().RemoveResearch())}
                >
                  Remove research key
                </Button>
              )}
            </form>
          </section>
          <section hidden={section !== 'Data'} aria-label="Data settings" className="settings-data">
            <h2>Data</h2>
            <p className="field-hint">
              Remove local chats, documents, history, preferences, and saved keys. Files in selected
              folders are kept.
            </p>
            <Button variant="danger" onClick={() => onReset()}>
              Reset Kiko
            </Button>
          </section>
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
        </div>
      </div>
    </section>
  )
}
