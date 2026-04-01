import { api } from "./api";

// ==================== Types ====================

export interface Standard {
  id: number;
  tenantId: number;
  name: string;
  category: string;
  scope: string;
  scopeId: number;
  parentId?: number;
  content: string;
  version: number;
  status: string;
  createdBy?: number;
  createdAt: string;
  updatedAt: string;
}

export interface PromptTemplate {
  id: number;
  tenantId: number;
  name: string;
  purpose: string;
  systemPrompt: string;
  userTemplate: string;
  variables: string[];
  version: number;
  isDefault: boolean;
  createdBy?: number;
  createdAt: string;
  updatedAt: string;
}

export interface ReviewRule {
  id: number;
  tenantId: number;
  name: string;
  category: string;
  scope: string;
  scopeId: number;
  ruleType: string;
  definition: Record<string, unknown>;
  severity: string;
  autoFix: boolean;
  fixTemplate?: string;
  enabled: boolean;
  createdBy?: number;
  createdAt: string;
  updatedAt: string;
}

export interface ScaffoldTemplate {
  id: number;
  tenantId: number;
  name: string;
  projectType: string;
  description?: string;
  templateRepo?: string;
  variables: string[];
  postHooks: string[];
  version: number;
  createdBy?: number;
  createdAt: string;
  updatedAt: string;
}

export interface PageResult<T> {
  items: T[];
  total: number;
  page: number;
  pageSize: number;
}

export interface EffectiveSpecs {
  standards: Standard[];
  rules: ReviewRule[];
}

// ==================== Standards API ====================

export async function listStandards(params?: {
  category?: string;
  scope?: string;
  scopeId?: number;
  page?: number;
  pageSize?: number;
}): Promise<PageResult<Standard>> {
  const searchParams = new URLSearchParams();
  if (params?.category) searchParams.set("category", params.category);
  if (params?.scope) searchParams.set("scope", params.scope);
  if (params?.scopeId !== undefined) searchParams.set("scopeId", String(params.scopeId));
  if (params?.page) searchParams.set("page", String(params.page));
  if (params?.pageSize) searchParams.set("pageSize", String(params.pageSize));
  const query = searchParams.toString();
  return api.get(`/specs/standards${query ? `?${query}` : ""}`);
}

export async function getStandard(id: number): Promise<Standard> {
  return api.get(`/specs/standards/${id}`);
}

export async function createStandard(data: {
  name: string;
  category: string;
  scope: string;
  scopeId: number;
  parentId?: number;
  content: string;
}): Promise<Standard> {
  return api.post("/specs/standards", data);
}

export async function updateStandard(
  id: number,
  data: { name: string; content: string }
): Promise<Standard> {
  return api.put(`/specs/standards/${id}`, data);
}

export async function deleteStandard(id: number): Promise<void> {
  return api.delete(`/specs/standards/${id}`);
}

// ==================== Prompt Templates API ====================

export async function listPromptTemplates(params?: {
  purpose?: string;
  page?: number;
  pageSize?: number;
}): Promise<PageResult<PromptTemplate>> {
  const searchParams = new URLSearchParams();
  if (params?.purpose) searchParams.set("purpose", params.purpose);
  if (params?.page) searchParams.set("page", String(params.page));
  if (params?.pageSize) searchParams.set("pageSize", String(params.pageSize));
  const query = searchParams.toString();
  return api.get(`/specs/prompts${query ? `?${query}` : ""}`);
}

export async function getPromptTemplate(id: number): Promise<PromptTemplate> {
  return api.get(`/specs/prompts/${id}`);
}

export async function createPromptTemplate(data: {
  name: string;
  purpose: string;
  systemPrompt: string;
  userTemplate: string;
  variables: string[];
  isDefault: boolean;
}): Promise<PromptTemplate> {
  return api.post("/specs/prompts", data);
}

export async function updatePromptTemplate(
  id: number,
  data: {
    name: string;
    purpose: string;
    systemPrompt: string;
    userTemplate: string;
    variables: string[];
    isDefault: boolean;
  }
): Promise<PromptTemplate> {
  return api.put(`/specs/prompts/${id}`, data);
}

export async function deletePromptTemplate(id: number): Promise<void> {
  return api.delete(`/specs/prompts/${id}`);
}

// ==================== Review Rules API ====================

export async function listReviewRules(params?: {
  category?: string;
  severity?: string;
  page?: number;
  pageSize?: number;
}): Promise<PageResult<ReviewRule>> {
  const searchParams = new URLSearchParams();
  if (params?.category) searchParams.set("category", params.category);
  if (params?.severity) searchParams.set("severity", params.severity);
  if (params?.page) searchParams.set("page", String(params.page));
  if (params?.pageSize) searchParams.set("pageSize", String(params.pageSize));
  const query = searchParams.toString();
  return api.get(`/specs/rules${query ? `?${query}` : ""}`);
}

export async function getReviewRule(id: number): Promise<ReviewRule> {
  return api.get(`/specs/rules/${id}`);
}

export async function createReviewRule(data: {
  name: string;
  category: string;
  scope: string;
  scopeId: number;
  ruleType: string;
  definition: Record<string, unknown>;
  severity: string;
  autoFix: boolean;
  fixTemplate?: string;
}): Promise<ReviewRule> {
  return api.post("/specs/rules", data);
}

export async function updateReviewRule(
  id: number,
  data: {
    name: string;
    category: string;
    ruleType: string;
    definition: Record<string, unknown>;
    severity: string;
    autoFix: boolean;
    fixTemplate?: string;
  }
): Promise<ReviewRule> {
  return api.put(`/specs/rules/${id}`, data);
}

export async function toggleReviewRule(id: number): Promise<ReviewRule> {
  return api.delete(`/specs/rules/${id}`);
}

// ==================== Scaffold Templates API ====================

export async function listScaffoldTemplates(params?: {
  projectType?: string;
  page?: number;
  pageSize?: number;
}): Promise<PageResult<ScaffoldTemplate>> {
  const searchParams = new URLSearchParams();
  if (params?.projectType) searchParams.set("projectType", params.projectType);
  if (params?.page) searchParams.set("page", String(params.page));
  if (params?.pageSize) searchParams.set("pageSize", String(params.pageSize));
  const query = searchParams.toString();
  return api.get(`/specs/scaffolds${query ? `?${query}` : ""}`);
}

export async function getScaffoldTemplate(id: number): Promise<ScaffoldTemplate> {
  return api.get(`/specs/scaffolds/${id}`);
}

// ==================== Effective Specs API ====================

export async function getEffectiveSpecs(projectId: number): Promise<EffectiveSpecs> {
  return api.get(`/specs/effective/${projectId}`);
}
