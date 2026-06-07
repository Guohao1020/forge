// @vitest-environment jsdom

import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ProviderRef, ProviderShape } from "@multica/core/types";

const mockListProviders = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: {
    listProviders: (...args: unknown[]) => mockListProviders(...args),
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

import { ProviderPicker, providerMissingSecret } from "./provider-picker";

// name differs from protocol so the name <span> and the protocol <Badge>
// don't collide on a getByText("…") lookup.
const ANTHROPIC: ProviderShape = {
  name: "prod-claude",
  namespace: "ws-1",
  version: "1.0.0",
  protocol: "anthropic",
  base_url: "https://api.example.test",
  auth_key: "ANTHROPIC_API_KEY",
  lifecycle: "published",
};

const DRAFT: ProviderShape = {
  name: "draft-router",
  namespace: "ws-1",
  version: "0.1.0",
  protocol: "codex-router",
  base_url: "https://router.example.test",
  auth_key: "ROUTER_API_KEY",
  lifecycle: "draft",
};

function renderPicker(props: {
  value: ProviderRef | null;
  agentEnvKeys: string[];
  onChange: (ref: ProviderRef | null) => void;
}) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <ProviderPicker {...props} />
    </QueryClientProvider>,
  );
}

describe("providerMissingSecret", () => {
  it("returns the auth_key when it is absent from the agent env", () => {
    expect(providerMissingSecret(ANTHROPIC, [])).toBe("ANTHROPIC_API_KEY");
    expect(providerMissingSecret(ANTHROPIC, ["OTHER_KEY"])).toBe(
      "ANTHROPIC_API_KEY",
    );
  });

  it("returns null when the agent already supplies the auth_key", () => {
    expect(providerMissingSecret(ANTHROPIC, ["ANTHROPIC_API_KEY"])).toBeNull();
  });

  it("returns null when the provider has no auth_key", () => {
    const noKey: ProviderShape = { ...ANTHROPIC, auth_key: "" };
    expect(providerMissingSecret(noKey, [])).toBeNull();
  });
});

describe("ProviderPicker", () => {
  beforeEach(() => {
    mockListProviders.mockReset();
  });
  afterEach(() => cleanup());

  it("lists only published providers and selecting one calls onChange with a ProviderRef", async () => {
    mockListProviders.mockResolvedValue({ providers: [ANTHROPIC, DRAFT] });
    const onChange = vi.fn();
    renderPicker({ value: null, agentEnvKeys: [], onChange });

    await waitFor(() =>
      expect(screen.getByLabelText("select prod-claude")).toBeTruthy(),
    );
    // draft provider is filtered out
    expect(screen.queryByLabelText("select draft-router")).toBeNull();

    fireEvent.click(screen.getByLabelText("select prod-claude"));
    expect(onChange).toHaveBeenCalledWith({
      namespace: "ws-1",
      name: "prod-claude",
      ref: "stable",
    });
  });

  it("selecting None clears the ref", async () => {
    mockListProviders.mockResolvedValue({ providers: [ANTHROPIC] });
    const onChange = vi.fn();
    renderPicker({
      value: { namespace: "ws-1", name: "prod-claude", ref: "stable" },
      agentEnvKeys: [],
      onChange,
    });

    await waitFor(() =>
      expect(screen.getByLabelText("select prod-claude")).toBeTruthy(),
    );
    fireEvent.click(screen.getByLabelText("select none"));
    expect(onChange).toHaveBeenCalledWith(null);
  });

  it("warns about the missing secret only for the selected provider", async () => {
    mockListProviders.mockResolvedValue({ providers: [ANTHROPIC] });
    const { rerender } = renderPicker({
      value: { namespace: "ws-1", name: "prod-claude", ref: "stable" },
      agentEnvKeys: [],
      onChange: vi.fn(),
    });

    await waitFor(() =>
      expect(screen.getByLabelText("select prod-claude")).toBeTruthy(),
    );
    expect(screen.getByText(/ANTHROPIC_API_KEY/)).toBeTruthy();

    // Once the agent supplies the key, the warning disappears.
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });
    rerender(
      <QueryClientProvider client={qc}>
        <ProviderPicker
          value={{ namespace: "ws-1", name: "prod-claude", ref: "stable" }}
          agentEnvKeys={["ANTHROPIC_API_KEY"]}
          onChange={vi.fn()}
        />
      </QueryClientProvider>,
    );
    await waitFor(() =>
      expect(screen.getByLabelText("select prod-claude")).toBeTruthy(),
    );
    expect(screen.queryByText(/Needs agent env/)).toBeNull();
  });

  it("does not warn for an unselected provider even when its secret is missing", async () => {
    mockListProviders.mockResolvedValue({ providers: [ANTHROPIC] });
    renderPicker({ value: null, agentEnvKeys: [], onChange: vi.fn() });

    await waitFor(() =>
      expect(screen.getByLabelText("select prod-claude")).toBeTruthy(),
    );
    // Nothing selected → no missing-secret warning.
    expect(screen.queryByText(/Needs agent env/)).toBeNull();
  });
});
