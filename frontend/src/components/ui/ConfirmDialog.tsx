import * as Dialog from '@radix-ui/react-dialog'
import { LoaderCircle } from 'lucide-react'
import type { FormEvent, ReactNode } from 'react'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description: ReactNode
  confirmLabel: string
  onConfirm: () => void
  pending?: boolean
  error?: string | null
  /** Keeps the confirm button disabled (e.g. until "DELETE" has been typed). */
  confirmDisabled?: boolean
  children?: ReactNode
}

/** A modal that must be answered: focus is trapped, Escape and Cancel close it. Used for destructive actions. */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  onConfirm,
  pending = false,
  error,
  confirmDisabled = false,
  children,
}: Props) {
  function submit(e: FormEvent) {
    e.preventDefault()
    if (!pending && !confirmDisabled) onConfirm()
  }

  return (
    <Dialog.Root open={open} onOpenChange={(next) => !pending && onOpenChange(next)}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-50 bg-overlay" />
        <Dialog.Content
          role="alertdialog"
          className="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2rem)] max-w-md -translate-x-1/2 -translate-y-1/2 rounded-lg bg-surface p-6 shadow-xl"
        >
          <form onSubmit={submit}>
            <Dialog.Title className="text-lg font-semibold text-text">{title}</Dialog.Title>
            <Dialog.Description asChild>
              <div className="mt-2 text-sm text-text-muted">{description}</div>
            </Dialog.Description>
            {children && <div className="mt-4">{children}</div>}
            {error && (
              <p role="alert" className="mt-4 rounded-md bg-danger-soft px-3 py-2 text-sm text-danger">
                {error}
              </p>
            )}
            <div className="mt-6 flex justify-end gap-3">
              <Dialog.Close
                type="button"
                disabled={pending}
                className="rounded-md border border-border-strong bg-surface px-4 py-2 text-sm font-medium text-text hover:bg-surface-muted disabled:opacity-60"
              >
                Cancel
              </Dialog.Close>
              <button
                type="submit"
                disabled={pending || confirmDisabled}
                className="inline-flex items-center gap-2 rounded-md bg-danger px-4 py-2 text-sm font-semibold text-on-primary hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-50"
              >
                {pending && <LoaderCircle aria-hidden className="size-4 animate-spin" />}
                {confirmLabel}
              </button>
            </div>
          </form>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  )
}
