import { render, screen, waitFor, fireEvent } from "@testing-library/react"
import { describe, expect, it, vi } from "vitest"

import { ClarificationInput } from "./clarification-input"

describe("ClarificationInput", () => {
  const baseProps = {
    question: "Which database should I use -- Postgres or MySQL?",
    toolUseId: "toolu_abc123",
  }

  it("renders the question", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    expect(
      screen.getByText(/Postgres or MySQL/),
    ).toBeInTheDocument()
  })

  it("disables submit when response is empty", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    const btn = screen.getByRole("button", { name: /submit response/i })
    expect(btn).toBeDisabled()
  })

  it("enables submit when response is non-empty", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "Postgres" } })
    const btn = screen.getByRole("button", { name: /submit response/i })
    expect(btn).not.toBeDisabled()
  })

  it("calls onSubmit with toolUseId and response on click", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<ClarificationInput {...baseProps} onSubmit={onSubmit} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "Postgres please" } })
    const btn = screen.getByRole("button", { name: /submit response/i })
    fireEvent.click(btn)
    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith("toolu_abc123", "Postgres please")
    })
  })

  it("shows submitting state during async call", async () => {
    let resolveSubmit!: () => void
    const onSubmit = vi.fn().mockImplementation(
      () =>
        new Promise<void>((resolve) => {
          resolveSubmit = resolve
        }),
    )
    render(<ClarificationInput {...baseProps} onSubmit={onSubmit} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "test" } })
    fireEvent.click(screen.getByRole("button", { name: /submit response/i }))
    await waitFor(() => {
      expect(screen.getByText("Submitting...")).toBeInTheDocument()
    })
    resolveSubmit()
  })

  it("shows submitted state on success", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    render(<ClarificationInput {...baseProps} onSubmit={onSubmit} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "go" } })
    fireEvent.click(screen.getByRole("button", { name: /submit response/i }))
    await waitFor(() => {
      expect(screen.getByText("Submitted")).toBeInTheDocument()
    })
    expect(screen.getByText(/waiting for agent/i)).toBeInTheDocument()
  })

  it("shows error state on failure and allows retry", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("HTTP 500: boom"))
    render(<ClarificationInput {...baseProps} onSubmit={onSubmit} />)
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "x" } })
    fireEvent.click(screen.getByRole("button", { name: /submit response/i }))
    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(/HTTP 500/)
    })
    // After error the button returns to 'Submit Response' and is re-enabled
    expect(
      screen.getByRole("button", { name: /submit response/i }),
    ).not.toBeDisabled()
  })

  it("respects maxLength of 4096", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    const textarea = screen.getByLabelText(
      /clarification response textarea/i,
    ) as HTMLTextAreaElement
    expect(textarea.maxLength).toBe(4096)
  })

  it("disables the form when disabled prop is true", () => {
    render(
      <ClarificationInput {...baseProps} onSubmit={vi.fn()} disabled />,
    )
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    expect(textarea).toBeDisabled()
    const btn = screen.getByRole("button", { name: /submit response/i })
    expect(btn).toBeDisabled()
  })

  it("shows character counter", () => {
    render(<ClarificationInput {...baseProps} onSubmit={vi.fn()} />)
    expect(screen.getByText(/0\/4096 characters/)).toBeInTheDocument()
    const textarea = screen.getByLabelText(/clarification response textarea/i)
    fireEvent.change(textarea, { target: { value: "hello" } })
    expect(screen.getByText(/5\/4096 characters/)).toBeInTheDocument()
  })
})
