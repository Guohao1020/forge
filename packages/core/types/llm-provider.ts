// Forge prometheus (N2): LLM provider catalog (Nacos config center). Shape only —
// auth_key names the secret; the value lives in each agent's env. protocol/
// lifecycle are string (not narrow unions) so backend enum drift downgrades.
export interface ProviderModel { id: string; label?: string; default?: boolean }
export interface ProviderShape {
  name: string;
  namespace?: string;
  version: string;
  protocol: string;      // "anthropic" | "codex-router"
  base_url: string;
  auth_key: string;      // KEY name, never the value
  wire_api?: string;
  models?: ProviderModel[];
  lifecycle: string;     // "published" | "offline" | "draft"
}
export interface ProviderRef { namespace: string; name: string; ref: string }
export interface ProviderList { providers: ProviderShape[] }
