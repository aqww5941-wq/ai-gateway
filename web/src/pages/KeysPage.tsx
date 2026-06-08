import { useState } from 'react';
import { usePolling } from '@/hooks/usePolling';
import { api } from '@/api';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Key, Plus, Trash2, Power, PowerOff, Copy, Check, Zap, Shield, User } from 'lucide-react';
import type { KeyInfo, CreateKeyResponse } from '@/types';

function formatTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
  return String(n);
}

function UsageBar({ used, limit }: { used: number; limit: number }) {
  if (limit <= 0) return <span className="text-xs text-muted-foreground">unlimited</span>;
  const pct = Math.min((used / limit) * 100, 100);
  let barColor = 'bg-emerald-500';
  if (pct > 90) barColor = 'bg-red-500';
  else if (pct > 75) barColor = 'bg-amber-500';

  return (
    <div className="flex items-center gap-2">
      <div className="h-2 flex-1 rounded-full bg-muted overflow-hidden">
        <div className={`h-full rounded-full transition-all ${barColor}`} style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xs text-muted-foreground whitespace-nowrap">{pct.toFixed(0)}%</span>
    </div>
  );
}

export default function KeysPage() {
  const { data: keyData, loading: keysLoading, error: keysError } = usePolling<{ keys: KeyInfo[] }>(api.getKeys, 5000);
  const { data: quotaData } = usePolling<{ quotas: Array<{ name: string; used_tokens: number; daily_limit: number }>; enabled: boolean }>(api.getQuotas, 10000);

  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newRole, setNewRole] = useState('user');
  const [newDaily, setNewDaily] = useState('1000000');
  const [newMonthly, setNewMonthly] = useState('');
  const [newModels, setNewModels] = useState('');
  const [createdToken, setCreatedToken] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);

  const keys = keyData?.keys ?? [];
  const quotas = quotaData?.quotas ?? [];

  const getUsage = (name: string) => {
    return quotas.find(q => q.name === name);
  };

  const handleCreate = async () => {
    setCreateError(null);
    try {
      const result: CreateKeyResponse = await api.createKey(
        newName,
        newRole,
        newModels,
        Number(newDaily) || 0,
        Number(newMonthly) || 0
      );
      setCreatedToken(result.token);
      setNewName('');
      setNewRole('user');
      setNewDaily('1000000');
      setNewMonthly('');
      setNewModels('');
    } catch (e: unknown) {
      setCreateError(e instanceof Error ? e.message : String(e));
    }
  };

  const handleToggleActive = async (key: KeyInfo) => {
    try {
      await api.updateKey(key.id, { is_active: !key.is_active });
    } catch { /* ignore */ }
  };

  const handleDelete = async (key: KeyInfo) => {
    if (!confirm(`Delete key "${key.name}"? This cannot be undone.`)) return;
    try {
      await api.deleteKey(key.id);
    } catch { /* ignore */ }
  };

  if (keysLoading) return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
    </div>
  );
  if (keysError) return <div className="text-destructive font-semibold p-4 bg-destructive/10 rounded-lg border border-destructive/20">{keysError}</div>;

  return (
    <div className="space-y-6 pb-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">API Keys ({keys.length})</h2>
          <p className="text-muted-foreground mt-1">Manage API keys, roles, and model access.</p>
        </div>
        <button
          onClick={() => { setShowCreate(true); setCreatedToken(null); setCreateError(null); }}
          className="flex items-center gap-2 px-4 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium hover:bg-primary/90"
        >
          <Plus className="w-4 h-4" />
          New Key
        </button>
      </div>

      {/* Create Key Dialog */}
      {showCreate && (
        <Card className="p-6 border-primary/30">
          <h3 className="font-semibold mb-4">Create API Key</h3>
          {createdToken ? (
            <div className="space-y-4">
              <div className="bg-emerald-500/10 border border-emerald-500/20 rounded-lg p-4">
                <p className="text-sm text-emerald-500 font-medium mb-2">Key created! Copy it now — it won't be shown again.</p>
                <div className="flex items-center gap-2">
                  <code className="flex-1 bg-muted px-3 py-2 rounded text-sm break-all font-mono">{createdToken}</code>
                  <button
                    onClick={() => { navigator.clipboard.writeText(createdToken); setCopied(true); }}
                    className="p-2 rounded-md hover:bg-muted"
                  >
                    {copied ? <Check className="w-4 h-4 text-emerald-500" /> : <Copy className="w-4 h-4" />}
                  </button>
                </div>
              </div>
              <button
                onClick={() => { setShowCreate(false); setCreatedToken(null); setCopied(false); }}
                className="px-4 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium"
              >
                Done
              </button>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Name</label>
                  <input
                    type="text"
                    value={newName}
                    onChange={e => setNewName(e.target.value)}
                    placeholder="e.g. dev-team"
                    className="w-full mt-1 px-3 py-2 rounded-md border bg-background text-sm"
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Role</label>
                  <select
                    value={newRole}
                    onChange={e => setNewRole(e.target.value)}
                    className="w-full mt-1 px-3 py-2 rounded-md border bg-background text-sm"
                  >
                    <option value="user">User</option>
                    <option value="admin">Admin</option>
                  </select>
                </div>
              </div>
              <div>
                <label className="text-sm font-medium">Allowed Models (comma-separated, empty = all)</label>
                <input
                  type="text"
                  value={newModels}
                  onChange={e => setNewModels(e.target.value)}
                  placeholder="e.g. deepseek-v4-flash,kimi"
                  className="w-full mt-1 px-3 py-2 rounded-md border bg-background text-sm"
                />
              </div>
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium">Daily Token Limit (0 = unlimited)</label>
                  <input
                    type="number"
                    value={newDaily}
                    onChange={e => setNewDaily(e.target.value)}
                    className="w-full mt-1 px-3 py-2 rounded-md border bg-background text-sm"
                  />
                </div>
                <div>
                  <label className="text-sm font-medium">Monthly Token Limit (0 = unlimited)</label>
                  <input
                    type="number"
                    value={newMonthly}
                    onChange={e => setNewMonthly(e.target.value)}
                    placeholder="0"
                    className="w-full mt-1 px-3 py-2 rounded-md border bg-background text-sm"
                  />
                </div>
              </div>
              {createError && <p className="text-sm text-destructive">{createError}</p>}
              <div className="flex gap-2 justify-end">
                <button onClick={() => setShowCreate(false)} className="px-4 py-2 rounded-md text-sm border">Cancel</button>
                <button
                  onClick={handleCreate}
                  disabled={!newName.trim()}
                  className="px-4 py-2 rounded-md bg-primary text-primary-foreground text-sm font-medium disabled:opacity-50"
                >
                  Create
                </button>
              </div>
            </div>
          )}
        </Card>
      )}

      {keys.length === 0 ? (
        <Card className="flex flex-col items-center justify-center h-48 border-dashed">
          <Key className="h-8 w-8 text-muted-foreground mb-4 opacity-50" />
          <p className="text-muted-foreground text-sm font-medium">No API keys yet</p>
          <p className="text-xs text-muted-foreground/70 mt-1">Create one to get started.</p>
        </Card>
      ) : (
        <Card className="overflow-hidden border-border/50">
          <Table>
            <TableHeader className="bg-muted/50">
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Role</TableHead>
                <TableHead>Models</TableHead>
                <TableHead>Daily Limit</TableHead>
                <TableHead className="w-[150px]">Daily Usage</TableHead>
                <TableHead className="text-right">Used Today</TableHead>
                <TableHead className="text-right">Status</TableHead>
                <TableHead className="text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {keys.map((k) => {
                const usage = getUsage(k.name);
                const used = usage?.used_tokens ?? 0;
                return (
                  <TableRow key={k.id}>
                    <TableCell className="font-medium">
                      <div className="flex items-center gap-2">
                        <Key className="w-4 h-4 text-muted-foreground" />
                        {k.name}
                      </div>
                    </TableCell>
                    <TableCell>
                      {k.role === 'admin' ? (
                        <Badge variant="default" className="h-6 gap-1">
                          <Shield className="w-3 h-3" />
                          Admin
                        </Badge>
                      ) : (
                        <Badge variant="secondary" className="h-6 gap-1">
                          <User className="w-3 h-3" />
                          User
                        </Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      {k.models ? (
                        <span className="text-xs text-muted-foreground">{k.models}</span>
                      ) : (
                        <Badge variant="outline" className="text-xs">All models</Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      {k.daily_limit > 0 ? (
                        <Badge variant="secondary">{formatTokens(k.daily_limit)}</Badge>
                      ) : (
                        <Badge variant="outline">Unlimited</Badge>
                      )}
                    </TableCell>
                    <TableCell>
                      <UsageBar used={used} limit={k.daily_limit} />
                    </TableCell>
                    <TableCell className="text-right">
                      <span className="flex items-center justify-end gap-1">
                        <Zap className="w-3 h-3 text-muted-foreground" />
                        <span className={used > 0 ? 'text-foreground' : 'text-muted-foreground'}>
                          {formatTokens(used)}
                        </span>
                      </span>
                    </TableCell>
                    <TableCell className="text-right">
                      {k.is_active ? (
                        <Badge variant="success" className="h-6">Active</Badge>
                      ) : (
                        <Badge variant="destructive" className="h-6">Disabled</Badge>
                      )}
                    </TableCell>
                    <TableCell className="text-right">
                      <div className="flex items-center justify-end gap-1">
                        <button
                          onClick={() => handleToggleActive(k)}
                          className="p-1.5 rounded hover:bg-muted"
                          title={k.is_active ? 'Disable' : 'Enable'}
                        >
                          {k.is_active ? <PowerOff className="w-4 h-4 text-amber-500" /> : <Power className="w-4 h-4 text-emerald-500" />}
                        </button>
                        <button
                          onClick={() => handleDelete(k)}
                          className="p-1.5 rounded hover:bg-muted"
                          title="Delete"
                        >
                          <Trash2 className="w-4 h-4 text-destructive" />
                        </button>
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </Card>
      )}
    </div>
  );
}
