import { usePolling } from '@/hooks/usePolling'
import { api } from '@/api'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { strategyLabel } from '@/utils'
import { RefreshCw, Zap, ServerCrash, Clock, Brain, Route } from 'lucide-react'
import type { RouteInfo } from '@/types'

function getStrategyBadge(strategy: string) {
  switch (strategy) {
    case 'round_robin': return <Badge variant="secondary" className="bg-blue-500/15 text-blue-500 hover:bg-blue-500/25"><RefreshCw className="mr-1 h-3 w-3" />{strategyLabel(strategy)}</Badge>
    case 'weighted': return <Badge variant="secondary" className="bg-emerald-500/15 text-emerald-500 hover:bg-emerald-500/25"><Zap className="mr-1 h-3 w-3" />{strategyLabel(strategy)}</Badge>
    case 'fallback': return <Badge variant="secondary" className="bg-amber-500/15 text-amber-500 hover:bg-amber-500/25"><ServerCrash className="mr-1 h-3 w-3" />{strategyLabel(strategy)}</Badge>
    case 'latency': return <Badge variant="secondary" className="bg-indigo-500/15 text-indigo-400 hover:bg-indigo-500/25"><Clock className="mr-1 h-3 w-3" />{strategyLabel(strategy)}</Badge>
    case 'semantic': return <Badge variant="secondary" className="bg-purple-500/15 text-purple-400 hover:bg-purple-500/25"><Brain className="mr-1 h-3 w-3" />{strategyLabel(strategy)}</Badge>
    default: return <Badge variant="outline">{strategyLabel(strategy)}</Badge>
  }
}

export default function RoutesPage() {
  const { data, loading, error } = usePolling<{ routes: RouteInfo[] }>(api.getRoutes, 10000)

  if (loading) return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
    </div>
  )
  if (error) return <div className="text-destructive font-semibold p-4 bg-destructive/10 rounded-lg border border-destructive/20">{error}</div>

  const routes = data?.routes ?? []

  return (
    <div className="space-y-6 pb-8">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Active Routes ({routes.length})</h2>
        <p className="text-muted-foreground mt-1">Configured model dispatch and load balancing rules.</p>
      </div>

      {routes.length === 0 ? (
        <Card className="flex flex-col items-center justify-center h-48 border-dashed">
          <Route className="h-8 w-8 text-muted-foreground mb-4" />
          <p className="text-muted-foreground text-sm">No routing configurations found.</p>
        </Card>
      ) : (
        <Card className="overflow-hidden border-border/50">
          <Table>
            <TableHeader className="bg-muted/50">
              <TableRow>
                <TableHead className="w-[150px]">Route Name</TableHead>
                <TableHead>Match Pattern</TableHead>
                <TableHead>Strategy</TableHead>
                <TableHead>Targets</TableHead>
                <TableHead>Semantic Rules</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {routes.map((r) => (
                <TableRow key={r.name} className="group">
                  <TableCell className="font-medium">{r.name}</TableCell>
                  <TableCell>
                    <code className="bg-muted px-2 py-1 rounded text-xs text-primary/90 font-mono">{r.match_model}</code>
                  </TableCell>
                  <TableCell>
                    {getStrategyBadge(r.strategy)}
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-col gap-1.5 py-1">
                      {r.targets.length > 0
                        ? r.targets.map((t) => (
                            <div key={t.provider + t.model} className="flex items-center gap-1.5 text-sm group-hover:text-foreground text-muted-foreground transition-colors">
                              <span className="font-medium text-foreground">{t.provider}</span>
                              <span className="text-muted-foreground/50">/</span>
                              <code className="font-mono text-xs">{t.model}</code>
                              {t.weight > 0 && <Badge variant="outline" className="h-5 text-[10px] ml-1 bg-background">w:{t.weight}</Badge>}
                            </div>
                          ))
                        : <span className="text-muted-foreground">-</span>}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div className="flex flex-col gap-1.5 py-1">
                      {r.semantic_rules && r.semantic_rules.length > 0
                        ? r.semantic_rules.map((sr, idx) => (
                            <div key={idx} className="flex flex-wrap items-center gap-1.5 text-sm group-hover:text-foreground text-muted-foreground transition-colors">
                              <Badge variant={sr.complexity === 'simple' ? 'success' : 'warning'} className="h-5 text-[10px] px-1.5">
                                {sr.complexity}
                              </Badge>
                              <span className="text-muted-foreground/50 text-xs">→</span>
                              <span className="font-medium text-foreground">{sr.provider}</span>
                              <span className="text-muted-foreground/50">/</span>
                              <code className="font-mono text-xs">{sr.model}</code>
                            </div>
                          ))
                        : <span className="text-muted-foreground">-</span>}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}
    </div>
  )
}
