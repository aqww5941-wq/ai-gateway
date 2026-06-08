import { useState, useCallback } from 'react'
import { usePolling } from '@/hooks/usePolling'
import { api } from '@/api'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { formatPct, formatDate } from '@/utils'
import { Database, Target, XCircle, Search, ExternalLink, X } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import type { CacheInfo, CacheEntryDetail } from '@/types'

export default function CachePage() {
  const { data, loading } = usePolling<CacheInfo>(api.getCache, 5000)
  const [detail, setDetail] = useState<CacheEntryDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [detailError, setDetailError] = useState<string | null>(null)

  const viewEntry = useCallback(async (key: string) => {
    setDetailLoading(true)
    setDetailError(null)
    try {
      const d = await api.getCacheEntry(key)
      setDetail(d)
    } catch (e: unknown) {
      setDetailError((e as Error).message)
    } finally {
      setDetailLoading(false)
    }
  }, [])

  const closeDetail = useCallback(() => {
    setDetail(null)
    setDetailError(null)
  }, [])

  if (loading) return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
    </div>
  )

  return (
    <div className="space-y-6 pb-8">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Cache Storage</h2>
        <p className="text-muted-foreground mt-1">Manage semantic caching entries and monitor hit rates.</p>
      </div>

      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
        <Card className="hover:border-primary/50 transition-colors">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Hit Rate</CardTitle>
            <Target className="h-4 w-4 text-emerald-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold text-emerald-400">{formatPct(data?.hit_rate_pct ?? 0)}</div>
          </CardContent>
        </Card>
        
        <Card className="hover:border-primary/50 transition-colors">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Current Size</CardTitle>
            <Database className="h-4 w-4 text-primary" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{data?.current_size ?? 0}</div>
            <p className="text-xs text-muted-foreground mt-1">Max capacity {data?.max_size ?? 0}</p>
          </CardContent>
        </Card>
        
        <Card className="hover:border-primary/50 transition-colors">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Hits</CardTitle>
            <Search className="h-4 w-4 text-emerald-500" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{data?.hits ?? 0}</div>
          </CardContent>
        </Card>
        
        <Card className="hover:border-primary/50 transition-colors">
          <CardHeader className="flex flex-row items-center justify-between pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">Misses</CardTitle>
            <XCircle className="h-4 w-4 text-destructive" />
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">{data?.misses ?? 0}</div>
          </CardContent>
        </Card>
      </div>

      <Card className="border-border/50">
        <CardHeader className="border-b border-border/50 bg-muted/20">
          <div className="flex items-center justify-between">
            <CardTitle className="text-lg">Cache Entries ({(data?.entries ?? []).length})</CardTitle>
          </div>
        </CardHeader>
        
        {(data?.entries ?? []).length === 0 ? (
          <div className="flex flex-col items-center justify-center p-12 text-muted-foreground">
            <Database className="h-10 w-10 mb-4 opacity-50 text-border" />
            <p>Cache is currently empty.</p>
          </div>
        ) : (
          <Table>
            <TableHeader className="bg-muted/30">
              <TableRow>
                <TableHead className="w-[180px]">Key Hash</TableHead>
                <TableHead>Target Model</TableHead>
                <TableHead>Expires</TableHead>
                <TableHead className="text-right">Tokens</TableHead>
                <TableHead className="w-[100px] text-center">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data!.entries.map((e) => (
                <TableRow key={e.key} className="group cursor-pointer hover:bg-muted/50 transition-colors" onClick={() => viewEntry(e.key)}>
                  <TableCell>
                    <code className="text-xs font-mono text-muted-foreground group-hover:text-primary transition-colors" title={e.key}>
                      {e.key.slice(0, 16)}...
                    </code>
                  </TableCell>
                  <TableCell className="font-medium">{e.model}</TableCell>
                  <TableCell className="text-muted-foreground text-sm">{formatDate(e.expires_at)}</TableCell>
                  <TableCell className="text-right">
                    <Badge variant="secondary" className="font-mono text-xs">{e.token_count}</Badge>
                  </TableCell>
                  <TableCell className="text-center">
                    <Button variant="ghost" size="icon" className="h-8 w-8 text-muted-foreground group-hover:text-primary" onClick={(ev) => { ev.stopPropagation(); viewEntry(e.key); }}>
                      <ExternalLink className="h-4 w-4" />
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </Card>

      <AnimatePresence>
        {(detail || detailLoading || detailError) && (
          <div className="fixed inset-0 z-50 flex items-center justify-center pt-16">
            <motion.div 
              initial={{ opacity: 0 }} 
              animate={{ opacity: 1 }} 
              exit={{ opacity: 0 }} 
              onClick={closeDetail} 
              className="absolute inset-0 bg-background/80 backdrop-blur-sm"
            />
            <motion.div 
              initial={{ opacity: 0, scale: 0.95, y: 10 }} 
              animate={{ opacity: 1, scale: 1, y: 0 }} 
              exit={{ opacity: 0, scale: 0.95, y: 10 }}
              className="relative w-full max-w-2xl bg-card border border-border shadow-2xl rounded-xl z-50 overflow-hidden m-4 max-h-[85vh] flex flex-col"
            >
              <div className="flex items-center justify-between border-b border-border/50 px-6 py-4 bg-muted/20">
                <h3 className="font-semibold text-lg flex items-center gap-2">
                  <Database className="h-5 w-5 text-primary" />
                  Cache Details
                </h3>
                <Button variant="ghost" size="icon" onClick={closeDetail} className="h-8 w-8 rounded-full">
                  <X className="h-4 w-4" />
                </Button>
              </div>
              
              <div className="p-6 overflow-y-auto flex-1">
                {detailLoading && (
                  <div className="flex flex-col items-center justify-center py-12 text-muted-foreground">
                    <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary mb-4"></div>
                    Loading payload...
                  </div>
                )}
                {detailError && <div className="text-destructive bg-destructive/10 p-4 rounded-lg border border-destructive/20 font-medium">{detailError}</div>}
                
                {detail && (
                  <div className="space-y-4">
                    <div className="flex flex-wrap gap-2 mb-4">
                      <Badge variant="outline" className="bg-background">Model: {detail.model}</Badge>
                      <Badge variant="outline" className="bg-background">Tokens: {detail.token_count}</Badge>
                      <Badge variant="outline" className="bg-background">Expires: {formatDate(detail.expires_at)}</Badge>
                    </div>
                    <div>
                      <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">Key Reference</div>
                      <code className="block p-3 rounded-md bg-muted/50 border border-border/50 text-xs text-muted-foreground break-all font-mono">
                        {detail.key}
                      </code>
                    </div>
                    <div>
                      <div className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">Cached Response</div>
                      <pre className="block p-4 rounded-md bg-[#0a0a0a] border border-border/50 text-xs text-emerald-400 overflow-x-auto whitespace-pre-wrap word-break">
                        {JSON.stringify(detail.response, null, 2)}
                      </pre>
                    </div>
                  </div>
                )}
              </div>
            </motion.div>
          </div>
        )}
      </AnimatePresence>
    </div>
  )
}
