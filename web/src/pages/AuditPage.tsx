import { useState, useCallback } from 'react';
import { usePolling } from '@/hooks/usePolling';
import { api } from '@/api';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { FileText, ChevronLeft, ChevronRight, Filter } from 'lucide-react';
import type { AuditEntry, KeyInfo } from '@/types';

const PAGE_SIZE = 50;

function formatLatency(ms: number): string {
  if (ms >= 1000) return (ms / 1000).toFixed(1) + 's';
  return ms + 'ms';
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
  return String(n);
}

export default function AuditPage() {
  const [filterKey, setFilterKey] = useState('');
  const [offset, setOffset] = useState(0);

  const fetchLogs = useCallback(() => {
    return api.getAuditLogs(filterKey || undefined, PAGE_SIZE, offset);
  }, [filterKey, offset]);

  const { data: logData, loading, error } = usePolling(fetchLogs, 5000);
  const { data: keyData } = usePolling<{ keys: KeyInfo[] }>(api.getKeys, 10000);

  const logs = logData?.logs ?? [];
  const total = logData?.total ?? 0;
  const keys = keyData?.keys ?? [];
  const totalPages = Math.ceil(total / PAGE_SIZE);
  const currentPage = Math.floor(offset / PAGE_SIZE) + 1;

  const handlePrev = () => setOffset(Math.max(0, offset - PAGE_SIZE));
  const handleNext = () => {
    if (offset + PAGE_SIZE < total) setOffset(offset + PAGE_SIZE);
  };

  if (loading && logs.length === 0) return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
    </div>
  );
  if (error) return <div className="text-destructive font-semibold p-4 bg-destructive/10 rounded-lg border border-destructive/20">{error}</div>;

  return (
    <div className="space-y-6 pb-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">Audit Logs</h2>
          <p className="text-muted-foreground mt-1">Every API request — who called what model, token usage, latency.</p>
        </div>
      </div>

      {/* Filter bar */}
      <div className="flex items-center gap-3">
        <Filter className="w-4 h-4 text-muted-foreground" />
        <select
          value={filterKey}
          onChange={e => { setFilterKey(e.target.value); setOffset(0); }}
          className="px-3 py-1.5 rounded-md border bg-background text-sm"
        >
          <option value="">All keys</option>
          {keys.map(k => (
            <option key={k.name} value={k.name}>{k.name}</option>
          ))}
        </select>
        <span className="text-xs text-muted-foreground">{total.toLocaleString()} entries</span>
      </div>

      {logs.length === 0 ? (
        <Card className="flex flex-col items-center justify-center h-48 border-dashed">
          <FileText className="h-8 w-8 text-muted-foreground mb-4 opacity-50" />
          <p className="text-muted-foreground text-sm font-medium">No audit logs yet</p>
          <p className="text-xs text-muted-foreground/70 mt-1">Requests will appear here after they complete.</p>
        </Card>
      ) : (
        <Card className="overflow-hidden border-border/50">
          <Table>
            <TableHeader className="bg-muted/50">
              <TableRow>
                <TableHead>Time</TableHead>
                <TableHead>Key</TableHead>
                <TableHead>Model</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead className="text-right">Tokens</TableHead>
                <TableHead className="text-right">Latency</TableHead>
                <TableHead className="text-right">Status</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {logs.map((e: AuditEntry) => (
                <TableRow key={e.id}>
                  <TableCell className="text-xs text-muted-foreground whitespace-nowrap">
                    {e.created_at}
                  </TableCell>
                  <TableCell className="font-medium text-sm">{e.key_name || <span className="text-muted-foreground italic">anonymous</span>}</TableCell>
                  <TableCell className="text-sm">{e.model}</TableCell>
                  <TableCell className="text-sm">
                    <Badge variant="secondary" className="text-xs">{e.provider}</Badge>
                  </TableCell>
                  <TableCell className="text-right text-sm">
                    {e.total_tokens > 0 ? (
                      <span>
                        <span className="text-muted-foreground text-xs">{formatTokens(e.prompt_tokens)} in / </span>
                        {formatTokens(e.completion_tokens)} out
                      </span>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell className="text-right text-sm tabular-nums">
                    {e.latency_ms > 0 ? formatLatency(e.latency_ms) : <span className="text-muted-foreground">—</span>}
                  </TableCell>
                  <TableCell className="text-right">
                    {e.status_code >= 200 && e.status_code < 300 ? (
                      <Badge variant="success" className="h-6">{e.status_code}</Badge>
                    ) : e.status_code >= 400 && e.status_code < 500 ? (
                      <Badge variant="destructive" className="h-6">{e.status_code}</Badge>
                    ) : (
                      <Badge variant="destructive" className="h-6">{e.status_code}</Badge>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Card>
      )}

      {/* Pagination */}
      {total > PAGE_SIZE && (
        <div className="flex items-center justify-between">
          <span className="text-sm text-muted-foreground">
            Page {currentPage} of {totalPages} ({total.toLocaleString()} total)
          </span>
          <div className="flex gap-2">
            <button
              onClick={handlePrev}
              disabled={offset === 0}
              className="flex items-center gap-1 px-3 py-1.5 rounded-md border text-sm hover:bg-muted disabled:opacity-50"
            >
              <ChevronLeft className="w-4 h-4" />
              Previous
            </button>
            <button
              onClick={handleNext}
              disabled={offset + PAGE_SIZE >= total}
              className="flex items-center gap-1 px-3 py-1.5 rounded-md border text-sm hover:bg-muted disabled:opacity-50"
            >
              Next
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
