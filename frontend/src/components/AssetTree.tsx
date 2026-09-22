import { Network } from 'lucide-react'
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import { useRoots } from '../api/queries'
import { useExplorer } from '../state/explorerContext'
import { AssetTreeNode, TreeSkeleton } from './AssetTreeNode'
import { TreeContext } from './treeContext'
import { EmptyState } from './ui/EmptyState'
import { ErrorState } from './ui/ErrorState'

interface Props {
  selectedId: string | undefined
  onSelect: (id: string) => void
}

/**
 * Recursive asset tree (role=tree). Roots load up front; every other level
 * loads on first expand. Keyboard follows the WAI-ARIA tree pattern with a
 * roving tabindex: Tab enters/leaves the tree once, arrows move inside it.
 */
export function AssetTree({ selectedId, onSelect }: Props) {
  const roots = useRoots()
  const { expand, collapse } = useExplorer()
  const treeRef = useRef<HTMLUListElement>(null)
  const [activeId, setActiveId] = useState<string | null>(null)
  const [tabbableId, setTabbableId] = useState<string | null>(null)

  // Exactly one rendered node is tabbable: the last focused, else the selected,
  // else the first — never one that is not on screen (collapsed or not loaded).
  // No dependency list on purpose: children load lazily and change which nodes
  // are rendered without changing any prop here. The guarded setState converges.
  // oxlint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => {
    const ids = Array.from(treeRef.current?.querySelectorAll<HTMLElement>('[role=treeitem]') ?? []).map(
      (el) => el.dataset.id as string,
    )
    const wanted = [activeId, selectedId].find((id) => id && ids.includes(id)) ?? ids[0] ?? null
    setTabbableId((cur) => (cur === wanted ? cur : wanted))
  })

  const ctx = useMemo(
    () => ({ selectedId, tabbableId, onSelect, onFocusNode: setActiveId }),
    [selectedId, tabbableId, onSelect],
  )

  function onKeyDown(e: KeyboardEvent<HTMLUListElement>) {
    const current = (e.target as HTMLElement).closest<HTMLElement>('[role=treeitem]')
    if (!current) return
    const items = Array.from(treeRef.current!.querySelectorAll<HTMLElement>('[role=treeitem]'))
    const index = items.indexOf(current)
    const id = current.dataset.id as string
    const expandable = current.dataset.expandable === 'true'
    const isOpen = current.getAttribute('aria-expanded') === 'true'
    const level = Number(current.getAttribute('aria-level'))
    const focus = (el: HTMLElement | undefined) => el?.focus()

    switch (e.key) {
      case 'ArrowDown':
        focus(items[index + 1])
        break
      case 'ArrowUp':
        focus(items[index - 1])
        break
      case 'Home':
        focus(items[0])
        break
      case 'End':
        focus(items[items.length - 1])
        break
      case 'ArrowRight':
        if (expandable && !isOpen) expand(id)
        else if (expandable && isOpen) focus(items[index + 1])
        break
      case 'ArrowLeft':
        if (expandable && isOpen) collapse(id)
        else focus(items.slice(0, index).reverse().find((el) => Number(el.getAttribute('aria-level')) === level - 1))
        break
      case 'Enter':
      case ' ':
        onSelect(id)
        if (expandable) expand(id)
        break
      default:
        return
    }
    e.preventDefault()
  }

  if (roots.isPending) return <TreeSkeleton level={1} rows={6} />
  if (roots.isError) {
    return (
      <ErrorState
        title="Could not load the asset tree"
        error={roots.error}
        onRetry={() => void roots.refetch()}
      />
    )
  }
  if (roots.data.roots.length === 0) {
    return (
      <EmptyState
        icon={Network}
        title="No assets yet"
        description="Import a CSV file to build the asset hierarchy."
        className="py-8"
      />
    )
  }

  return (
    <TreeContext.Provider value={ctx}>
      <ul
        ref={treeRef}
        role="tree"
        aria-label="Asset tree"
        onKeyDown={onKeyDown}
        className="space-y-1 py-1"
      >
        {roots.data.roots.map((node) => (
          <AssetTreeNode key={node.asset_id} node={node} level={1} />
        ))}
      </ul>
    </TreeContext.Provider>
  )
}
