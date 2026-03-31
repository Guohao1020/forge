"use client"

import { Component, type ReactNode } from "react"

interface Props {
  children: ReactNode
  fallback?: ReactNode
}

interface State {
  hasError: boolean
  error: Error | null
}

export class ErrorBoundary extends Component<Props, State> {
  constructor(props: Props) {
    super(props)
    this.state = { hasError: false, error: null }
  }

  static getDerivedStateFromError(error: Error): State {
    return { hasError: true, error }
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback ?? (
        <div className="flex min-h-screen items-center justify-center bg-background">
          <div className="text-center space-y-4">
            <h2 className="text-xl font-semibold text-foreground">出了点问题</h2>
            <p className="text-muted-foreground text-sm">
              {this.state.error?.message || "发生了未知错误"}
            </p>
            <button
              onClick={() => this.setState({ hasError: false, error: null })}
              className="px-4 py-2 bg-primary text-primary-foreground rounded-md text-sm"
            >
              重试
            </button>
          </div>
        </div>
      )
    }
    return this.props.children
  }
}
