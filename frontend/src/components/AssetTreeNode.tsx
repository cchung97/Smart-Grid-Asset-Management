import { ChevronRight } from 'lucide-react'
import { useEffect, useId, useRef } from 'react'
import { assetTypeMeta } from '../api/assetTypes'
import { useChildren } from '../api/queries'
import type { AssetNode } from '../api/types'
import { useExplorer } from '../state/explorerContext'
import { useTree } from './treeContext'
import { StatusDot } from './ui/Badges'
import { ErrorState } from './ui/ErrorState'
import { Skeleton, SkeletonRegion } from './ui/Skeleton'

const indent = (level: number) => ({ paddingLeft: `${0.5 + (level - 1) * 1.5}rem` })

export function TreeSkeleton({ level, rows = 3 }: { level: number; rows?: number }) {
  return (
    <SkeletonRegion label="assets" className="space-y-2 py-2" >
      {Array.from({ length: rows }, (_, i) => (
        <div key={i} style={indent(level)} className="pr-3">
          <Skeleton className="h-6" />
        </div>
      ))}
    </SkeletonRegion>
  )
}

interface Props {
  node: AssetNode
  level: number
}

/** One row of the tree. Children are fetched lazily, the first time it expands. */
export function AssetTreeNode({ node, level }: Props) {
  const { expanded, toggle, expand } = useExplorer()
  const { selectedId, tabbableId, onSelect, onFocusNode } = useTree()

  const id = node.asset_id
  const expandable = node.child_count > 0
  const isExpanded = expandable && expanded.has(id)
  const isSelected = selectedId === id
  const meta = assetTypeMeta(node.asset_type)
  const Icon = meta.icon
  const isRoot = node.parent_asset_id === null
  const descendants = node.subtree_count - 1

  const children = useChildren(id, isExpanded)
  const rowRef = useRef<HTMLDivElement>(null)

  // Bring the selected node into view. It runs on mount too, so a node that was
  // only rendered because its ancestors just finished loading still scrolls.
  useEffect(() => {
    if (isSelected) rowRef.current?.scrollIntoView({ block: 'nearest' })
  }, [isSelected])

  // useId, not the asset id: ids may contain characters that are invalid in id refs.
  const uid = useId()
  const groupId = `${uid}-group`
  const labelId = `${uid}-label`
  const childNodes = children.data?.groups.flatMap((g) => g.assets) ?? []

  return (
    <li role="none">
      <div
        ref={rowRef}
        role="treeitem"
        data-id={id}
        data-expandable={expandable}
        aria-level={level}
        aria-expanded={expandable ? isExpanded : undefined}
        aria-selected={isSelected}
        aria-owns={isExpanded ? groupId : undefined}
        // Own label only: with aria-owns the name-from-content would otherwise
        // include every descendant and a screen reader would read the subtree.
        aria-labelledby={labelId}
        tabIndex={tabbableId === id ? 0 : -1}
        onFocus={() => onFocusNode(id)}
        onClick={() => {
          onSelect(id)
          if (expandable) expand(id)
        }}
        style={indent(level)}
        className={`flex cursor-pointer items-center gap-3 rounded-md border-l-[3px] py-2 pr-3 text-sm ${
          isSelected
            ? 'border-primary bg-primary-soft font-semibold text-primary'
            : 'border-transparent text-text hover:bg-neutral-soft'
        }`}
      >
        {expandable ? (
          <span
            aria-hidden
            onClick={(e) => {
              e.stopPropagation()
              toggle(id)
            }}
            className="grid size-5 shrink-0 place-items-center rounded-sm hover:bg-border"
          >
            <ChevronRight className={`size-4 transition-transform ${isExpanded ? 'rotate-90' : ''}`} />
          </span>
        ) : (
          <span aria-hidden className="size-5 shrink-0" />
        )}
        <Icon aria-hidden className="size-4 shrink-0" />
        <span id={labelId} className="flex min-w-0 flex-1 items-center gap-2">
          <span className="sr-only">{meta.label}: </span>
          <span className="min-w-0 flex-1 truncate" title={node.asset_name}>
            {node.asset_name}
          </span>
          {isRoot && descendants > 0 && (
            <span
              title={`${descendants} descendant assets`}
              className="rounded-full bg-neutral-soft px-2 text-xs font-semibold text-text-muted tabular-nums"
            >
              {descendants}
              <span className="sr-only"> descendant assets</span>
            </span>
          )}
          <StatusDot status={node.operational_status} />
        </span>
      </div>

      {isExpanded && (
        <ul role="group" id={groupId}>
          {children.isPending && (
            <li role="none">
              <TreeSkeleton level={level + 1} rows={Math.min(node.child_count, 3)} />
            </li>
          )}
          {children.isError && (
            <li role="none">
              <ErrorState
                title="Could not load children"
                error={children.error}
                onRetry={() => void children.refetch()}
                className="py-4"
              />
            </li>
          )}
          {childNodes.map((child) => (
            <AssetTreeNode key={child.asset_id} node={child} level={level + 1} />
          ))}
        </ul>
      )}
    </li>
  )
}
