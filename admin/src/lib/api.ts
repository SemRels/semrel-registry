// API client for the semrel-registry Go API

const API_BASE = '/api/v1';

export function getToken(): string {
  return localStorage.getItem('admin_token') ?? '';
}

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };

  const token = getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const resp = await fetch(`${API_BASE}${path}`, { ...options, headers });

  if (resp.status === 401) {
    localStorage.removeItem('admin_token');
    globalThis.location.href = '/login';
    throw new Error('Unauthorized');
  }

  if (!resp.ok) {
    const body = await resp.json().catch(() => ({})) as {
      message?: string;
      error?: { message?: string };
    };
    const message = body.message ?? body.error?.message;
    throw new Error(message ?? `HTTP ${resp.status}`);
  }

  if (resp.status === 204) return undefined as T;
  return resp.json() as Promise<T>;
}

export interface PluginVersion {
  id: number;
  pluginId: number;
  version: string;
  releaseDate?: string;
  changelog: string;
  downloadUrl: string;
  checksums?: Record<string, string>;
  prerelease: boolean;
  createdAt: string;
}

export interface Plugin {
  id: number;
  namespace?: string; // e.g. "@semrel"
  name: string;
  description: string;
  author: string;
  category: string;
  repository: string;
  license: string;
  status: string; // "active" | "pending" | "rejected"
  tags: string[];
  versions?: PluginVersion[];
  latestVersion?: string;
  downloads: number;
  validationChecks?: ValidationResult; // pre-analysis results stored by server
  validatedAt?: string;
  createdAt: string;
  updatedAt: string;
}

export interface Pagination {
  page: number;
  limit: number;
  total: number;
  pages: number;
}

export interface PluginListResponse {
  data: Plugin[];
  pagination: Pagination;
}

export interface Stats {
  totalPlugins: number;
  categories: Record<string, number>;
  timestamp: string;
}

export interface SyncResult {
  created: number;
  updated: number;
  failed: number;
  total: number;
  source?: string;
}

// ---- Plugin CRUD ----

export async function listPlugins(params?: {
  page?: number;
  limit?: number;
  category?: string;
  search?: string;
  author?: string;
  status?: string;
}): Promise<PluginListResponse> {
  const qs = new URLSearchParams();
  if (params?.page)     qs.set('page',     String(params.page));
  if (params?.limit)    qs.set('limit',    String(params.limit));
  if (params?.category) qs.set('category', params.category);
  if (params?.search)   qs.set('search',   params.search);
  if (params?.author)   qs.set('author',   params.author);
  if (params?.status)   qs.set('status',   params.status);
  return request<PluginListResponse>(`/plugins?${qs}`);
}

export async function getPlugin(id: string | number): Promise<{ data: Plugin }> {
  return request<{ data: Plugin }>(`/plugins/${id}`);
}

export async function createPlugin(
  data: Partial<Plugin>,
): Promise<{ data: Plugin }> {
  return request<{ data: Plugin }>('/plugins', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function updatePlugin(
  id: string | number,
  data: Partial<Plugin>,
): Promise<{ data: Plugin }> {
  return request<{ data: Plugin }>(`/plugins/${id}`, {
    method: 'PUT',
    body: JSON.stringify(data),
  });
}

export async function deletePlugin(id: string | number): Promise<void> {
  return request<void>(`/plugins/${id}`, { method: 'DELETE' });
}

export async function listVersions(
  pluginId: string | number,
): Promise<{ data: PluginVersion[] }> {
  return request<{ data: PluginVersion[] }>(`/plugins/${pluginId}/versions`);
}

export async function createVersion(
  pluginId: string | number,
  data: Partial<PluginVersion>,
): Promise<{ data: PluginVersion }> {
  return request<{ data: PluginVersion }>(`/plugins/${pluginId}/versions`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

// ---- Admin ----

export async function getStats(): Promise<Stats> {
  return request<Stats>('/stats');
}

export async function syncFromFile(): Promise<SyncResult> {
  return request<SyncResult>('/admin/sync-file', { method: 'POST' });
}

export async function verifyToken(token: string): Promise<boolean> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Authorization: `Bearer ${token}`,
  };
  const resp = await fetch(`${API_BASE}/admin/status`, { headers });
  return resp.ok;
}

export function saveToken(token: string): void {
  localStorage.setItem('admin_token', token);
}

export function clearToken(): void {
  localStorage.removeItem('admin_token');
}

export function hasToken(): boolean {
  return Boolean(localStorage.getItem('admin_token'));
}

// ---- Auth config ----

export interface AuthConfig {
  githubOAuthEnabled: boolean;
  loginURL: string;
}

export async function getAuthConfig(): Promise<AuthConfig> {
  const resp = await fetch('/auth/config');
  return resp.json() as Promise<AuthConfig>;
}

// ---- Version sync ----

export interface SyncVersionsResult {
  results: Array<{
    plugin: string;
    created: number;
    skipped: number;
    error?: string;
  }>;
}

export async function syncVersions(plugin?: string): Promise<SyncVersionsResult> {
  return request<SyncVersionsResult>('/admin/sync-versions', {
    method: 'POST',
    body: JSON.stringify(plugin ? { plugin } : {}),
  });
}

// ---- Plugin standards validation ----

export interface ValidationCheck {
  id: string;
  label: string;
  passed: boolean;
  message?: string;
}

export interface ValidationResult {
  valid: boolean;
  plugin: string;
  owner: string;
  checks: ValidationCheck[];
  summary: string;
}

export async function validatePlugin(repository: string): Promise<ValidationResult> {
  const resp = await fetch(`${API_BASE}/plugins/validate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ repository }),
  });
  return resp.json() as Promise<ValidationResult>;
}


// ---- Community plugin submission ----

export async function submitPlugin(plugin: Partial<Plugin>): Promise<Plugin> {
  return request<{ data: Plugin }>('/plugins/submit', {
    method: 'POST',
    body: JSON.stringify(plugin),
  }).then(r => r.data);
}

// ---- Admin: approve/reject submissions ----

export async function approvePlugin(id: number | string): Promise<Plugin> {
  return request<{ data: Plugin }>(`/admin/plugins/${id}/approve`, { method: 'PUT' }).then(r => r.data);
}

export async function rejectPlugin(id: number | string): Promise<Plugin> {
  return request<{ data: Plugin }>(`/admin/plugins/${id}/reject`, { method: 'PUT' }).then(r => r.data);
}

export async function revalidatePlugin(id: number | string): Promise<ValidationResult> {
  return request<{ data: ValidationResult }>(`/admin/plugins/${id}/revalidate`, { method: 'POST' }).then(r => r.data);
}

// ---- Admin: sync GitHub org ----

export interface OrgSyncResult {
  org: string;
  total: number;
  results: { repo: string; action: string; versions?: number; error?: string }[];
}

export async function syncGitHubOrg(): Promise<OrgSyncResult> {
  return request<OrgSyncResult>('/admin/sync-github-org', { method: 'POST' });
}
