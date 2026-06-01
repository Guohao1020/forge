// Forge F2: verification checks. A named shell command run in the task workdir
// after the agent session ends; non-zero exit blocks the task.

export interface ForgeCheck {
  id: string;
  workspace_id: string;
  project_id?: string; // absent = workspace-level
  name: string;
  command: string;
  enabled: boolean;
}

export interface ForgeCheckInput {
  project_id?: string;
  name: string;
  command: string;
  enabled?: boolean;
}
