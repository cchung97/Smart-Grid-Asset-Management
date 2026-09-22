import { useCallback, useMemo, useReducer, useState, type ReactNode } from 'react'
import { ExplorerContext, type Explorer, type ImportFlow } from './explorerContext'
import { expandedReducer } from './expandedReducer'

// The only client state in the app: which tree nodes are expanded, where the
// two-step import is, and whether the mobile drawer is open. Everything else is
// server state owned by TanStack Query.
export function ExplorerProvider({ children }: { children: ReactNode }) {
  const [expanded, dispatch] = useReducer(expandedReducer, new Set<string>())
  const [importFlow, setImportFlow] = useState<ImportFlow>({ stage: 'idle' })
  const [railOpen, setRailOpen] = useState(false)

  // Stable identities: the tree reveals the routed asset in an effect that lists expandMany as a
  // dependency, so a new function on every change would re-open what the user just collapsed.
  const toggle = useCallback((id: string) => dispatch({ type: 'toggle', id }), [])
  const expand = useCallback((id: string) => dispatch({ type: 'expand', id }), [])
  const collapse = useCallback((id: string) => dispatch({ type: 'collapse', id }), [])
  const expandMany = useCallback((ids: string[]) => dispatch({ type: 'expandMany', ids }), [])
  const collapseAll = useCallback(() => dispatch({ type: 'collapseAll' }), [])

  const value = useMemo<Explorer>(
    () => ({ expanded, toggle, expand, collapse, expandMany, collapseAll, importFlow, setImportFlow, railOpen, setRailOpen }),
    [expanded, toggle, expand, collapse, expandMany, collapseAll, importFlow, railOpen],
  )

  return <ExplorerContext.Provider value={value}>{children}</ExplorerContext.Provider>
}
