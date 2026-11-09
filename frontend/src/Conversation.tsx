import { Attachments } from './Attachments'
import { flushDocument } from './document-session'
import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  Folder,
  Laptop,
  Server,
  SlidersHorizontal,
  ShieldCheck,
  LoaderCircle,
  RotateCcw,
  Square,
} from 'lucide-react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { ChatMessage, Run, State } from './api'
import { desktop } from './api'
import { mergeMessages, requestId } from './conversation-state'
import { Brand, Button, Icon, IconButton } from './components/ui'

function Prose({ content }: { content: string }) {
  return (
    <Markdown
      remarkPlugins={[remarkGfm]}
      skipHtml
      components={{
        img: ({ alt }) => <span className="muted">[Image: {alt || 'image'}]</span>,
        a: ({ href, children }) =>
          href && /^https?:\/\//i.test(href) ? (
            <a
              href={href}
              target="_blank"
              rel="noopener noreferrer"
              onClick={(event) => {
                event.preventDefault()
                void desktop()
                  .OpenLink(href)
                  .catch(() => {})
              }}
            >
              {children}
            </a>
          ) : (
            <span>{children}</span>
          ),
      }}
    >
      {content}
    </Markdown>
  )
}

export function Conversation({
  state,
  heading,
  subtitle,
  onState,
  run,
  onReconnect,
  onPreferences,
}: {
  state: State
  heading: string
  subtitle: string
  onState(value: State): void
  run: Run
  onReconnect?(): void
  onPreferences?(): void
}) {
  const project = state.selection.projectId
  const chat = state.selection.chatId
  const [messages, setMessages] = useState(state.messages)
  const [draft, setDraft] = useState(state.draft)
  const [error, setError] = useState('')
  const [sending, setSending] = useState(false)
  const [stopping, setStopping] = useState('')
  const [saving, setSaving] = useState(false)
  const [jump, setJump] = useState(false)
  const draftValue = useRef(state.draft)
  const edited = useRef(false)
  const mounted = useRef(true)
  const queue = useRef<Promise<void>>(Promise.resolve())
  const saveNumber = useRef(0)
  const inFlight = useRef(false)
  const pendingRequest = useRef<{ text: string; id: string } | null>(null)
  const thread = useRef<HTMLDivElement>(null)
  const textarea = useRef<HTMLTextAreaElement>(null)
  const follow = useRef(true)
  const composing = useRef(false)
  const activityStamp = useRef('')
  const running = messages.find((message) => message.status === 'running')
  const stoppingThisRun = !!running && stopping === `${running.runId}:${running.attempt}`
  const isRunning = !!running
  const lastUser = messages.filter((message) => message.role === 'user').at(-1)

  useEffect(() => {
    mounted.current = true
    const off = window.runtime?.EventsOnMultiple(
      'kiko:message',
      (message: ChatMessage) => {
        if (message.chatId !== chat) return
        setMessages((current) => mergeMessages(current, [message]))
        const stamp = JSON.stringify(message.steps?.map((step) => [step.id, step.status]))
        if (
          stamp !== activityStamp.current &&
          message.steps?.some((step) => step.status === 'ok')
        ) {
          activityStamp.current = stamp
          void desktop()
            .Bootstrap()
            .then((value) => {
              if (mounted.current) onState(value)
            })
            .catch(() => {})
        }
      },
      -1,
    )
    void desktop()
      .Conversation(project, chat)
      .then((value) => {
        if (!mounted.current) return
        setMessages((current) => mergeMessages(current, value.messages))
        if (!edited.current) {
          draftValue.current = value.draft
          setDraft(value.draft)
        }
      })
      .catch(() => {
        if (mounted.current) setError('Couldn’t load this conversation. Reopen it to retry.')
      })
    return () => {
      mounted.current = false
      off?.()
    }
  }, [project, chat, onState])
  useEffect(() => {
    if (!isRunning) return
    const timer = setInterval(() => {
      void desktop()
        .Conversation(project, chat)
        .then((value) => {
          if (mounted.current) setMessages((current) => mergeMessages(current, value.messages))
        })
        .catch(() => {})
    }, 2000)
    return () => clearInterval(timer)
  }, [isRunning, project, chat])
  useLayoutEffect(() => {
    if (follow.current && thread.current) thread.current.scrollTop = thread.current.scrollHeight
  }, [messages])
  useLayoutEffect(() => {
    if (textarea.current) {
      textarea.current.style.height = 'auto'
      textarea.current.style.height = `${Math.min(textarea.current.scrollHeight, 180)}px`
    }
  }, [draft])

  const saveDraft = (value: string) => {
    draftValue.current = value
    edited.current = true
    setDraft(value)
    setSaving(true)
    const number = ++saveNumber.current
    queue.current = queue.current
      .catch(() => {})
      .then(() => desktop().SaveDraft(project, chat, value))
    void queue.current
      .catch(() => {
        if (mounted.current)
          setError('Couldn’t save your draft. Keep this window open and try again.')
      })
      .finally(() => {
        if (mounted.current && saveNumber.current === number) setSaving(false)
      })
  }
  const refresh = async () => {
    const value = await desktop().Conversation(project, chat)
    if (mounted.current) setMessages((current) => mergeMessages(current, value.messages))
  }
  const send = async () => {
    const text = draftValue.current.trim()
    if (!text || inFlight.current || running || !state.connected) return
    inFlight.current = true
    setSending(true)
    setError('')
    follow.current = true
    setJump(false)
    if (pendingRequest.current?.text !== text) pendingRequest.current = { text, id: requestId() }
    const request = pendingRequest.current
    try {
      await flushDocument()
      await queue.current.catch(() => {})
      await desktop().SaveDraft(project, chat, draftValue.current)
      const next = await desktop().SendMessage(project, chat, text, request.id)
      draftValue.current = ''
      edited.current = true
      setDraft('')
      pendingRequest.current = null
      setMessages((current) => mergeMessages(current, next.messages))
      onState(next)
    } catch (problem) {
      setError(String(problem instanceof Error ? problem.message : problem))
      void refresh().catch(() => {})
    } finally {
      inFlight.current = false
      if (mounted.current) setSending(false)
    }
  }
  const retry = async (message: ChatMessage) => {
    if (running || inFlight.current) return
    inFlight.current = true
    setSending(true)
    setError('')
    try {
      await flushDocument()
      const next = await desktop().RetryMessage(message.runId, requestId())
      setMessages((current) => mergeMessages(current, next.messages))
      onState(next)
    } catch (problem) {
      setError(String(problem instanceof Error ? problem.message : problem))
      void refresh().catch(() => {})
    } finally {
      inFlight.current = false
      if (mounted.current) setSending(false)
    }
  }
  const stop = async () => {
    if (!running || stoppingThisRun) return
    setStopping(`${running.runId}:${running.attempt}`)
    try {
      await desktop().StopRun(running.runId, running.attempt)
    } catch {
      setError('Couldn’t send Stop. Try again.')
      setStopping('')
    }
  }
  return (
    <section
      className={`conversation-shell ${messages.length ? '' : 'empty'}`}
      aria-label="Conversation"
    >
      {messages.length === 0 ? (
        <div className="conversation-welcome">
          <Brand compact />
          <h1>{heading}</h1>
          {subtitle && <p className="muted">{subtitle}</p>}
        </div>
      ) : (
        <div className="conversation-history">
          <div
            className="conversation-thread"
            ref={thread}
            onScroll={(event) => {
              const element = event.currentTarget
              follow.current = element.scrollHeight - element.scrollTop - element.clientHeight < 80
              setJump(!follow.current)
            }}
          >
            <div className="message-column">
              {messages.map((message) => (
                <article
                  key={message.id}
                  className={`message ${message.role}`}
                  aria-label={message.role === 'user' ? 'Your message' : 'Muse reply'}
                >
                  {message.role === 'assistant' && (
                    <div className="assistant-byline">
                      <Brand compact />
                      <strong>Kiko</strong>
                      {message.status === 'running' && <Icon icon={LoaderCircle} size={14} />}
                    </div>
                  )}
                  <div className={message.role === 'assistant' ? 'prose' : 'user-bubble'}>
                    {message.content ? (
                      message.role === 'assistant' ? (
                        <Prose content={message.content} />
                      ) : (
                        message.content
                      )
                    ) : message.status === 'running' ? (
                      <p className="muted">Muse is responding…</p>
                    ) : null}
                  </div>
                  {message.role === 'assistant' && (
                    <div className="reply-footer">
                      {!!message.steps?.length && (
                        <details className="tool-activity" open={message.status === 'running'}>
                          <summary>
                            {message.status === 'running'
                              ? 'Working on your document'
                              : `${message.steps.length} steps completed`}
                          </summary>
                          <ol>
                            {message.steps.map((step) => (
                              <li key={step.id} data-status={step.status}>
                                <span className="activity-dot" />
                                {step.label}
                                <small>
                                  {step.status === 'running'
                                    ? 'In progress'
                                    : step.status === 'ok'
                                      ? 'Done'
                                      : step.status === 'refused'
                                        ? 'Needs adjustment'
                                        : 'Stopped'}
                                </small>
                              </li>
                            ))}
                          </ol>
                        </details>
                      )}
                      {message.status === 'stopped' && <span>Stopped · partial reply saved</span>}
                      {message.status === 'interrupted' && <span>Interrupted</span>}
                      {message.error && <p className="error">{message.error}</p>}
                      {!running &&
                        message.status !== 'complete' &&
                        message.status !== 'running' &&
                        message.ordinal === (lastUser?.ordinal ?? -2) + 1 && (
                          <Button
                            variant="ghost"
                            disabled={sending || !state.connected}
                            onClick={() => void retry(message)}
                          >
                            <Icon icon={RotateCcw} size={14} />
                            Retry reply
                          </Button>
                        )}
                      {message.previous.length > 0 && (
                        <details className="previous-attempts">
                          <summary>Previous attempts ({message.previous.length})</summary>
                          {message.previous.map((attempt, index) => (
                            <div className="previous-attempt" key={index}>
                              <small>{attempt.status}</small>
                              <Prose content={attempt.content || 'No response was received.'} />
                            </div>
                          ))}
                        </details>
                      )}
                      {message.errorKind === 'auth' && (
                        <Button
                          variant="secondary"
                          onClick={() => {
                            if (state.provider?.id && state.provider.id !== 'muse' && onReconnect)
                              onReconnect()
                            else void run(() => desktop().ReconnectMuse())
                          }}
                        >
                          Reconnect Meta Muse
                        </Button>
                      )}
                    </div>
                  )}
                </article>
              ))}
            </div>
          </div>
          {jump && (
            <Button
              variant="secondary"
              className="jump-latest"
              onClick={() => {
                follow.current = true
                setJump(false)
                if (thread.current) thread.current.scrollTop = thread.current.scrollHeight
              }}
            >
              <Icon icon={ArrowDown} size={14} />
              Jump to latest
            </Button>
          )}
        </div>
      )}
      <div className="composer-dock">
        <div aria-live="polite" className="conversation-status">
          {running
            ? 'Muse is responding'
            : messages.length > 0
              ? messages.some((message) => message.errorKind === 'storage')
                ? 'Saving needs attention'
                : 'Conversation saved locally'
              : ''}
        </div>
        {!state.connected && (
          <div className="connection-warning">
            <p>
              {state.connectionIssue ||
                'Reconnect Meta Muse to send messages. Your conversations are still available.'}
            </p>
            <Button
              variant="secondary"
              onClick={() => {
                if (state.provider?.id && state.provider.id !== 'muse' && onReconnect) onReconnect()
                else void run(() => desktop().ReconnectMuse())
              }}
            >
              Reconnect Meta Muse
            </Button>
          </div>
        )}
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <div className="composer-context">
          <span className="composer-project">
            <Icon icon={Folder} size={16} />
            {state.projects.find((item) => item.id === project)?.name ?? 'Personal workspace'}
          </span>
          <span className="composer-device">
            <Icon icon={window.kikoWeb ? Server : Laptop} size={16} />
            {window.kikoWeb ? 'This server' : 'This computer'}
          </span>
          <span className="spacer" />
          {onPreferences && (
            <IconButton
              icon={SlidersHorizontal}
              label="Conversation settings"
              onClick={onPreferences}
            />
          )}
        </div>
        <form
          className="composer"
          onSubmit={(event) => {
            event.preventDefault()
            void send()
          }}
        >
          <Attachments chat={chat} disabled={sending || isRunning} />
          <textarea
            ref={textarea}
            rows={1}
            aria-label="Message"
            placeholder={state.connected ? 'Ask, plan, create…' : 'Write a draft…'}
            value={draft}
            disabled={sending}
            maxLength={128 * 1024}
            onChange={(event) => saveDraft(event.target.value)}
            onCompositionStart={() => {
              composing.current = true
            }}
            onCompositionEnd={() => {
              composing.current = false
            }}
            onKeyDown={(event) => {
              if (
                event.key === 'Enter' &&
                !event.shiftKey &&
                !composing.current &&
                !event.nativeEvent.isComposing &&
                event.nativeEvent.keyCode !== 229
              ) {
                event.preventDefault()
                void send()
              }
            }}
          />
          <div className="composer-footer">
            <span
              className="edit-mode"
              title={
                state.preferences.suggestionMode
                  ? 'Review proposed document changes before applying them'
                  : 'AI edits are applied directly to your documents'
              }
            >
              <Icon icon={ShieldCheck} size={15} />
              {state.preferences.suggestionMode ? 'Review edits' : 'Direct edits'}
            </span>
            <span className="spacer" />
            <button
              type="button"
              className="model-label"
              aria-label="Model settings"
              onClick={onReconnect}
              disabled={!onReconnect}
              title={state.provider?.model ?? 'Muse Spark 1.3 Contributor'}
            >
              {state.provider?.id && state.provider.id !== 'muse'
                ? state.provider.id
                : 'Muse Spark'}
              <span>
                {' '}
                ·{' '}
                {state.provider?.id && state.provider.id !== 'muse'
                  ? state.provider.model
                  : '1.3 Contributor'}
              </span>
              <Icon icon={ChevronDown} size={12} />
            </button>
            <span className="draft-status">{saving ? 'Saving draft…' : ''}</span>
            {running ? (
              <IconButton
                icon={Square}
                label={stoppingThisRun ? 'Stopping reply' : 'Stop reply'}
                disabled={stoppingThisRun}
                onClick={() => void stop()}
              />
            ) : (
              <IconButton
                icon={ArrowUp}
                label="Send message"
                type="submit"
                disabled={sending || !state.connected || !draft.trim()}
              />
            )}
          </div>
        </form>
        <p className="composer-hint">Muse can make mistakes. Review important details.</p>
      </div>
    </section>
  )
}
