import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { axe } from "vitest-axe"
import { AgentChat, detectFixLoopStart } from "./agent-chat"

// Minimal EventSource stub so useEffect SSE setup doesn't throw in jsdom.
class MockEventSource {
  onopen: ((ev?: Event) => void) | null = null
  onerror: ((ev?: Event) => void) | null = null
  listeners: Record<string, Array<(e: MessageEvent) => void>> = {}
  constructor(public url: string) {
    // Auto-open async to mimic real EventSource.
    setTimeout(() => this.onopen?.(), 0)
  }
  addEventListener(type: string, fn: (e: MessageEvent) => void) {
    ;(this.listeners[type] ??= []).push(fn)
  }
  close() {}
}

describe("AgentChat", () => {
  beforeEach(() => {
    // @ts-expect-error — override global EventSource for jsdom.
    globalThis.EventSource = MockEventSource
    localStorage.setItem("forge_token", "fake-jwt")
    globalThis.fetch = vi.fn(async (input: unknown) => {
      const url = typeof input === "string" ? input : (input as Request).url
      if (url.includes("/agent/suggestions")) {
        return new Response(
          JSON.stringify({
            code: 0,
            message: "ok",
            data: {
              suggestions: [
                { text: "Add user registration with JWT auth" },
                { text: "Fix the login bug in feat/auth" },
                { text: "Write tests for the API" },
              ],
              source: "fallback",
            },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        )
      }
      return new Response(
        JSON.stringify({ code: 0, message: "ok", data: { session_id: "sess-123" } }),
        { status: 202, headers: { "Content-Type": "application/json" } },
      )
    }) as typeof fetch
  })

  afterEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  it("renders empty state with 3 CLI Try suggestions", () => {
    render(<AgentChat projectId="1" sessionId={null} />)
    expect(
      screen.getByText(/describe what you want to build/i),
    ).toBeInTheDocument()
    const tryButtons = screen
      .getAllByRole("button")
      .filter((b) => b.textContent?.includes("\u2192 Try:"))
    expect(tryButtons.length).toBe(3)
  })

  it("renders the chat conversation with role=log", () => {
    render(<AgentChat projectId="1" sessionId={null} />)
    const log = screen.getByRole("log", { name: /agent conversation/i })
    expect(log).toHaveAttribute("aria-live", "polite")
  })

  it("the textarea has the Forge Agent aria-label", () => {
    render(<AgentChat projectId="1" sessionId={null} />)
    expect(screen.getByLabelText("Chat with Forge Agent")).toBeInTheDocument()
  })

  it("clicking a suggestion fills the textarea", () => {
    render(<AgentChat projectId="1" sessionId={null} />)
    const btn = screen
      .getAllByRole("button")
      .find((b) => b.textContent?.includes("Add user registration"))
    expect(btn).toBeDefined()
    fireEvent.click(btn!)
    const textarea = screen.getByLabelText("Chat with Forge Agent") as HTMLTextAreaElement
    expect(textarea.value).toContain("Add user registration")
  })

  it("Send button is disabled when input is empty", () => {
    render(<AgentChat projectId="1" sessionId={null} />)
    const send = screen.getByRole("button", { name: /send message/i })
    expect(send).toBeDisabled()
  })

  it("Send button enables when input has text", () => {
    render(<AgentChat projectId="1" sessionId={null} />)
    const textarea = screen.getByLabelText("Chat with Forge Agent")
    fireEvent.change(textarea, { target: { value: "build me a calculator" } })
    const send = screen.getByRole("button", { name: /send message/i })
    expect(send).not.toBeDisabled()
  })

  it("clicking Send POSTs to /api/projects/:id/agent/chat", async () => {
    const mockFetch = vi.fn(async (input: unknown) => {
      const url = typeof input === "string" ? input : (input as Request).url
      if (url.includes("/agent/suggestions")) {
        return new Response(
          JSON.stringify({
            code: 0, message: "ok",
            data: { suggestions: [], source: "fallback" },
          }),
          { status: 200 },
        )
      }
      return new Response(
        JSON.stringify({ code: 0, message: "ok", data: { session_id: "s1" } }),
        { status: 202 },
      )
    }) as typeof fetch
    globalThis.fetch = mockFetch

    render(<AgentChat projectId="42" sessionId={null} />)
    const textarea = screen.getByLabelText("Chat with Forge Agent")
    fireEvent.change(textarea, { target: { value: "hello" } })
    const send = screen.getByRole("button", { name: /send message/i })
    fireEvent.click(send)

    await waitFor(() => {
      const calls = (mockFetch as unknown as { mock: { calls: Array<[string | Request]> } }).mock.calls
      const chatCall = calls.find(([u]) =>
        (typeof u === "string" ? u : (u as Request).url).includes("/agent/chat"),
      )
      expect(chatCall).toBeDefined()
    })
  })

  it("notifies parent of conn status changes via callback", async () => {
    const onConnStatusChange = vi.fn()
    render(
      <AgentChat
        projectId="1"
        sessionId="sess-1"
        onConnStatusChange={onConnStatusChange}
      />,
    )
    await waitFor(() => {
      expect(onConnStatusChange).toHaveBeenCalledWith("connecting")
    })
  })

  it("empty state has no axe violations", async () => {
    const { container } = render(<AgentChat projectId="1" sessionId={null} />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})

describe("detectFixLoopStart", () => {
  const base = { id: "1", role: "assistant" as const, content: "", timestamp: 0 }

  it("returns null when the new tool is not bash", () => {
    const messages = [{ ...base, tools: [] }]
    expect(detectFixLoopStart(messages, "read_file")).toBeNull()
  })

  it("returns null when there's no previous bash in the message", () => {
    const messages = [
      { ...base, tools: [{ name: "read_file", input: {}, isError: false }] },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBeNull()
  })

  it("returns null when the previous bash succeeded", () => {
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: false },
          { name: "edit_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBeNull()
  })

  it("returns null when there's an error bash but no edit between", () => {
    const messages = [
      { ...base, tools: [{ name: "bash", input: {}, isError: true }] },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBeNull()
  })

  it("returns insert_banner for bash-error -> edit -> new-bash", () => {
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: true },
          { name: "edit_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBe("insert_banner")
  })

  it("returns insert_banner for bash-error -> write_file -> new-bash", () => {
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: true },
          { name: "write_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBe("insert_banner")
  })

  it("walks back through multiple writes to find the bash", () => {
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: true },
          { name: "edit_file", input: {}, isError: false },
          { name: "edit_file", input: {}, isError: false },
          { name: "write_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBe("insert_banner")
  })

  it("stops at the most recent bash, not earlier ones", () => {
    // Sequence: bash-error edit bash-success edit NEW bash
    // The most recent bash (bash-success) is NOT an error, so no banner.
    const messages = [
      {
        ...base,
        tools: [
          { name: "bash", input: {}, isError: true },
          { name: "edit_file", input: {}, isError: false },
          { name: "bash", input: {}, isError: false },
          { name: "edit_file", input: {}, isError: false },
        ],
      },
    ]
    expect(detectFixLoopStart(messages, "bash")).toBeNull()
  })
})
