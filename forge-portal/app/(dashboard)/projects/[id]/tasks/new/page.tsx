"use client";

import { useState, useCallback } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import { ArrowLeft } from "lucide-react";
import { ChatPanel } from "@/components/chat/chat-panel";
import { createTask } from "@/lib/tasks";
import {
  Conversation,
  SendMessageResponse,
  sendMessage,
  confirmPlan,
} from "@/lib/conversation";

export default function NewTaskPage() {
  const params = useParams();
  const router = useRouter();
  const projectId = Number(params.id);

  const [messages, setMessages] = useState<Conversation[]>([]);
  const [isLoading, setIsLoading] = useState(false);
  const [isConfirming, setIsConfirming] = useState(false);
  const [taskId, setTaskId] = useState<number | null>(null);
  const [confirmationData, setConfirmationData] = useState<{
    summary: string;
    taskTitle: string;
    affectedModules?: string[];
    riskLevel?: string;
    estimatedComplexity?: string;
  } | null>(null);

  const handleSend = useCallback(
    async (content: string) => {
      // Optimistic: add user message immediately
      const userMsg: Conversation = {
        id: Date.now(),
        taskId: taskId ?? 0,
        role: "user",
        content,
        createdAt: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, userMsg]);
      setIsLoading(true);

      try {
        let currentTaskId = taskId;

        // First message: create task
        if (!currentTaskId) {
          const detail = await createTask(projectId, content);
          currentTaskId = detail.task.id;
          setTaskId(currentTaskId);
        }

        // Send message to AI
        const res: SendMessageResponse = await sendMessage(
          projectId,
          currentTaskId,
          content
        );

        // Add AI response
        setMessages((prev) => [...prev, res.conversation]);

        // If confirmed, show confirmation card
        if (res.status === "confirmed" && res.metadata) {
          setConfirmationData({
            summary: (res.metadata.summary as string) || res.conversation.content,
            taskTitle: (res.metadata.taskTitle as string) || content,
            affectedModules: res.metadata.affectedModules as string[] | undefined,
            riskLevel: res.metadata.riskLevel as string | undefined,
            estimatedComplexity: res.metadata.estimatedComplexity as string | undefined,
          });
        }
      } catch (err) {
        // Add error as system message
        const errMsg: Conversation = {
          id: Date.now() + 1,
          taskId: taskId ?? 0,
          role: "system",
          content: err instanceof Error ? err.message : "Failed to send message",
          createdAt: new Date().toISOString(),
        };
        setMessages((prev) => [...prev, errMsg]);
      } finally {
        setIsLoading(false);
      }
    },
    [projectId, taskId]
  );

  const handleConfirm = useCallback(async () => {
    if (!taskId) return;
    setIsConfirming(true);
    try {
      await confirmPlan(projectId, taskId);
      router.push(`/projects/${projectId}/tasks/${taskId}`);
    } catch (err) {
      const errMsg: Conversation = {
        id: Date.now(),
        taskId,
        role: "system",
        content: err instanceof Error ? err.message : "Confirmation failed",
        createdAt: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, errMsg]);
    } finally {
      setIsConfirming(false);
    }
  }, [projectId, taskId, router]);

  const handleModify = useCallback(() => {
    setConfirmationData(null);
  }, []);

  const handleCancel = useCallback(() => {
    router.push(`/projects/${projectId}`);
  }, [projectId, router]);

  return (
    <div className="flex flex-col h-[calc(100vh-64px)]">
      <div className="px-4 py-3 border-b border-white/10">
        <Link
          href={`/projects/${projectId}`}
          className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to tasks
        </Link>
        <h1 className="text-lg font-semibold mt-1">New requirement</h1>
      </div>

      <ChatPanel
        messages={messages}
        onSend={handleSend}
        onConfirm={handleConfirm}
        onModify={handleModify}
        onCancel={handleCancel}
        isLoading={isLoading}
        confirmationData={confirmationData}
        isConfirming={isConfirming}
      />
    </div>
  );
}
