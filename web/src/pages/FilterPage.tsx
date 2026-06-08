import { usePolling } from '@/hooks/usePolling';
import { api } from '@/api';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Shield, ShieldOff } from 'lucide-react';
import type { FilterStatus } from '@/types';

export default function FilterPage() {
  const { data, loading, error } = usePolling<FilterStatus>(api.getFilterStatus, 10000);

  if (loading) return (
    <div className="flex h-[50vh] items-center justify-center">
      <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
    </div>
  );
  if (error) return <div className="text-destructive font-semibold p-4 bg-destructive/10 rounded-lg border border-destructive/20">{error}</div>;

  if (!data) return null;

  return (
    <div className="space-y-6 pb-8">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">PII Filter</h2>
          <p className="text-muted-foreground mt-1">Sensitive information detection and masking for prompts and responses.</p>
        </div>
        <div className="flex items-center gap-2">
          {data.enabled ? (
            <Badge variant="success" className="h-6 gap-1">
              <Shield className="w-3.5 h-3.5" />
              Active
            </Badge>
          ) : (
            <Badge variant="outline" className="h-6 gap-1">
              <ShieldOff className="w-3.5 h-3.5" />
              Disabled
            </Badge>
          )}
          <Badge variant="secondary" className="h-6">{data.mode === 'mask' ? 'Mask mode' : 'Block mode'}</Badge>
        </div>
      </div>

      <Card className="overflow-hidden border-border/50">
        <div className="p-6">
          <h3 className="text-sm font-semibold mb-4">Detection Rules</h3>
          <div className="space-y-3">
            {data.rules.map((rule) => (
              <div
                key={rule.name}
                className="flex items-center justify-between p-3 rounded-lg border bg-muted/30"
              >
                <div className="flex-1">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-sm">{rule.label}</span>
                    <code className="text-xs bg-muted px-1.5 py-0.5 rounded">{rule.name}</code>
                  </div>
                  <p className="text-xs text-muted-foreground mt-0.5">{rule.description}</p>
                </div>
                <div>
                  {rule.enabled ? (
                    <Badge variant="success" className="h-5 text-xs">Enabled</Badge>
                  ) : (
                    <Badge variant="outline" className="h-5 text-xs">Disabled</Badge>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </Card>

      <Card className="p-6 border-border/50">
        <h3 className="text-sm font-semibold mb-2">How it works</h3>
        <ul className="text-sm text-muted-foreground space-y-1 list-disc list-inside">
          <li><strong>Mask mode:</strong> PII in prompts and responses is replaced with <code className="bg-muted px-1 rounded">[REDACTED_rule_name]</code> before reaching the LLM or the client.</li>
          <li><strong>Block mode:</strong> Requests containing PII are rejected with HTTP 422 before any upstream call.</li>
          <li>Rules are configured in <code className="bg-muted px-1 rounded">config/gateway.yaml</code> under the <code className="bg-muted px-1 rounded">filter</code> section.</li>
          <li>Longer patterns (e.g. ID card) are matched before shorter ones (e.g. phone) to prevent partial redaction.</li>
        </ul>
      </Card>
    </div>
  );
}
