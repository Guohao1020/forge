'use client';

import { useEffect, useState } from 'react';
import { CheckCircle, AlertCircle, XCircle } from 'lucide-react';

interface HealthStatus {
  status: string;
  database: string;
  redis: string;
  temporal?: string;
  ai_worker?: string;
  uptime: string;
}

export function SystemStatus() {
  const [health, setHealth] = useState<HealthStatus | null>(null);
  const [error, setError] = useState(false);

  useEffect(() => {
    const check = async () => {
      try {
        const res = await fetch('/api/health');
        if (res.ok) {
          setHealth(await res.json());
          setError(false);
        } else {
          setError(true);
        }
      } catch {
        setError(true);
      }
    };
    check();
    const interval = setInterval(check, 30000);
    return () => clearInterval(interval);
  }, []);

  if (error || !health) {
    return (
      <div className="flex items-center gap-2 px-3 py-2 text-xs text-red-400" title="Cannot reach forge-core">
        <XCircle className="h-3 w-3" />
        <span>System offline</span>
      </div>
    );
  }

  const services = [
    { name: 'Database', status: health.database },
    { name: 'Redis', status: health.redis },
    { name: 'Temporal', status: health.temporal || 'unknown' },
    { name: 'AI Worker', status: health.ai_worker || 'unknown' },
  ];

  const allUp = services.every(s => s.status === 'up');
  const criticalDown = health.database !== 'up' || health.temporal !== 'up';

  const dotColor = criticalDown ? 'bg-red-400' : allUp ? 'bg-emerald-400' : 'bg-yellow-400';

  return (
    <div className="group relative px-3 py-2">
      <div className="flex items-center gap-2 text-xs text-[var(--muted-foreground)]">
        <div className={`h-2 w-2 rounded-full ${dotColor} animate-pulse`} />
        <span>{allUp ? 'All systems operational' : criticalDown ? 'Critical service down' : 'Degraded'}</span>
      </div>

      {/* Tooltip on hover */}
      <div className="absolute bottom-full left-0 mb-2 hidden group-hover:block z-50">
        <div className="rounded-lg border border-[var(--border)] bg-[var(--card)] p-3 shadow-xl min-w-[200px]">
          <div className="text-xs font-medium text-[var(--foreground)] mb-2">System Status</div>
          {services.map(s => (
            <div key={s.name} className="flex items-center justify-between py-1 text-xs">
              <span className="text-[var(--muted-foreground)]">{s.name}</span>
              <span className={s.status === 'up' ? 'text-emerald-400' : s.status === 'unknown' ? 'text-[var(--muted-foreground)]' : 'text-red-400'}>
                {s.status === 'up' ? '● Up' : s.status === 'unknown' ? '○ N/A' : '● Down'}
              </span>
            </div>
          ))}
          <div className="mt-2 pt-2 border-t border-[var(--border)] text-xs text-[var(--muted-foreground)]">
            Uptime: {health.uptime}
          </div>
        </div>
      </div>
    </div>
  );
}
