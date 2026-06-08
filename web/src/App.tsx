import { useState, Suspense, lazy, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { LayoutDashboard, Route, Database, ShieldAlert, Cpu, Key, FileText, Shield, Menu, X, LogIn, LogOut } from 'lucide-react'
import { cn } from '@/lib/utils'
import { ErrorBoundary } from '@/components/ErrorBoundary'
import { getToken, setToken, clearToken } from '@/api'

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

function useAuth() {
  const [token, setTokenState] = useState<string | null>(getToken)

  const login = useCallback((t: string) => {
    setToken(t)
    setTokenState(t)
  }, [])

  const logout = useCallback(() => {
    clearToken()
    setTokenState(null)
  }, [])

  return { token, login, logout }
}

function LoginForm({ onLogin }: { onLogin: (t: string) => void }) {
  const [value, setValue] = useState('')
  const [error, setError] = useState('')

  const handleSubmit = () => {
    const t = value.trim()
    if (!t) { setError('Enter an API key'); return }
    setToken(t)
    onLogin(t)
  }

  return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="w-full max-w-sm space-y-4 rounded-lg border border-border bg-card p-6">
        <div className="flex items-center gap-2">
          <LogIn className="w-5 h-5 text-muted-foreground" />
          <h2 className="font-semibold">Login</h2>
        </div>
        <p className="text-sm text-muted-foreground">Enter your admin API key to access the dashboard.</p>
        <input
          type="password"
          value={value}
          onChange={e => { setValue(e.target.value); setError(''); }}
          onKeyDown={e => e.key === 'Enter' && handleSubmit()}
          placeholder="sk-..."
          className="w-full px-3 py-2 rounded-md border bg-background text-sm font-mono"
          autoFocus
        />
        {error && <p className="text-sm text-destructive">{error}</p>}
        <button
          onClick={handleSubmit}
          disabled={!value.trim()}
          className="w-full px-4 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium disabled:opacity-50"
        >
          Authenticate
        </button>
      </div>
    </div>
  )
}

function App() {
  const { token, login, logout } = useAuth()
  const [activeTab, setActiveTab] = useState<Tab>('dashboard')
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false)

  if (!token) {
    return (
      <div className="flex h-screen w-full bg-background text-foreground items-center justify-center">
        <LoginForm onLogin={login} />
      </div>
    )
  }

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
        <div className="p-4 border-t border-border">
          <button
            onClick={logout}
            className="w-full flex items-center gap-2 px-3 py-2 rounded-md text-sm text-muted-foreground hover:bg-muted/50 hover:text-foreground transition-colors"
          >
            <LogOut className="w-4 h-4" />
            Logout
          </button>
        </div>
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
