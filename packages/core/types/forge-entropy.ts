// Forge F4: entropy scan config — periodic whole-repo quality scan for a scope.

export interface ForgeEntropyScan {
  id: string;
  project_id?: string;
  name: string;
  scanner_agent_id: string;
  custom_focus: string;
  include_standards: boolean;
  include_checks: boolean;
  cron_expression: string;
  timezone: string;
  enabled: boolean;
  auto_fix: boolean;
  autopilot_id?: string;
}

export interface ForgeEntropyScanInput {
  project_id?: string;
  name: string;
  scanner_agent_id: string;
  custom_focus: string;
  include_standards: boolean;
  include_checks: boolean;
  cron_expression: string;
  timezone: string;
  enabled: boolean;
  auto_fix: boolean;
}
