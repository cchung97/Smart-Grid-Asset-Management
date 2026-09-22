export type ExpandAction =
  | { type: 'toggle'; id: string }
  | { type: 'expand'; id: string }
  | { type: 'collapse'; id: string }
  | { type: 'expandMany'; ids: string[] }
  | { type: 'collapseAll' }

export function expandedReducer(state: ReadonlySet<string>, action: ExpandAction): Set<string> {
  const next = new Set(state)
  switch (action.type) {
    case 'toggle':
      if (!next.delete(action.id)) next.add(action.id)
      return next
    case 'expand':
      next.add(action.id)
      return next
    case 'collapse':
      next.delete(action.id)
      return next
    case 'collapseAll':
      return state.size === 0 ? (state as Set<string>) : new Set()
    case 'expandMany':
      // Return the same set when nothing changes so effects do not re-run.
      if (action.ids.every((id) => state.has(id))) return state as Set<string>
      action.ids.forEach((id) => next.add(id))
      return next
  }
}
