import { Navigate, Route, Routes, useLocation } from 'react-router-dom'
import { AllAssetsPanel } from './components/AllAssetsPanel'
import { AppRoot } from './components/AppRoot'
import { AssetDetailsPanel } from './components/AssetDetailsPanel'
import { ExplorerLayout } from './components/ExplorerLayout'
import { HomePanel } from './components/HomePanel'
import { ImportPage } from './components/ImportPage'
import { NotFoundPanel } from './components/NotFoundPanel'
import { PageLayout } from './components/PageLayout'

// `/imports` (the old activity page) now lives on the Import page; keep old
// links and bookmarks working, including the history's ?page=.
function ImportsRedirect() {
  const { search } = useLocation()
  return <Navigate to={{ pathname: '/import', search }} replace />
}

// Two layouts share one shell and one client-state provider:
//  - ExplorerLayout: header + asset tree rail + <Outlet/>; selecting an asset never leaves the tree.
//  - PageLayout: header + full-width <Outlet/>, no tree (the Import page).
export default function App() {
  return (
    <Routes>
      <Route element={<AppRoot />}>
        <Route element={<ExplorerLayout />}>
          <Route index element={<HomePanel />} />
          <Route path="assets" element={<AllAssetsPanel />} />
          <Route path="assets/:assetId" element={<AssetDetailsPanel />} />
          <Route path="*" element={<NotFoundPanel />} />
        </Route>
        <Route element={<PageLayout />}>
          <Route path="import" element={<ImportPage />} />
          <Route path="imports" element={<ImportsRedirect />} />
        </Route>
      </Route>
    </Routes>
  )
}
