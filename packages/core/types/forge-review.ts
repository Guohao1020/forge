// Forge F3: AI review config — which agent reviews for a scope.

export interface ForgeReviewConfig {
  project_id?: string; // absent = workspace-level
  reviewer_agent_id: string;
  enabled: boolean;
}
