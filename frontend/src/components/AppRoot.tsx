import { Outlet } from 'react-router-dom'
import { ExplorerProvider } from '../state/explorer'

// Client state (expanded tree nodes, the import flow and its chosen File, the
// mobile drawer) sits above both layouts so it survives moving between the
// explorer and the Import page.
export function AppRoot() {
  return (
    <ExplorerProvider>
      <Outlet />
    </ExplorerProvider>
  )
}
