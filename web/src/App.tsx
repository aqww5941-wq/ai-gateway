import { useState, Suspense, lazy } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { LayoutDashboard, Route, Database, ShieldAlert, Cpu, Key, FileText, Shield, Menu, X } from 'lucide-react'
import { cn } from '@/lib/utils'
import { ErrorBoundary } from '@/components/ErrorBoundary'

const DashboardPage = lazy(() => import('./pages/DashboardPage'))
const RoutesPage = lazy(() => import('./pages/RoutesPage'))
const CachePage = lazy(() => import('./pages/CachePage'))
const BreakersPage = lazy(() => import('./pages/BreakersPage'))
const ProvidersPage = lazy(() => import('./pages/ProvidersPage'))
const KeysPage = lazy(() => import('./pages/KeysPage'))
const AuditPage = lazy(() => import('./pages/AuditPage'))
const FilterPage = lazy(() => import('./pages/FilterPage'))

type Tab = 'dashboard' | 'routes' | 'cache' | 'breakers' | 'providers' | 'keys' | 'audit' | 'filter'

const tabs = [
  { key: 'dashboard', label: 'Overview', icon: LayoutDashboard },
  { key: 'routes', label: 'Routes', icon: Route },
  { key: 'cache', label: 'Cache', icon: Database },
  { key: 'breakers', label: 'Breakers', icon: ShieldAlert },
  { key: 'providers', label: 'Providers', icon: Cpu },
  { key: 'keys', label: 'Keys', icon: Key },
  { key: 'audit', label: 'Logs', icon: FileText },
  { key: 'filter', label: 'Filter', icon: Shield },
] as const

function App() {
  const [activeTab, setActiveTab] = useState<Tab>('dashboard')
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  return (
    <div className="flex h-screen w-full bg-background text-foreground overflow-hidden">
      {/* Sidebar for Desktop */}
      <aside className="hidden md:flex w-64 flex-col border-r border-border bg-card">
        <div className="h-16 flex items-center px-6 border-b border-border">
          <h1 className="font-bold text-lg tracking-tight bg-gradient-to-br from-indigo-400 to-cyan-400 bg-clip-text text-transparent">AI Gateway</h1>
        </div>
        <nav className="flex-1 space-y-1 p-4">
          {tabs.map((tab) => {
            const Icon = tab.icon
            const isActive = activeTab === tab.key
            return (
              <button
                key={tab.key}
                onClick={() => setActiveTab(tab.key)}
                className={cn(
                  "w-full flex items-center gap-3 px-3 py-2.5 rounded-md text-sm font-medium transition-all duration-200",
                  isActive
                    ? "bg-primary/10 text-primary"
                    : "text-muted-foreground hover:bg-muted/50 hover:text-foreground"
                )}
              >
                <Icon className={cn("w-5 h-5", isActive ? "text-primary" : "text-muted-foreground")} />
                {tab.label}
              </button>
            )
          })}
        </nav>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col h-screen overflow-hidden relative">
        {/* Mobile Header */}
        <header className="md:hidden h-16 flex items-center justify-between px-4 border-b border-border bg-card">
          <h1 className="font-bold text-lg tracking-tight text-white">AI Gateway</h1>
          <button onClick={() => setMobileMenuOpen(!mobileMenuOpen)} className="p-2 -mr-2 text-muted-foreground">
            {mobileMenuOpen ? <X className="w-6 h-6" /> : <Menu className="w-6 h-6" />}
          </button>
        </header>

        {/* Mobile Menu Overlay */}
        <AnimatePresence>
          {mobileMenuOpen && (
            <motion.div
              initial={{ height: 0, opacity: 0 }}
              animate={{ height: 'auto', opacity: 1 }}
              exit={{ height: 0, opacity: 0 }}
              className="md:hidden absolute top-16 left-0 right-0 bg-card border-b border-border z-50 overflow-hidden"
            >
              <nav className="p-4 space-y-1">
                {tabs.map((tab) => {
                  const Icon = tab.icon
                  const isActive = activeTab === tab.key
                  return (
                    <button
                      key={tab.key}
                      onClick={() => {
                        setActiveTab(tab.key)
                        setMobileMenuOpen(false)
                      }}
                      className={cn(
                        "w-full flex items-center gap-3 px-4 py-3 rounded-md text-base font-medium",
                        isActive ? "bg-primary/10 text-primary" : "text-muted-foreground"
                      )}
                    >
                      <Icon className="w-5 h-5" />
                      {tab.label}
                    </button>
                  )
                })}
              </nav>
            </motion.div>
          )}
        </AnimatePresence>

        <main className="flex-1 overflow-auto p-4 md:p-8">
          <Suspense fallback={
            <div className="flex h-full w-full items-center justify-center">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
            </div>
          }>
            <ErrorBoundary>
              <AnimatePresence mode="wait">
                <motion.div
                  key={activeTab}
                  initial={{ opacity: 0, y: 10 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -10 }}
                  transition={{ duration: 0.2 }}
                  className="h-full"
                >
                  {activeTab === 'dashboard' && <DashboardPage />}
                  {activeTab === 'routes' && <RoutesPage />}
                  {activeTab === 'cache' && <CachePage />}
                  {activeTab === 'breakers' && <BreakersPage />}
                  {activeTab === 'providers' && <ProvidersPage />}
                  {activeTab === 'keys' && <KeysPage />}
                  {activeTab === 'audit' && <AuditPage />}
                  {activeTab === 'filter' && <FilterPage />}
                </motion.div>
              </AnimatePresence>
            </ErrorBoundary>
          </Suspense>
        </main>
      </div>
    </div>
  )
}

export default App
