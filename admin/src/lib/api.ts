// API client for the semrel-registry Go API

const API_BASE = '/api/v1';

/**
 * Key for the development-only static admin token.
 *
 * GitHub sign-in no longer produces a token the browser can see: the session
 * lives in an HttpOnly cookie the API sets. This remains only for the
 * ADMIN_TOKEN login form, which is the fallback when OAuth is not configured
 * locally.
 */
const DEV_TOKEN_KEY = 'admin_token';

export function getToken(): string {
  try {
    return localStorage.getItem(DEV_TOKEN_KEY) ?? '';
  } catch {
    return '';
  }
}

/** Raised when the session is gone, so callers can prompt instead of failing. */
export class SessionExpiredError extends Error {
  constructor() {
    super('Your session has expired. Please sign in again.');
    this.name = 'SessionExpiredError';
  }
}

/** Raised when the server asks for a fresh interactive sign-in. */
export class ReauthRequiredError extends Error {
  readonly signInURL: string;
  constructor(signInURL: string) {
    super('Please sign in with GitHub again to confirm this action.');
    this.name = 'ReauthRequiredError';
    this.signInURL = signInURL;
  }
}

type SessionListener = () => void;
const sessionListeners = new Set<SessionListener>();

/**
 * Subscribes to session loss.
 *
 * The client used to respond to a 401 with `location.href = '/login'`, which
 * reloads the app and discards whatever the user had typed, with no
 * explanation. Instead the app is notified and can say what happened and
 * return the user to the page they were on.
 */
export function onSessionExpired(listener: SessionListener): () => void {
  sessionListeners.add(listener);
  return () => sessionListeners.delete(listener);
}

function notifySessionExpired() {
  try {
    localStorage.removeItem(DEV_TOKEN_KEY);
  } catch {
    // Nothing to clean up if storage is unavailable.
  }
  sessionListeners.forEach(listener => listener());
}

interface ApiErrorBody {
  message?: string;
  error?: { message?: string; code?: string; details?: { signInURL?: string } };
}

async function request<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  };

  // Only the development token still travels in a header; the GitHub session
  // rides along as a cookie, which is why credentials must be included.
  const token = getToken();
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const resp = await fetch(`${API_BASE}${path}`, { ...options, headers, credentials: 'include' });

  if (!resp.ok) {
    const body = await resp.json().catch(() => ({})) as ApiErrorBody;

    if (resp.status === 401) {
      if (body.error?.code === 'REAUTH_REQUIRED') {
        throw new ReauthRequiredError(body.error.details?.signInURL ?? '/auth/github');
      }
      notifySessionExpired();
      throw new SessionExpiredError();
    }

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
  views: number;
  downloads: number;
  createdAt: string;
  /** Set when the version is retracted; it stays resolvable for pinned installs. */
  yankedAt?: string;
  yankedBy?: string;
  yankedReason?: string;
}

