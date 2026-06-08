import { usePolling } from '@/hooks/usePolling'
import { api } from '@/api'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { providerTypeLabel } from '@/utils'
import { Cpu, CheckCircle2, XCircle, Timer, ShieldHalf } from 'lucide-react'
import type { ProviderInfo } from '@/types'

function getHealthBadge(status: string) {
  if (status.toLowerCase() === 'healthy' || status === 'up') {
    return <Badge variant="success" className="h-5 text-[10px] uppercase font-bold"><CheckCircle2 className="w-3 h-3 mr-1" /> Healthy</Badge>
  }
  return <Badge variant="destructive" className="h-5 text-[10px] uppercase font-bold"><XCircle className="w-3 h-3 mr-1" /> Down</Badge>
}

function getBreakerBadge(state: string) {
  switch (state.toLowerCase()) {
    case 'closed':
      return <Badge variant="outline" className="h-5 text-[10px] bg-background">Closed</Badge>
    case 'open':
      return <Badge variant="destructive" className="h-5 text-[10px]">Open</Badge>
    case 'half_open':
    case 'half-open':
      return <Badge variant="warning" className="h-5 text-[10px]"><ShieldHalf className="w-3 h-3 mr-1" />Half-Open</Badge>
    default:
      return <Badge variant="outline" className="h-5 text-[10px] bg-background">{state}</Badge>
  }
}

export default function ProvidersPage() {
  const { data, loading, error } = usePolling<{ providers: ProviderInfo[] }>(api.getProviders, 10000)

  if (loading) return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
    </div>
  )
  if (error) return <div className="text-destructive font-semibold p-4 bg-destructive/10 rounded-lg border border-destructive/20">{error}</div>

  const providers = data?.providers ?? []

  return (
    <div className="space-y-6 pb-8">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Providers ({providers.length})</h2>
        <p className="text-muted-foreground mt-1">Configured AI endpoints and their supported models.</p>
      </div>

      {providers.length === 0 ? (
        <Card className="flex flex-col items-center justify-center h-48 border-dashed">
          <Cpu className="h-8 w-8 text-muted-foreground mb-4 opacity-50" />
          <p className="text-muted-foreground text-sm font-medium">No providers configured</p>
        </Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
          {providers.map((p) => (
            <Card key={p.name} className="flex flex-col hover:border-primary/50 transition-colors">
              <CardHeader className="pb-3 border-b border-border/50">
                <div className="flex items-start justify-between">
                  <div>
                    <CardTitle className="text-lg flex items-center gap-2">
                      {p.name}
                    </CardTitle>
                    <CardDescription className="mt-1">{providerTypeLabel(p.type)}</CardDescription>
                  </div>
                  {getHealthBadge(p.health)}
                </div>
              </CardHeader>
              <CardContent className="pt-4 flex-1 flex flex-col justify-between">
                <div className="space-y-3 mb-6">
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground flex items-center gap-1.5"><Timer className="h-3.5 w-3.5" /> Timeout</span>
                    <code className="text-xs font-mono">{p.timeout}</code>
                  </div>
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground">Breaker State</span>
                    {getBreakerBadge(p.breaker_state)}
                  </div>
                </div>
                
                <div>
                  <div className="text-xs font-semibold text-muted-foreground uppercase tracking-wider mb-2">Supported Models</div>
                  <div className="flex flex-wrap gap-1.5">
                    {p.models.map((m) => (
                      <Badge key={m} variant="secondary" className="bg-primary/10 text-primary hover:bg-primary/20 transition-colors rounded">
                        {m}
                      </Badge>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
