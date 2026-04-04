"use client";

import { useEffect, useRef, useCallback, useState } from "react";

export interface TaskStreamEvent {
  type: "connected" | "FULL_STATE" | "TASK_PROGRESS" | "STEPS_UPDATE" | "TASK_COMPLETE" | "code_token" | "analyze_token";
  task_id: number;
  status?: string;
  step_type?: string;
  step_name?: string;
  progress?: number;
  data?: string;
}

interface UseTaskStreamOptions {
  taskId: string | number;
  onEvent?: (event: TaskStreamEvent) => void;
  enabled?: boolean;
}

export function useTaskStream({ taskId, onEvent, enabled = true }: UseTaskStreamOptions) {
  const [connected, setConnected] = useState(false);
  const [streamingTokens, setStreamingTokens] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [analyzeThinking, setAnalyzeThinking] = useState("");
  const [isAnalyzing, setIsAnalyzing] = useState(false);
  const eventSourceRef = useRef<EventSource | null>(null);
  const onEventRef = useRef(onEvent);
  useEffect(() => {
    onEventRef.current = onEvent;
  });

  const disconnect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    setConnected(false);
    setIsStreaming(false);
    setIsAnalyzing(false);
  }, []);

  useEffect(() => {
    if (!enabled || !taskId) return;

    const token = localStorage.getItem("forge_token");
    if (!token) return;

    // SSE must connect directly to Go backend — Next.js rewrites buffer the response
    const sseBase = process.env.NEXT_PUBLIC_SSE_URL || "http://localhost:8080";
    const url = `${sseBase}/api/stream/tasks/${taskId}?token=${encodeURIComponent(token)}`;
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

        // Handle analyze_token events (AI thinking process)
        if (event.type === "analyze_token") {
          const payload = event.data ?? "";
          try {
            const parsed = JSON.parse(payload);
            if (parsed.event === "thinking_start") {
              setAnalyzeThinking("");
              setIsAnalyzing(true);
            } else if (parsed.event === "thinking_end") {
              setIsAnalyzing(false);
            } else {
              // Raw text chunk
              setAnalyzeThinking((prev) => prev + payload);
            }
          } catch {
            // Not JSON — treat as raw text chunk
            setAnalyzeThinking((prev) => prev + payload);
            setIsAnalyzing(true);
          }
        }

        // Handle code_token events
        if (event.type === "code_token") {
          const chunk = event.data ?? "";
          setStreamingTokens((prev) => prev + chunk);
          setIsStreaming(true);
        }

        // Reset streaming when a new GENERATE step starts
        if (
          event.type === "STEPS_UPDATE" &&
          event.step_type === "GENERATE" &&
          event.status === "RUNNING"
        ) {
          setStreamingTokens("");
          setIsStreaming(false);
        }

        // Stop streaming when GENERATE completes
        if (
          event.type === "STEPS_UPDATE" &&
          event.step_type === "GENERATE" &&
          event.status === "COMPLETED"
        ) {
          setIsStreaming(false);
        }

        // Stop streaming on task complete
        if (event.type === "TASK_COMPLETE") {
          setIsStreaming(false);
        }

        onEventRef.current?.(event);
      } catch {
        // Ignore malformed messages
      }
    };

    es.onerror = () => {
      setConnected(false);
      setIsStreaming(false);
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
      setIsStreaming(false);
    };
  }, [taskId, enabled]);

  return { connected, disconnect, streamingTokens, isStreaming, analyzeThinking, isAnalyzing };
}
