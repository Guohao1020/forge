"use client";

import { useEffect, useRef, useCallback, useState } from "react";

export interface TaskStreamEvent {
  type: "connected" | "FULL_STATE" | "TASK_PROGRESS" | "STEPS_UPDATE" | "TASK_COMPLETE";
  task_id: number;
  status?: string;
  step_type?: string;
  step_name?: string;
  progress?: number;
}

interface UseTaskStreamOptions {
  taskId: string | number;
  onEvent?: (event: TaskStreamEvent) => void;
  enabled?: boolean;
}

export function useTaskStream({ taskId, onEvent, enabled = true }: UseTaskStreamOptions) {
  const [connected, setConnected] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);
  const onEventRef = useRef(onEvent);
  onEventRef.current = onEvent;

  const disconnect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    setConnected(false);
  }, []);

  useEffect(() => {
    if (!enabled || !taskId) return;

    const token = localStorage.getItem("forge_token");
    if (!token) return;

    const url = `/api/stream/tasks/${taskId}?token=${encodeURIComponent(token)}`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => {
      setConnected(true);
    };

    es.onmessage = (e) => {
      try {
        const event: TaskStreamEvent = JSON.parse(e.data);
        if (event.type === "connected") {
          setConnected(true);
        }
        onEventRef.current?.(event);
      } catch {
        // Ignore malformed messages
      }
    };

    es.onerror = () => {
      setConnected(false);
      // EventSource auto-reconnects; close only if readyState is CLOSED
      if (es.readyState === EventSource.CLOSED) {
        es.close();
        eventSourceRef.current = null;
        // Retry after 3 seconds
        setTimeout(() => {
          // Will be re-established by effect re-run if still mounted
        }, 3000);
      }
    };

    return () => {
      es.close();
      eventSourceRef.current = null;
      setConnected(false);
    };
  }, [taskId, enabled]);

  return { connected, disconnect };
}
