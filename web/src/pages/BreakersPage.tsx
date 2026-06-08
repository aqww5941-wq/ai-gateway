import { usePolling } from '@/hooks/usePolling'
import { api } from '@/api'
import { Card } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { ShieldAlert, Shield, ShieldHalf, ActivitySquare } from 'lucide-react'
import type { BreakerSnapshot } from '@/types'

function getStatusBadge(status: string | undefined | null) {
  switch (status?.toLowerCase()) {
    case 'closed':
      return <Badge variant="success" className="h-6"><Shield className="w-3.5 h-3.5 mr-1" /> Healthy (Closed)</Badge>
    case 'open':
      return <Badge variant="destructive" className="h-6"><ShieldAlert className="w-3.5 h-3.5 mr-1" /> Broken (Open)</Badge>
    case 'half_open':
    case 'half-open':
      return <Badge variant="warning" className="h-6 whitespace-nowrap"><ShieldHalf className="w-3.5 h-3.5 mr-1" /> Probing (Half-Open)</Badge>
    default:
      return <Badge variant="outline">{status}</Badge>
  }
}

export default function BreakersPage() {
  const { data, loading, error } = usePolling<{ breakers: BreakerSnapshot[] }>(api.getBreakers, 3000)

  if (loading) return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
    </div>
  )
  if (error) return <div className="text-destructive font-semibold p-4 bg-destructive/10 rounded-lg border border-destructive/20">{error}</div>

  const breakers = data?.breakers ?? []

  return (
    <div className="space-y-6 pb-8">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Circuit Breakers ({breakers.length})</h2>
        <p className="text-muted-foreground mt-1">Monitor the resilience state of backend AI providers.</p>
      </div>

      {breakers.length === 0 ? (
        <Card className="flex flex-col items-center justify-center h-48 border-dashed">
          <ActivitySquare className="h-8 w-8 text-muted-foreground mb-4 opacity-50" />
          <p className="text-muted-foreground text-sm font-medium">No breakers recorded yet</p>
          <p className="text-xs text-muted-foreground/70 mt-1">Make requests to providers to populate state.</p>
        </Card>
      ) : (
        <Card className="overflow-hidden border-border/50">
          <Table>
            <TableHeader className="bg-muted/50">
              <TableRow>
                <TableHead className="w-[200px]">Target Provider</TableHead>
                <TableHead>Breaker State</TableHead>
                <TableHead className="text-right">Recent Failures</TableHead>
                <TableHead className="text-right">Successful Probes</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {breakers.map((b) => (
                <TableRow key={b.name} className="group">
                  <TableCell className="font-medium">{b.name}</TableCell>
                  <TableCell>
                    {getStatusBadge(b.state)}
                  </TableCell>
                  <TableCell className="text-right">
                    <span className={b.failures > 0 ? "text-amber-500 font-bold" : "text-muted-foreground"}>
                      {b.failures}
                    </span>
                  </TableCell>
                  <TableCell className="text-right">
                    <span className={b.probes_ok > 0 ? "text-emerald-500 font-bold" : "text-muted-foreground"}>
                      {b.probes_ok}
                    </span>
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
