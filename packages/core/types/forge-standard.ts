// Forge F1: spec-center Standards. A categorized, profile-tagged coding
// convention with a mandatory core part (injected into agent instructions)
// and a detailed part (compiled into an on-demand forge-standards skill).

export interface ForgeStandard {
  id: string;
  workspace_id: string;
  project_id?: string; // absent = workspace-level
  name: string;
  category: string;
  profile_tags: string[];
  core_content: string;
  detail_content: string;
  enabled: boolean;
}

export interface ForgeStandardInput {
  project_id?: string;
  name: string;
  category: string;
  profile_tags: string[];
  core_content: string;
  detail_content: string;
  enabled?: boolean;
}
