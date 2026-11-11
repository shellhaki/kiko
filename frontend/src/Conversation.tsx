import { FileCards, WorkspaceFiles } from './ChatFiles'
import { replyFiles, chatFiles, recentFiles, greeting } from './chat-files'
import { Attachments } from './Attachments'
import { DeleteApproval } from './DeleteApproval'
import { flushDocument, registerConversationSave } from './document-session'
import { memo, useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react'
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  Folder,
  SlidersHorizontal,
  LoaderCircle,
  RotateCcw,
  Square,
} from 'lucide-react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import type { ChatMessage, Run, State } from './api'
import { desktop } from './api'
import { mergeMessages, requestId } from './conversation-state'
import { Brand, Button, CopyButton, Icon, IconButton, Select } from './components/ui'

const Prose = memo(function Prose({ content }: { content: string }) {
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
})

export function Conversation({
  state,
  heading,
  subtitle,
  onState,
  run,
  onPreferences,
  onOpenFile,
}: {
  onOpenFile?(path: string): void
  state: State
  heading: string
  subtitle: string
  onState(value: State): void
  run: Run
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
  const draftTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const pendingDraft = useRef<(() => void) | null>(null)
  const inFlight = useRef(false)
  const pendingRequest = useRef<{ text: string; id: string } | null>(null)
  const thread = useRef<HTMLDivElement>(null)
  const automaticScroll = useRef(false)
  const scrollStarted = useRef(false)
  const attachmentInput = useRef<HTMLInputElement>(null)
  const textarea = useRef<HTMLTextAreaElement>(null)
  const follow = useRef(true)
  const composing = useRef(false)
  const activityStamp = useRef('')
  const folder = state.projects.find((item) => item.id === project)
  const hasContent = messages.length > 0 || chatFiles(state).length > 0
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
  const scrollLatest = useCallback((smooth = true) => {
    const element = thread.current
    if (!element) return
    const reduced = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches
    automaticScroll.current = true
    if (element.scrollTo)
      element.scrollTo({
        top: element.scrollHeight,
        behavior: smooth && !reduced ? 'smooth' : 'instant',
      })
    else {
      element.scrollTop = element.scrollHeight
      automaticScroll.current = false
    }
  }, [])
  const interruptScroll = () => {
    automaticScroll.current = false
    const element = thread.current
    if (element?.scrollTo) element.scrollTo({ top: element.scrollTop, behavior: 'instant' })
    follow.current = false
    requestAnimationFrame(() => {
      if (!element || !mounted.current) return
      follow.current = element.scrollHeight - element.scrollTop - element.clientHeight < 80
      setJump(!follow.current)
    })
  }
  useEffect(() => {
    const column = thread.current?.firstElementChild
    if (!column || typeof ResizeObserver === 'undefined') return
    const observer = new ResizeObserver(() => {
      if (follow.current) scrollLatest()
    })
    observer.observe(column)
    return () => observer.disconnect()
  }, [hasContent, scrollLatest])
  useLayoutEffect(() => {
    if (follow.current) scrollLatest(scrollStarted.current)
    scrollStarted.current = true
  }, [messages, state.documents.length, state.artifacts?.length, scrollLatest])
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
    if (draftTimer.current) clearTimeout(draftTimer.current)
    pendingDraft.current = () => {
      pendingDraft.current = null
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
    draftTimer.current = setTimeout(() => pendingDraft.current?.(), 200)
  }
  const flushDraft = useCallback(async () => {
    if (draftTimer.current) clearTimeout(draftTimer.current)
    pendingDraft.current?.()
    await queue.current
  }, [])
  useEffect(() => registerConversationSave(flushDraft), [flushDraft])
  useEffect(
    () => () => {
      if (draftTimer.current) clearTimeout(draftTimer.current)
      pendingDraft.current?.()
    },
    [],
  )

  const importFile = (name: string, data: string) =>
    run(async () => {
      await flushDraft()
      let nextChat = chat
      try {
        if (!nextChat) {
          const created = await desktop().CreateChat(project)
          nextChat = created.selection.chatId
          if (!nextChat) throw new Error('Couldn’t create a chat for this import.')
          await desktop().SaveDraft('', nextChat, draftValue.current)
        }
        await desktop().AttachFile(nextChat, name, data)
        if (!chat) await desktop().SaveDraft(project, '', '')
        return await desktop().Bootstrap()
      } catch (problem) {
        if (nextChat && !chat) onState(await desktop().Bootstrap())
        throw problem
      }
    })
  const chooseFolder = () =>
    run(async () => {
      await flushDraft()
      const next = await desktop().ChooseFolder(chat)
      if (!chat && next.selection.projectId !== project) {
        await desktop().SaveDraft(next.selection.projectId, '', draftValue.current)
        return desktop().Bootstrap()
      }
      return next
    })
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
      await flushDraft()
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
      className={`conversation-shell ${hasContent ? '' : 'empty'}`}
      aria-label="Conversation"
    >
      {!hasContent ? (
        <div className="conversation-welcome">
          <p className="welcome-greeting">{greeting(state.preferences.nickname)}</p>
          <h1>{heading}</h1>
          {subtitle && <p className="muted">{subtitle}</p>}
          {recentFiles(state).length > 0 && (
            <section className="home-recents" aria-label="Recent files">
              <h2>Recent</h2>
              <FileCards files={recentFiles(state)} run={run} />
            </section>
          )}
        </div>
      ) : (
        <div className="conversation-history">
          <div
            className="conversation-thread"
            ref={thread}
            onWheel={interruptScroll}
            onTouchStart={interruptScroll}
            onPointerDown={interruptScroll}
            onKeyDown={(event) => {
              if (['ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End'].includes(event.key))
                interruptScroll()
            }}
            onScroll={(event) => {
              const element = event.currentTarget
              const atBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 80
              if (automaticScroll.current && !atBottom) return
              automaticScroll.current = false
              follow.current = atBottom
              setJump(!follow.current)
            }}
          >
            <div className="message-column">
              {messages.map((message) => (
                <article
                  key={message.id}
                  className={`message ${message.role}`}
                  aria-label={message.role === 'user' ? 'Your message' : 'Kiko reply'}
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
                      <p className="muted">Kiko is responding…</p>
                    ) : null}
                  </div>
                  {message.role === 'user' && message.content && (
                    <div className="message-actions">
                      <CopyButton text={message.content} label="Copy message" />
                    </div>
                  )}
                  {message.role === 'assistant' && (
                    <FileCards files={replyFiles(state, messages, message)} run={run} />
                  )}
                  {message.role === 'assistant' && (
                    <div className="reply-footer">
                      {message.content && (
                        <CopyButton text={message.content} label="Copy response" />
                      )}
                      {!!message.steps?.length && (
                        <details className="tool-activity" open={message.status === 'running'}>
                          <summary>
                            {message.status === 'running'
                              ? 'Working on your request'
                              : `${message.steps.length} ${message.steps.length === 1 ? 'step' : 'steps'}`}
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
                          onClick={() => void run(() => desktop().ReconnectMuse())}
                        >
                          Reconnect Meta Muse
                        </Button>
                      )}
                    </div>
                  )}
                </article>
              ))}
              {!!replyFiles(state, messages).length && (
                <section className="chat-file-group" aria-label="Files in this chat">
                  <h2>Files in this chat</h2>
                  <FileCards files={replyFiles(state, messages)} run={run} />
                </section>
              )}
              {folder?.root && (
                <WorkspaceFiles
                  chat={chat}
                  onOpen={onOpenFile}
                  stamp={JSON.stringify(messages.map((message) => message.steps))}
                />
              )}
            </div>
          </div>
          {jump && (
            <Button
              variant="secondary"
              className="jump-latest"
              onClick={() => {
                follow.current = true
                setJump(false)
                scrollLatest()
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
            ? 'Kiko is responding'
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
            <Button variant="secondary" onClick={() => void run(() => desktop().ReconnectMuse())}>
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
          <button
            type="button"
            className="composer-project context-button"
            disabled={sending || isRunning || !!window.kikoWeb}
            title={
              window.kikoWeb
                ? 'Local folder access is available in the desktop app.'
                : folder?.root || 'Choose a folder to give this chat a project root'
            }
            onClick={() => void chooseFolder()}
          >
            <Icon icon={Folder} size={16} />
            {folder?.name ?? 'This workspace'}
            <Icon icon={ChevronDown} size={12} />
          </button>
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
          <Attachments
            chat={chat}
            disabled={sending || isRunning}
            inputRef={attachmentInput}
            importFile={importFile}
          />
          <textarea
            ref={textarea}
            rows={1}
            aria-label="Message"
            aria-keyshortcuts="Enter Shift+Enter"
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
            <DeleteApproval chat={state.selection.chatId} />
            <span className="spacer" />
            <Select
              className="model-select"
              label="Model"
              value={state.model}
              disabled={!!running || sending}
              onChange={(model) => void run(() => desktop().SetModel(model))}
              options={state.models.map((model) => ({
                value: model.id,
                label: `${model.name} · ${model.mode === 'contributor' ? 'Contributor' : 'Standard'}`,
                hint: model.description,
              }))}
            />
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
        <p className="composer-hint">
          Kiko can make mistakes. Review important details. Enter to send · Shift+Enter for a new
          line.
        </p>
      </div>
    </section>
  )
}
