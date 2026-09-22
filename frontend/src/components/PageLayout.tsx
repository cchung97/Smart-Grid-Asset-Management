import { Outlet } from 'react-router-dom'
import { AppShell } from './AppShell'
import { SearchBar } from './SearchBar'

// Layout route for full-width pages (Import): the same header, no asset tree.
export function PageLayout() {
  return (
    <AppShell search={<SearchBar />}>
      <Outlet />
    </AppShell>
  )
}