export interface Plugin {
  id: number;
  namespace?: string; // e.g. "@semrel"
  name: string;
  aliases?: string[];
  description: string;
  author: string;
  category: string;
  repository: string;
  license: string;
  status: string; // "active" | "pending" | "rejected"
  /** Why a submission was rejected. Shown to the author on their plugin list. */
  rejectionReason?: string;
  reviewedAt?: string;
  reviewedBy?: string;
  tags: string[];
  versions?: PluginVersion[];
  latestVersion?: string;
  /** The semrel core range the latest version declares compatibility with. */
  latestSemrelCore?: string;
  views: number;
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
  totalUsers?: number;
  categories: Record<string, number>;
  statusCounts?: Record<string, number>;
  totalViews: number;
  totalDownloads: number;
  topPlugins?: Array<{
    pluginId: number;
    namespace?: string;
    name: string;
    category: string;
    views: number;
    downloads: number;
  }>;
  topVersions?: Array<{
    versionId: number;
    pluginId: number;
    namespace?: string;
    pluginName: string;
    version: string;
    views: number;
    downloads: number;
  }>;
  series?: Record<string, Array<{
    period: string;
    views: number;
    downloads: number;
  }>>;
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
  sort?: 'name' | 'category' | 'created_at' | 'updated_at' | 'downloads' | 'views';
  order?: 'asc' | 'desc';
}): Promise<PluginListResponse> {
  const qs = new URLSearchParams();
  if (params?.page)     qs.set('page',     String(params.page));
  if (params?.limit)    qs.set('limit',    String(params.limit));
  if (params?.category) qs.set('category', params.category);
  if (params?.search)   qs.set('search',   params.search);
  if (params?.author)   qs.set('author',   params.author);
  if (params?.status)   qs.set('status',   params.status);
  if (params?.sort)     qs.set('sort',     params.sort);
  if (params?.order)    qs.set('order',    params.order);
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

export async function deletePlugin(
  id: string | number,
  data: { confirmation: string; deleteVersions: boolean; reason?: string },
): Promise<void> {
  return request<void>(`/plugins/${id}`, { method: 'DELETE', body: JSON.stringify(data) });
}

export async function deleteVersion(
  pluginId: string | number,
  versionId: number,
  data: { confirmation: string; reason?: string },
): Promise<void> {
  return request<void>(`/plugins/${pluginId}/versions/${versionId}`, {
    method: 'DELETE',
    body: JSON.stringify(data),
  });
}

/**
 * Deletes the signed-in account.
 *
 * `reauthToken` is only for API clients that authenticate with a bearer token.
 * Browsers prove freshness by having signed in with GitHub recently; the server
 * answers with ReauthRequiredError when they have not.
 */
export async function deleteAccount(data: {
  confirmation: string;
  deleteOwnedPlugins: boolean;
  reason?: string;
  reauthToken?: string;
}): Promise<{ data: { pluginsDeleted: number; versionsDeleted: number } }> {
  return request('/auth/me', { method: 'DELETE', body: JSON.stringify(data) });
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
  const resp = await fetch(`${API_BASE}/admin/status`, { headers, credentials: 'include' });
  return resp.ok;
}

export function saveToken(token: string): void {
  try {
    localStorage.setItem(DEV_TOKEN_KEY, token);
  } catch {
    // Non-fatal: the caller still holds a working session for this page load.
  }
}

export function clearToken(): void {
  try {
    localStorage.removeItem(DEV_TOKEN_KEY);
  } catch {
    // Nothing to clean up if storage is unavailable.
  }
}

// ---- Session ----

export interface SessionUser {
  login: string;
  name: string;
  avatarUrl: string;
  role: string;    // "admin" | "user"
  isAdmin: boolean;
}

interface RawSessionUser {
  login?: string;
  name?: string;
  avatar_url?: string;
  avatarUrl?: string;
  role?: string;
  is_admin?: boolean;
  isAdmin?: boolean;
}

/**
 * Returns the signed-in user, or null when there is no session.
 *
 * The identity comes from the API rather than from decoding a JWT in the
 * browser: with an HttpOnly cookie there is no token to decode, and a token the
 * page can read is a token an injected script can steal.
 */
export async function getCurrentUser(): Promise<SessionUser | null> {
  const headers: Record<string, string> = {};
  const token = getToken();
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const resp = await fetch(`${API_BASE}/auth/me`, { headers, credentials: 'include' });
  if (!resp.ok) {
    // A stale dev token takes precedence over a perfectly valid GitHub session
    // cookie (the API checks the Authorization header first) and, unlike
    // request()'s shared 401 handling, this call never clears it -- so a
    // leftover token from the ADMIN_TOKEN login form permanently locks the
    // user out of their real session. Only ever clear the token itself, never
    // the caller's session cookie.
    if (token) clearToken();
    return null;
  }

  const body = await resp.json().catch(() => null) as { user?: RawSessionUser } | null;
  const user = body?.user;
  if (!user?.login) return null;

  const isAdmin = user.is_admin === true || user.isAdmin === true || user.role === 'admin';
  return {
    login: user.login,
    name: user.name ?? '',
    avatarUrl: user.avatar_url ?? user.avatarUrl ?? '',
    role: user.role ?? (isAdmin ? 'admin' : 'user'),
    isAdmin,
  };
}

/** Ends the session server-side, then clears any local development token. */
export async function signOut(): Promise<void> {
  try {
    await request<void>('/auth/logout', { method: 'POST' });
  } catch {
    // An already-invalid session is still a successful sign-out.
  }
  clearToken();
}

// ---- Auth config ----

export interface AuthConfig {
  githubOAuthEnabled: boolean;
  loginURL: string;
}

export async function getAuthConfig(): Promise<AuthConfig> {
  const resp = await fetch('/auth/config');
  if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
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
  return request<ValidationResult>('/plugins/validate', {
    method: 'POST',
    body: JSON.stringify({ repository }),
  });
}


// ---- Community plugin submission ----

/**
 * Submits a community plugin for review.
 *
 * `notifyEmail` is optional and is stored apart from the plugin record: the
 * registry returns it from no endpoint and erases it when the account is
 * deleted.
 */
export async function submitPlugin(
  plugin: Partial<Plugin>,
  notifyEmail?: string,
): Promise<Plugin> {
  return request<{ data: Plugin }>('/plugins/submit', {
    method: 'POST',
    body: JSON.stringify({ ...plugin, notifyEmail: notifyEmail?.trim() || undefined }),
  }).then(r => r.data);
}

// ---- Admin: approve/reject submissions ----

export async function approvePlugin(id: number | string, reason?: string): Promise<Plugin> {
  return request<{ data: Plugin }>(`/admin/plugins/${id}/approve`, {
    method: 'PUT',
    body: JSON.stringify({ reason: reason ?? '' }),
  }).then(r => r.data);
}

/**
 * Rejects a submission. The reason is required by the API and shown to the
 * author — a "rejected" badge on its own gives them nothing to act on.
 */
export async function rejectPlugin(id: number | string, reason: string): Promise<Plugin> {
  return request<{ data: Plugin }>(`/admin/plugins/${id}/reject`, {
    method: 'PUT',
    body: JSON.stringify({ reason }),
  }).then(r => r.data);
}

// ---- Yank ----

/**
 * Retracts a published version.
 *
 * Unlike deleting it, the version stays resolvable, so builds that already pin
 * it keep working — they simply stop being offered it as an update.
 */
export async function yankVersion(
  pluginId: string | number,
  versionId: number,
  reason: string,
): Promise<PluginVersion> {
  return request<{ data: PluginVersion }>(`/plugins/${pluginId}/versions/${versionId}/yank`, {
    method: 'PUT',
    body: JSON.stringify({ reason }),
  }).then(r => r.data);
}

export async function unyankVersion(
  pluginId: string | number,
  versionId: number,
): Promise<PluginVersion> {
  return request<{ data: PluginVersion }>(`/plugins/${pluginId}/versions/${versionId}/yank`, {
    method: 'DELETE',
  }).then(r => r.data);
}

export async function revalidatePlugin(id: number | string): Promise<ValidationResult> {
  return request<{ data: ValidationResult }>(`/admin/plugins/${id}/revalidate`, { method: 'POST' }).then(r => r.data);
}

export interface BatchRevalidationResult {
  id: number;
  name: string;
  repository: string;
  result?: ValidationResult;
  error?: string;
}

export interface BatchRevalidationResponse {
  data: BatchRevalidationResult[];
  summary: {
    total: number;
    processed: number;
    succeeded: number;
    failed: number;
  };
}

export async function revalidateAllPlugins(): Promise<BatchRevalidationResponse> {
  return request<BatchRevalidationResponse>('/admin/plugins/revalidate-all', { method: 'POST' });
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

// ---- Plugin README ----

export interface PluginReadme {
  /** Untrusted author markdown — render it through the Markdown component. */
  markdown: string;
  /** Canonical URL of the README on GitHub. */
  source: string;
}

/**
 * Fetches a plugin's README through the registry.
 *
 * The registry proxies it rather than the browser calling GitHub directly: the
 * browser has no API token and would hit the unauthenticated rate limit, and a
 * direct fetch would disclose every visitor's address to GitHub.
 */
export async function getPluginReadme(id: string | number): Promise<PluginReadme | null> {
  try {
    const { data } = await request<{ data: PluginReadme }>(`/plugins/${id}/readme`);
    return data;
  } catch {
    // A missing README is an ordinary state, not an error worth surfacing.
    return null;
  }
}

// ---- Repository ownership ----

export interface OwnershipResult {
  verified: boolean;
  /** How the claim was settled: account-owner, public-org-member, claim-file. */
  method?: string;
  issue?: string;
  howToFix?: string;
}

/**
 * Checks whether the signed-in user can show they control a repository.
 *
 * Called before the rest of the submission form is filled in, so a failed claim
 * surfaces while the submitter can still act on it rather than after they have
 * typed everything.
 */
export async function verifyRepositoryOwnership(repository: string): Promise<OwnershipResult> {
  const { data } = await request<{ data: OwnershipResult }>('/plugins/verify-ownership', {
    method: 'POST',
    body: JSON.stringify({ repository }),
  });
  return data;
}
