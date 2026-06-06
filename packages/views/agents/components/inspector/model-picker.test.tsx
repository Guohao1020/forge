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
import type { ProviderShape, RuntimeModel } from "@multica/core/types";
import { I18nProvider } from "@multica/core/i18n/react";
import enCommon from "../../../locales/en/common.json";
import enAgents from "../../../locales/en/agents.json";
import enIssues from "../../../locales/en/issues.json";

const TEST_RESOURCES = {
  en: { common: enCommon, agents: enAgents, issues: enIssues },
};

const mockInitiateListModels = vi.hoisted(() => vi.fn());
const mockGetListModelsResult = vi.hoisted(() => vi.fn());
const mockGetProvider = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: {
    initiateListModels: (...args: unknown[]) => mockInitiateListModels(...args),
    getListModelsResult: (...args: unknown[]) => mockGetListModelsResult(...args),
    getProvider: (...args: unknown[]) => mockGetProvider(...args),
  },
}));

import { ModelPicker } from "./model-picker";

const RUNTIME_MODEL: RuntimeModel = {
  id: "runtime-model-1",
  label: "Runtime Model 1",
  default: true,
};

function runtimeListResult(models: RuntimeModel[]) {
  return {
    id: "req-1",
    runtime_id: "runtime-1",
    status: "completed" as const,
    models,
    supported: true,
    created_at: "2026-06-06T00:00:00Z",
    updated_at: "2026-06-06T00:00:00Z",
  };
}

const PROVIDER: ProviderShape = {
  name: "anthropic",
  namespace: "ws-1",
  version: "1.0.0",
  protocol: "anthropic",
  base_url: "https://api.example.test",
  auth_key: "ANTHROPIC_API_KEY",
  lifecycle: "published",
  models: [
    { id: "claude-provider-model", label: "Claude Provider Model" },
    { id: "bare-id-only" },
  ],
};

function renderPicker(
  props: Partial<React.ComponentProps<typeof ModelPicker>> = {},
) {
  const onChange = vi.fn();
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  const utils = render(
    <I18nProvider locale="en" resources={TEST_RESOURCES}>
      <QueryClientProvider client={queryClient}>
        <ModelPicker
          runtimeId="runtime-1"
          runtimeOnline
          value=""
          canEdit
          onChange={onChange}
          {...props}
        />
      </QueryClientProvider>
    </I18nProvider>,
  );
  return { ...utils, onChange };
}

describe("ModelPicker — provider convergence (prometheus N2)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockInitiateListModels.mockResolvedValue(runtimeListResult([RUNTIME_MODEL]));
    mockGetListModelsResult.mockResolvedValue(runtimeListResult([RUNTIME_MODEL]));
    mockGetProvider.mockResolvedValue(PROVIDER);
  });

  afterEach(() => cleanup());

  it("sources options from the provider catalog when provider_ref is set", async () => {
    renderPicker({
      providerRef: { namespace: "ws-1", name: "anthropic", ref: "stable" },
    });

    // Open the picker popover (single chip-style trigger button).
    fireEvent.click(screen.getByRole("button"));

    // Provider models appear; runtime-discovered models do not.
    await waitFor(() =>
      expect(screen.getByText("Claude Provider Model")).toBeTruthy(),
    );
    expect(screen.queryByText("Runtime Model 1")).toBeNull();

    // The runtime discovery probe must NOT fire when a provider ref overrides.
    expect(mockGetProvider).toHaveBeenCalledWith("anthropic", "ws-1", "stable");
    expect(mockInitiateListModels).not.toHaveBeenCalled();
  });

  it("selecting a provider model saves its id through onChange", async () => {
    const { onChange } = renderPicker({
      providerRef: { namespace: "ws-1", name: "anthropic", ref: "stable" },
    });

    fireEvent.click(screen.getByRole("button"));
    const item = await screen.findByText("Claude Provider Model");
    fireEvent.click(item);

    expect(onChange).toHaveBeenCalledWith("claude-provider-model");
  });

  it("keeps the runtime-discovery path when provider_ref is null", async () => {
    renderPicker({ providerRef: null });

    fireEvent.click(screen.getByRole("button"));

    await waitFor(() => expect(mockInitiateListModels).toHaveBeenCalled());
    expect(await screen.findByText("Runtime Model 1")).toBeTruthy();
    // Provider catalog is never queried for an unbound agent.
    expect(mockGetProvider).not.toHaveBeenCalled();
  });
});
