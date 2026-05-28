// API client for the semrel-registry Go API

const API_BASE = '/api/v1';

function getToken(): string {
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
    window.location.href = '/login';
    throw new Error('Unauthorized');
  }

  if (!resp.ok) {
    const body = await resp.json().catch(() => ({})) as { message?: string };
    throw new Error(body.message ?? `HTTP ${resp.status}`);
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
  name: string;
  description: string;
  author: string;
  category: string;
  repository: string;
  license: string;
  tags: string[];
  versions?: PluginVersion[];
  latestVersion?: string;
  downloads: number;
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
}): Promise<PluginListResponse> {
  const qs = new URLSearchParams();
  if (params?.page) qs.set('page', String(params.page));
  if (params?.limit) qs.set('limit', String(params.limit));
  if (params?.category) qs.set('category', params.category);
  if (params?.search) qs.set('search', params.search);
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
