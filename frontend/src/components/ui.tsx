import type { ComponentProps, ReactNode } from 'react'
import { useRef } from 'react'
import type { LucideIcon } from 'lucide-react'
import { MoreHorizontal, X } from 'lucide-react'
import * as DialogPrimitive from '@radix-ui/react-dialog'
import * as Dropdown from '@radix-ui/react-dropdown-menu'
import * as Tooltip from '@radix-ui/react-tooltip'

export function Icon({ icon: Glyph, size = 18 }: { icon: LucideIcon; size?: number }) {
  return <Glyph size={size} strokeWidth={1.65} aria-hidden="true" className="icon" />
}
export function Brand({ compact = false }: { compact?: boolean }) {
  return (
    <div className="brand">
      <span className="brand-mark">
        <svg
          width={compact ? 26 : 34}
          height={compact ? 26 : 34}
          viewBox="0 0 32 32"
          fill="none"
          aria-hidden="true"
          className="kiko-symbol"
        >
          <path
            d="M8 4.5h11l6 6V25a2.5 2.5 0 0 1-2.5 2.5h-15A2.5 2.5 0 0 1 5 25V7a2.5 2.5 0 0 1 3-2.5Z"
            fill="currentColor"
            opacity=".14"
          />
          <path
            d="M19 4.5v6h6M8 4.5h11l6 6V25a2.5 2.5 0 0 1-2.5 2.5h-15A2.5 2.5 0 0 1 5 25V7a2.5 2.5 0 0 1 3-2.5Z"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinejoin="round"
          />
          <path
            d="M11 12v10m9-10-5 5 5 5m-9-5h4"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </span>
      {!compact && <span>Kiko</span>}
    </div>
  )
}
export function Button({
  variant = 'primary',
  className = '',
  ...props
}: ComponentProps<'button'> & { variant?: 'primary' | 'secondary' | 'ghost' | 'danger' }) {
  return <button type="button" className={`button ${variant} ${className}`} {...props} />
}
export function IconButton({
  icon,
  label,
  ...props
}: ComponentProps<'button'> & { icon: LucideIcon; label: string }) {
  return (
    <Tooltip.Root>
      <Tooltip.Trigger asChild>
        <Button variant="ghost" className="icon-button" aria-label={label} {...props}>
          <Icon icon={icon} />
        </Button>
      </Tooltip.Trigger>
      <Tooltip.Portal>
        <Tooltip.Content className="tooltip" sideOffset={6}>
          {label}
        </Tooltip.Content>
      </Tooltip.Portal>
    </Tooltip.Root>
  )
}
export function Field({ label, ...props }: ComponentProps<'input'> & { label: string }) {
  return (
    <label className="field">
      <span>{label}</span>
      <input {...props} />
    </label>
  )
}
export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  children,
  className = '',
}: {
  open: boolean
  onOpenChange(open: boolean): void
  title: string
  description: string
  children: ReactNode
  className?: string
}) {
  const restore = useRef<HTMLElement | null>(null)
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="overlay" />
        <DialogPrimitive.Content
          className={`dialog ${className}`}
          onOpenAutoFocus={() => {
            restore.current = document.activeElement as HTMLElement
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault()
            restore.current?.focus()
          }}
        >
          <DialogPrimitive.Title>{title}</DialogPrimitive.Title>
          <DialogPrimitive.Description>{description}</DialogPrimitive.Description>
          <DialogPrimitive.Close asChild>
            <Button variant="ghost" className="dialog-close icon-button" aria-label="Close dialog">
              <Icon icon={X} />
            </Button>
          </DialogPrimitive.Close>
          {children}
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  )
}
export function ItemMenu({
  label,
  onRename,
  onDelete,
}: {
  label: string
  onRename(): void
  onDelete(): void
}) {
  return (
    <Dropdown.Root>
      <Dropdown.Trigger asChild>
        <IconButton icon={MoreHorizontal} label={`Actions for ${label}`} />
      </Dropdown.Trigger>
      <Dropdown.Portal>
        <Dropdown.Content className="menu" sideOffset={6}>
          <Dropdown.Item onSelect={onRename}>Rename</Dropdown.Item>
          <Dropdown.Item className="danger-text" onSelect={onDelete}>
            Delete
          </Dropdown.Item>
        </Dropdown.Content>
      </Dropdown.Portal>
    </Dropdown.Root>
  )
}
export function NavRow({
  active,
  icon,
  children,
  onClick,
}: {
  active: boolean
  icon?: LucideIcon
  children: ReactNode
  onClick(): void
}) {
  return (
    <button
      className={`nav-row ${active ? 'selected' : ''}`}
      aria-current={active ? 'page' : undefined}
      onClick={onClick}
    >
      {icon && <Icon icon={icon} />}
      <span>{children}</span>
    </button>
  )
}
export function PaneHeader({ children }: { children: ReactNode }) {
  return <header className="pane-header">{children}</header>
}
export function UIProvider({ children }: { children: ReactNode }) {
  return <Tooltip.Provider delayDuration={350}>{children}</Tooltip.Provider>
}
