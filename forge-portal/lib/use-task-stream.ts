"use client";

import { useEffect, useRef, useCallback, useState } from "react";

export interface TaskStreamEvent {
  type: "connected" | "FULL_STATE" | "TASK_PROGRESS" | "STEPS_UPDATE" | "TASK_COMPLETE" | "code_token" | "analyze_token" | "ANALYSIS_COMPLETE" | "PLAN_COMPLETE";
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

// Flush interval for batching streaming tokens into state updates (ms).
// Reduces React re-renders from ~50/s to ~7/s during AI streaming.
const TOKEN_FLUSH_INTERVAL = 150;

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

  // Token accumulation refs — batch updates via interval instead of per-token setState
  const thinkingBufferRef = useRef("");
  const codeBufferRef = useRef("");
  const flushIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const disconnect = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
    if (flushIntervalRef.current) {
      clearInterval(flushIntervalRef.current);
      flushIntervalRef.current = null;
    }
    setConnected(false);
    setIsStreaming(false);
    setIsAnalyzing(false);
  }, []);

  useEffect(() => {
    if (!enabled || !taskId) return;

    const token = localStorage.getItem("forge_token");
    if (!token) return;

    const sseBase = process.env.NEXT_PUBLIC_SSE_URL || "http://localhost:8080";
    const url = `${sseBase}/api/stream/tasks/${taskId}?token=${encodeURIComponent(token)}`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    // Start flush interval for batching token updates
    flushIntervalRef.current = setInterval(() => {
      if (thinkingBufferRef.current) {
        const buf = thinkingBufferRef.current;
        thinkingBufferRef.current = "";
        setAnalyzeThinking((prev) => prev + buf);
      }
      if (codeBufferRef.current) {
        const buf = codeBufferRef.current;
        codeBufferRef.current = "";
        setStreamingTokens((prev) => prev + buf);
      }
    }, TOKEN_FLUSH_INTERVAL);

    es.onopen = () => {
      setConnected(true);
      // Reconnect catch-up: notify parent to re-fetch state in case events were missed
      onEventRef.current?.({ type: "connected", task_id: Number(taskId) });
    };

    es.onmessage = (e) => {
      try {
        const event: TaskStreamEvent = JSON.parse(e.data);
        if (event.type === "connected") {
          setConnected(true);
        }

        // Handle analyze_token events — accumulate in ref, flush via interval
        if (event.type === "analyze_token") {
          const payload = event.data ?? "";
          try {
            const parsed = JSON.parse(payload);
            if (parsed.event === "thinking_start") {
              setAnalyzeThinking("");
              thinkingBufferRef.current = "";
              setIsAnalyzing(true);
            } else if (parsed.event === "thinking_end") {
              // Flush remaining buffer
              if (thinkingBufferRef.current) {
                const buf = thinkingBufferRef.current;
                thinkingBufferRef.current = "";
                setAnalyzeThinking((prev) => prev + buf);
              }
              setIsAnalyzing(false);
            } else {
              thinkingBufferRef.current += payload;
              setIsAnalyzing(true);
            }
          } catch {
            thinkingBufferRef.current += payload;
            setIsAnalyzing(true);
          }
        }

        // Handle code_token events — accumulate in ref, flush via interval
        if (event.type === "code_token") {
          codeBufferRef.current += (event.data ?? "");
          setIsStreaming(true);
        }

        // Reset streaming when a new GENERATE step starts
        if (
          event.type === "STEPS_UPDATE" &&
          event.step_type === "GENERATE" &&
          event.status === "RUNNING"
        ) {
          setStreamingTokens("");
          codeBufferRef.current = "";
          setIsStreaming(false);
        }

        // Stop streaming when GENERATE completes
        if (
          event.type === "STEPS_UPDATE" &&
          event.step_type === "GENERATE" &&
          event.status === "COMPLETED"
        ) {
          // Flush remaining
          if (codeBufferRef.current) {
            const buf = codeBufferRef.current;
            codeBufferRef.current = "";
            setStreamingTokens((prev) => prev + buf);
          }
          setIsStreaming(false);
        }

        // Clear analyzing state when analysis completes
        if (event.type === "ANALYSIS_COMPLETE") {
          if (thinkingBufferRef.current) {
            const buf = thinkingBufferRef.current;
            thinkingBufferRef.current = "";
            setAnalyzeThinking((prev) => prev + buf);
          }
          setIsAnalyzing(false);
        }

        // Stop streaming on task complete — frontend closes SSE
        if (event.type === "TASK_COMPLETE") {
          setIsStreaming(false);
          setIsAnalyzing(false);
          // Frontend-initiated close to avoid reconnect loop
          es.close();
          eventSourceRef.current = null;
          if (flushIntervalRef.current) {
            clearInterval(flushIntervalRef.current);
            flushIntervalRef.current = null;
          }
          setConnected(false);
        }

        onEventRef.current?.(event);
      } catch {
        // Ignore malformed messages
      }
    };

    es.onerror = () => {
      setConnected(false);
      setIsStreaming(false);
      if (es.readyState === EventSource.CLOSED) {
        es.close();
        eventSourceRef.current = null;
        // Will be re-established by effect re-run if still mounted
      }
    };

    return () => {
      es.close();
      eventSourceRef.current = null;
      if (flushIntervalRef.current) {
        clearInterval(flushIntervalRef.current);
        flushIntervalRef.current = null;
      }
      setConnected(false);
      setIsStreaming(false);
    };
  }, [taskId, enabled]);

  const resetAnalyzing = useCallback(() => {
    setAnalyzeThinking("");
    thinkingBufferRef.current = "";
    setIsAnalyzing(false);
  }, []);

  return { connected, disconnect, streamingTokens, isStreaming, analyzeThinking, isAnalyzing, resetAnalyzing };
}
