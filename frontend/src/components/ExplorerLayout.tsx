import { useEffect } from 'react'
import { ChevronsDownUp } from 'lucide-react'
import { Outlet, useLocation, useMatch, useNavigate } from 'react-router-dom'
import { useAncestors } from '../api/queries'
import { useExplorer } from '../state/explorerContext'
import { AppShell } from './AppShell'
import { AssetTree } from './AssetTree'
import { SearchBar } from './SearchBar'

const iconBtn =
  'rounded-md p-2 text-text-muted hover:bg-neutral-soft hover:text-text disabled:cursor-not-allowed disabled:opacity-50'

function Rail() {
  const navigate = useNavigate()
  const match = useMatch('/assets/:assetId')
  const selectedId = match?.params.assetId
  const { expandMany, collapseAll } = useExplorer()

  // Deep links, refresh and back/forward all land here: reveal the selected
  // asset by expanding its ancestors. The search flow does the same up front.
  const ancestors = useAncestors(selectedId)
  useEffect(() => {
    if (ancestors.data) expandMany(ancestors.data.path.slice(0, -1).map((a) => a.asset_id))
  }, [ancestors.data, expandMany])

  return (
    <>
      <div className="flex items-center justify-between gap-2 px-4 pt-4 pb-2">
        <h2 className="text-xs font-semibold tracking-wider text-text-muted uppercase">Asset tree</h2>
        <button type="button" onClick={collapseAll} aria-label="Collapse all" title="Collapse all" className={iconBtn}>
          <ChevronsDownUp aria-hidden className="size-4" />
        </button>
      </div>
      {/* relative: every tree row has an sr-only status label (absolutely positioned); without a positioned
          scroll container they escape it and stretch the page below the tree. */}
      <div className="relative min-h-0 flex-1 overflow-y-auto px-2 pb-4">
        <AssetTree selectedId={selectedId} onSelect={(id) => void navigate(`/assets/${encodeURIComponent(id)}`)} />
      </div>
    </>
  )
}

function Shell() {
  const { railOpen, setRailOpen } = useExplorer()
  const { pathname } = useLocation()

  // Any navigation (selecting an asset, following a link) closes the mobile drawer.
  useEffect(() => setRailOpen(false), [pathname, setRailOpen])

  return (
    <AppShell search={<SearchBar />} rail={<Rail />} railOpen={railOpen} onRailOpenChange={setRailOpen}>
      <Outlet />
    </AppShell>
  )
}

// Layout route for the explorer: the same shell and tree render for every route
// under it; only the <Outlet/> (main panel) and the tree's expansion/selection differ.
export function ExplorerLayout() {
  return <Shell />
}
