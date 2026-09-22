import * as Dialog from '@radix-ui/react-dialog'
import { Layers, Menu, Upload, X } from 'lucide-react'
import { useEffect, type ReactNode } from 'react'
import { Link, NavLink } from 'react-router-dom'
import { AppFooter, AppVersion } from './AppFooter'
import { ThemeToggle } from './ThemeToggle'

interface Props {
  search: ReactNode
  /** The asset tree. Omit it for full-width pages: no aside and no mobile drawer. */
  rail?: ReactNode
  children: ReactNode
  /** Mobile: the rail is an off-canvas drawer. */
  railOpen?: boolean
  onRailOpenChange?: (open: boolean) => void
}

const NAV = [
  { to: '/', label: 'Overview', end: true },
  { to: '/assets', label: 'All assets', end: true },
]

function MainNav({ orientation, onNavigate }: { orientation: 'row' | 'column'; onNavigate?: () => void }) {
  return (
    <nav aria-label="Main" className={orientation === 'row' ? 'flex items-center gap-1' : 'flex flex-col gap-1 p-3'}>
      {NAV.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end={item.end}
          onClick={onNavigate}
          className={({ isActive }) =>
            `rounded-md px-3 py-2 text-sm font-medium no-underline ${
              isActive ? 'bg-primary-soft text-primary' : 'text-text hover:bg-neutral-soft'
            }`
          }
        >
          {item.label}
        </NavLink>
      ))}
    </nav>
  )
}

/**
 * Layout only: top bar, left rail and main panel. At md and above the rail is a
 * fixed column; below it, the rail opens as a drawer from the "Assets" button.
 */
export function AppShell({ search, rail, children, railOpen = false, onRailOpenChange }: Props) {
  // Resizing up to the desktop layout while the drawer is open must not strand an invisible modal.
  useEffect(() => {
    const mq = window.matchMedia?.('(min-width: 768px)')
    if (!mq || !onRailOpenChange) return
    const onChange = () => mq.matches && onRailOpenChange(false)
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [onRailOpenChange])

  return (
    <div className="flex h-full flex-col bg-surface text-text">
      <a
        href="#main"
        className="sr-only focus:not-sr-only focus:absolute focus:z-50 focus:m-2 focus:rounded-md focus:bg-primary focus:px-3 focus:py-2 focus:text-on-primary"
      >
        Skip to main content
      </a>
      <header className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-2 border-b border-border px-4 py-3 sm:px-6 lg:px-8">
        {rail && (
          <Dialog.Root open={railOpen} onOpenChange={(open) => onRailOpenChange?.(open)}>
            <Dialog.Trigger className="inline-flex items-center gap-2 rounded-md border border-border-strong bg-surface px-3 py-2 text-sm font-medium text-text hover:bg-surface-muted md:hidden">
              <Menu aria-hidden className="size-4" />
              Assets
            </Dialog.Trigger>
            <Dialog.Portal>
              <Dialog.Overlay className="fixed inset-0 z-40 bg-overlay md:hidden" />
              <Dialog.Content className="fixed inset-y-0 left-0 z-50 flex w-[min(24rem,90vw)] flex-col bg-surface-muted shadow-xl md:hidden">
                <Dialog.Title className="sr-only">Assets</Dialog.Title>
                <Dialog.Description className="sr-only">Browse the asset tree.</Dialog.Description>
                <div className="flex items-center justify-between border-b border-border px-4 py-3">
                  <span className="font-semibold">Assets</span>
                  <Dialog.Close aria-label="Close" className="rounded-md p-2 text-text-muted hover:bg-neutral-soft">
                    <X aria-hidden className="size-5" />
                  </Dialog.Close>
                </div>
                <MainNav orientation="column" onNavigate={() => onRailOpenChange?.(false)} />
                <div className="flex min-h-0 flex-1 flex-col border-t border-border">{rail}</div>
                <div className="shrink-0 px-4 py-3">
                  <AppVersion />
                </div>
              </Dialog.Content>
            </Dialog.Portal>
          </Dialog.Root>
        )}

        <Link to="/" className="flex items-center gap-3 rounded-md text-text no-underline">
          <Layers aria-hidden className="size-6 text-primary" />
          <span className="text-base font-semibold">Grid Asset Explorer</span>
        </Link>
        {/* With a rail the nav lives in the mobile drawer; without one it stays in the header at every width. */}
        <div className={rail ? 'hidden md:block' : ''}>
          <MainNav orientation="row" />
        </div>
        <div className="order-last w-full md:order-none md:ml-auto md:w-80 lg:w-96">{search}</div>
        <NavLink
          to="/import"
          className={({ isActive }) =>
            `ml-auto inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-semibold no-underline md:ml-0 ${
              isActive ? 'bg-primary-hover text-on-primary' : 'bg-primary text-on-primary hover:bg-primary-hover'
            }`
          }
        >
          <Upload aria-hidden className="size-4" />
          Import
        </NavLink>
        <ThemeToggle />
      </header>

      <div
        className={`grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)] grid-rows-[minmax(0,1fr)] ${
          rail ? 'md:grid-cols-[22rem_minmax(0,1fr)] lg:grid-cols-[26rem_minmax(0,1fr)] xl:grid-cols-[28rem_minmax(0,1fr)]' : ''
        }`}
      >
        {rail && (
          <aside aria-label="Asset tree" className="relative hidden min-h-0 flex-col overflow-hidden border-r border-border bg-surface-muted md:flex">
            <div className="flex min-h-0 flex-1 flex-col">{rail}</div>
            <div className="shrink-0 px-4 py-3">
              <AppVersion />
            </div>
          </aside>
        )}
        {/* relative: visually-hidden (sr-only) elements are absolutely positioned and would otherwise escape
            this panel's scrolling and stretch the whole page, in height and width. */}
        <main id="main" tabIndex={-1} className="relative min-h-0 overflow-y-auto">
          <div className="flex min-h-full flex-col">
            <div className="mx-auto w-full max-w-[110rem] flex-1 px-4 py-6 sm:px-6 sm:py-8 lg:px-8">{children}</div>
            {/* The footer is the end of the scrolling content, not a fixed bar. */}
            <AppFooter withVersion={!rail} />
          </div>
        </main>
      </div>
    </div>
  )
}
