import { useEffect, useState, useCallback, useRef } from 'react';

export function usePolling<T>(
  fetcher: () => Promise<T>,
  intervalMs: number
): { data: T | null; error: string | null; loading: boolean } {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const mounted = useRef(true);

  const refresh = useCallback(() => {
    fetcher()
      .then((d) => {
        if (mounted.current) {
          setData(d);
          setError(null);
          setLoading(false);
        }
      })
      .catch((e: Error) => {
        if (mounted.current) {
          setError(e.message);
          setLoading(false);
        }
      });
  }, [fetcher]);

  useEffect(() => {
    mounted.current = true;
    refresh();
    const id = setInterval(refresh, intervalMs);
    return () => {
      mounted.current = false;
      clearInterval(id);
    };
  }, [intervalMs, refresh]);

  return { data, error, loading };
}
