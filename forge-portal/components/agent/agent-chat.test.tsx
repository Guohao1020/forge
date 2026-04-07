import { describe, it, expect, vi, beforeEach, afterEach } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"
import { axe } from "vitest-axe"
import { AgentChat } from "./agent-chat"

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
    // Stub fetch for the POST /chat call.
    globalThis.fetch = vi.fn(async () =>
      new Response(JSON.stringify({ session_id: "sess-123" }), {
        status: 202,
        headers: { "Content-Type": "application/json" },
      }),
    ) as typeof fetch
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
      .filter((b) => b.textContent?.includes("→ Try:"))
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
    const mockFetch = vi.fn(async () =>
      new Response(JSON.stringify({ session_id: "s1" }), { status: 202 }),
    ) as typeof fetch
    globalThis.fetch = mockFetch

    render(<AgentChat projectId="42" sessionId={null} />)
    const textarea = screen.getByLabelText("Chat with Forge Agent")
    fireEvent.change(textarea, { target: { value: "hello" } })
    const send = screen.getByRole("button", { name: /send message/i })
    fireEvent.click(send)

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalled()
      const url = (mockFetch as unknown as { mock: { calls: [string][] } }).mock
        .calls[0][0]
      expect(url).toContain("/api/projects/42/agent/chat")
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
