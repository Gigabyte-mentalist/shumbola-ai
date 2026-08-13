import { useEffect, useRef, useState } from 'react';

/**
 * Refetches `fetcher` every `intervalMs` while `enabled` is true. There's
 * no websocket/SSE on the backend (see the Web UI plan), so periodic
 * polling is the pragmatic way to approximate "real-vaqtli kuzatuv"
 * (TZ 4.7) — ReAct steps take seconds-to-minutes anyway (LLM latency),
 * so a few-second interval is plenty responsive.
 */
export function usePolling<T>(
  fetcher: () => Promise<T>,
  intervalMs: number,
  enabled: boolean,
  deps: unknown[],
): { data: T | null; error: string | null; refetch: () => void } {
  const [data, setData] = useState<T | null>(null);
  const [error, setError] = useState<string | null>(null);
  const fetcherRef = useRef(fetcher);
  fetcherRef.current = fetcher;

  const [tick, setTick] = useState(0);
  const refetch = () => setTick((t) => t + 1);

  useEffect(() => {
    let cancelled = false;

    const run = () => {
      fetcherRef
        .current()
        .then((result) => {
          if (!cancelled) {
            setData(result);
            setError(null);
          }
        })
        .catch((err: unknown) => {
          if (!cancelled) {
            setError(err instanceof Error ? err.message : String(err));
          }
        });
    };

    run();
    if (!enabled) {
      return () => {
        cancelled = true;
      };
    }
    const id = setInterval(run, intervalMs);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, intervalMs, tick, ...deps]);

  return { data, error, refetch };
}
