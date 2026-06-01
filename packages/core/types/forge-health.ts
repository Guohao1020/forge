// Forge F5: Harness health observability.

export interface ForgeHealthCategoryCount { category: string; count: number; }
export interface ForgeHealth {
  standards: ForgeHealthCategoryCount[];
  standards_total: number;
  checks: number;
  review_configs: number;
  scans: number;
  gate: { passed: number; failed: number };
  review: { total: number; completed: number; avg_turnaround_sec: number };
  open_findings: number;
  scan_runs: number;
  fix_prs: { opened: number; merged: number; matched: number };
}
export interface ForgeTrendPoint { date: string; passed?: number; failed?: number; count?: number; }
export interface ForgeHealthTrends { findings: ForgeTrendPoint[]; gate: ForgeTrendPoint[]; fix_prs: ForgeTrendPoint[]; }
export interface ForgeIssueRef { issue_id: string; number: number; title: string; }
export interface ForgeFixPRRef { issue_id: string; number: number; title: string; pr_url: string; }
