import { useState } from 'react'
import TabBar from './components/TabBar'
import DashboardPage from './pages/DashboardPage'
import RoutesPage from './pages/RoutesPage'
import CachePage from './pages/CachePage'
import BreakersPage from './pages/BreakersPage'
import ProvidersPage from './pages/ProvidersPage'

type Tab = 'dashboard' | 'routes' | 'cache' | 'breakers' | 'providers'

const tabs: { key: Tab; label: string }[] = [
  { key: 'dashboard', label: '概览' },
  { key: 'routes', label: '路由' },
  { key: 'cache', label: '缓存' },
  { key: 'breakers', label: '熔断器' },
  { key: 'providers', label: '提供商' },
]

export default function App() {
  const [tab, setTab] = useState<Tab>('dashboard')

  return (
    <div className="app">
      <header className="app-header">
        <h1>AI 网关管理后台</h1>
        <TabBar active={tab} onChange={(k) => setTab(k as Tab)} tabs={tabs} />
      </header>
      <main className="app-main">
        {tab === 'dashboard' && <DashboardPage />}
        {tab === 'routes' && <RoutesPage />}
        {tab === 'cache' && <CachePage />}
        {tab === 'breakers' && <BreakersPage />}
        {tab === 'providers' && <ProvidersPage />}
      </main>
    </div>
  )
}
