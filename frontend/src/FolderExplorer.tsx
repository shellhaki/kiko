import { useCallback, useEffect, useRef, useState } from 'react'
import { ChevronDown, ChevronRight, FileText, Folder, FolderOpen, RefreshCw } from 'lucide-react'
import { desktop } from './api'
import { Icon, IconButton } from './components/ui'

type Entry = { name: string; directory: boolean }
export function FolderExplorer({
  project,
  onOpen,
}: {
  project: string
  onOpen(path: string): void
}) {
  const [expanded, setExpanded] = useState(new Set(['.']))
  const [entries, setEntries] = useState<Record<string, Entry[]>>({})
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const mounted = useRef(true)
  const serial = useRef(0)
  useEffect(() => {
    mounted.current = true
    return () => {
      mounted.current = false
    }
  }, [])
  const refresh = useCallback(
    async (active: () => boolean) => {
      const request = ++serial.current
      const results = await Promise.allSettled(
        [...expanded].map(async (path) => ({
          path,
          entries: await desktop().ListWorkspaceDirectory(project, path),
        })),
      )
      if (!active() || !mounted.current || request !== serial.current) return
      const nextEntries: Record<string, Entry[]> = {}
      const nextErrors: Record<string, string> = {}
      results.forEach((result, index) => {
        const path = [...expanded][index]
        if (result.status === 'fulfilled') nextEntries[path] = result.value.entries
        else
          nextErrors[path] = String(
            result.reason instanceof Error ? result.reason.message : result.reason,
          )
      })
      setEntries((current) => ({ ...current, ...nextEntries }))
      setErrors(nextErrors)
      setLoading(false)
    },
    [expanded, project],
  )
  useEffect(() => {
    let active = true
    void refresh(() => active)
    const timer = setInterval(() => {
      if (!document.hidden) void refresh(() => active)
    }, 3000)
    return () => {
      active = false
      clearInterval(timer)
    }
  }, [refresh])
  const toggle = (path: string) =>
    setExpanded((current) => {
      const next = new Set(current)
      if (next.has(path)) {
        for (const item of next) if (item === path || item.startsWith(path + '/')) next.delete(item)
      } else if (next.size < 20) next.add(path)
      return next
    })
  const branch = (directory: string, depth: number): React.ReactNode => (
    <ul className="file-tree-branch">
      {errors[directory] && (
        <li className="file-tree-error" role="status">
          {errors[directory]}
        </li>
      )}
      {!errors[directory] && entries[directory]?.length === 0 && (
        <li className="empty-list">
          {directory === '.' ? 'No files in this folder.' : 'Empty folder'}
        </li>
      )}
      {entries[directory]?.map((entry) => {
        const path = directory === '.' ? entry.name : directory + '/' + entry.name
        const open = expanded.has(path)
        return (
          <li key={path}>
            <button
              type="button"
              className="file-tree-row"
              style={{ paddingLeft: 8 + depth * 12 }}
              title={path}
              aria-expanded={entry.directory ? open : undefined}
              onClick={() => (entry.directory ? toggle(path) : onOpen(path))}
              onKeyDown={(event) => {
                if (!entry.directory) return
                if ((event.key === 'ArrowRight' && !open) || (event.key === 'ArrowLeft' && open)) {
                  event.preventDefault()
                  toggle(path)
                }
              }}
            >
              <span className="file-tree-disclosure">
                {entry.directory && <Icon icon={open ? ChevronDown : ChevronRight} size={12} />}
              </span>
              <Icon icon={entry.directory ? (open ? FolderOpen : Folder) : FileText} size={15} />
              <span>{entry.name}</span>
            </button>
            {entry.directory && (
              <div
                className="file-tree-children"
                data-open={open}
                aria-hidden={!open}
                inert={!open}
              >
                <div>{branch(path, depth + 1)}</div>
              </div>
            )}
          </li>
        )
      })}
    </ul>
  )
  return (
    <section className="sidebar-group folder-explorer" aria-label="Project files">
      <div className="sidebar-section-heading">
        <h2>Files</h2>
        <IconButton
          icon={RefreshCw}
          label="Refresh project files"
          disabled={loading}
          onClick={() => {
            setLoading(true)
            void refresh(() => true)
          }}
        />
      </div>
      {branch('.', 0)}
    </section>
  )
}
