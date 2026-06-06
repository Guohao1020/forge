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
import type { MCPRef, MCPServerShape } from "@multica/core/types";

const mockListMCPServers = vi.hoisted(() => vi.fn());

vi.mock("@multica/core/api", () => ({
  api: {
    listMCPServers: (...args: unknown[]) => mockListMCPServers(...args),
  },
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "ws-1",
}));

import { McpRefPicker, missingSecrets } from "./mcp-picker";

const VOC: MCPServerShape = {
  name: "voc",
  namespace: "ws-1",
  version: "1.0.0",
  transport: "stdio",
  command: "voc-mcp",
  env_keys: ["VOC_API_KEY"],
  lifecycle: "published",
};

const OFFLINE: MCPServerShape = {
  name: "old",
  namespace: "ws-1",
  version: "0.1.0",
  transport: "stdio",
  command: "old-mcp",
  lifecycle: "offline",
};

function renderPicker(props: {
  value: MCPRef[];
  agentEnvKeys: string[];
  onChange: (refs: MCPRef[]) => void;
}) {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
  return render(
    <QueryClientProvider client={qc}>
      <McpRefPicker {...props} />
    </QueryClientProvider>,
  );
}

describe("missingSecrets", () => {
  it("returns env_keys + header_keys not present in agent env", () => {
    const server: MCPServerShape = {
      name: "x",
      version: "1",
      transport: "http",
      env_keys: ["A_KEY"],
      header_keys: ["Authorization"],
      lifecycle: "published",
    };
    expect(missingSecrets(server, ["A_KEY"])).toEqual(["Authorization"]);
    expect(missingSecrets(server, ["A_KEY", "Authorization"])).toEqual([]);
  });

  it("returns [] when the server needs no secrets", () => {
    const server: MCPServerShape = {
      name: "x",
      version: "1",
      transport: "stdio",
      command: "x",
      lifecycle: "published",
    };
    expect(missingSecrets(server, [])).toEqual([]);
  });
});

describe("McpRefPicker", () => {
  beforeEach(() => {
    mockListMCPServers.mockReset();
  });
  afterEach(() => cleanup());

  it("lists only published servers and toggling adds a correct MCPRef", async () => {
    mockListMCPServers.mockResolvedValue({ servers: [VOC, OFFLINE] });
    const onChange = vi.fn();
    renderPicker({ value: [], agentEnvKeys: [], onChange });

    await waitFor(() => expect(screen.getByText("voc")).toBeTruthy());
    // offline server is filtered out
    expect(screen.queryByText("old")).toBeNull();

    fireEvent.click(screen.getByLabelText("select voc"));
    expect(onChange).toHaveBeenCalledWith([
      { namespace: "ws-1", name: "voc", ref: "stable" },
    ]);
  });

  it("warns about missing secrets only for a selected server", async () => {
    mockListMCPServers.mockResolvedValue({ servers: [VOC] });
    const { rerender } = renderPicker({
      value: [{ namespace: "ws-1", name: "voc", ref: "stable" }],
      agentEnvKeys: [],
      onChange: vi.fn(),
    });

    await waitFor(() => expect(screen.getByText("voc")).toBeTruthy());
    expect(screen.getByText(/VOC_API_KEY/)).toBeTruthy();

    // Once the agent has the key, the warning disappears.
    const qc = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });
    rerender(
      <QueryClientProvider client={qc}>
        <McpRefPicker
          value={[{ namespace: "ws-1", name: "voc", ref: "stable" }]}
          agentEnvKeys={["VOC_API_KEY"]}
          onChange={vi.fn()}
        />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(screen.getByText("voc")).toBeTruthy());
    expect(screen.queryByText(/Needs agent env/)).toBeNull();
  });
});
