import { api } from "./client";
import type {
  AdminLoginResponse,
  AuditLogEntry,
  DashboardStats,
  GeoPoint,
  ParticipantRoute,
  ParticipantType,
  PlatformSettings,
  PermissionSet,
  Tool,
  User,
  UserDetail,
  UserStatus,
  VerificationDetail,
  VerificationQueueItem,
  VerificationStatus,
} from "./types";

export function login(email: string, password: string) {
  return api.post<AdminLoginResponse>("/admin/login", { email, password });
}

export function getDashboardStats() {
  return api.get<DashboardStats>("/admin/dashboard/stats");
}

export function getAuditLog(limit = 10, offset = 0) {
  return api.get<AuditLogEntry[]>(`/admin/audit-log?limit=${limit}&offset=${offset}`);
}

export function getUserRoutes(userId: string) {
  return api.get<ParticipantRoute[]>(`/admin/users/${userId}/routes`);
}

export function addUserRoute(userId: string, origin: GeoPoint, destination: GeoPoint) {
  return api.post<ParticipantRoute>(`/admin/users/${userId}/routes`, { origin, destination });
}

export function deleteUserRoute(userId: string, routeId: string) {
  return api.del<{ status: string }>(`/admin/users/${userId}/routes/${routeId}`);
}

export function getVerificationQueue(status: VerificationStatus = "pending") {
  return api.get<VerificationQueueItem[]>(
    `/admin/verifications?status=${encodeURIComponent(status)}`
  );
}

export function getVerificationDetail(id: string) {
  return api.get<VerificationDetail>(`/admin/verifications/${id}`);
}

export function approveVerification(id: string) {
  return api.post<{ status: string }>(`/admin/verifications/${id}/approve`);
}

export function rejectVerification(id: string, reason: string) {
  return api.post<{ status: string }>(`/admin/verifications/${id}/reject`, { reason });
}

export interface UserListParams {
  type?: ParticipantType | "";
  status?: UserStatus | "";
  search?: string;
}

export function getUsers(params: UserListParams = {}) {
  const query = new URLSearchParams();
  if (params.type) query.set("type", params.type);
  if (params.status) query.set("status", params.status);
  if (params.search) query.set("search", params.search);
  const qs = query.toString();
  return api.get<User[]>(`/admin/users${qs ? `?${qs}` : ""}`);
}

export function getUserDetail(id: string) {
  return api.get<UserDetail>(`/admin/users/${id}`);
}

export function setUserTools(id: string, toolIds: string[]) {
  return api.post<{ status: string }>(`/admin/users/${id}/tools`, { tool_ids: toolIds });
}

export function applyPermissionSet(id: string, setId: string) {
  return api.post<{ status: string }>(`/admin/users/${id}/apply-set`, { set_id: setId });
}

export function blockUser(id: string) {
  return api.post<{ status: string }>(`/admin/users/${id}/block`);
}

export function getPlatformSettings() {
  return api.get<PlatformSettings>("/admin/settings");
}

export function updatePlatformSettings(settings: PlatformSettings) {
  return api.patch<PlatformSettings>("/admin/settings", settings);
}

export function setUserSubscription(id: string, hasSubscription: boolean) {
  return api.post<{ status: string }>(`/admin/users/${id}/subscription`, {
    has_subscription: hasSubscription,
  });
}

export function unblockUser(id: string) {
  return api.post<{ status: string }>(`/admin/users/${id}/unblock`);
}

export function getTools() {
  return api.get<Tool[]>("/admin/tools");
}

export interface CreateToolInput {
  key: string;
  name: string;
  description: string;
  category: string;
}

export function createTool(input: CreateToolInput) {
  return api.post<Tool>("/admin/tools", input);
}

export interface UpdateToolInput {
  name?: string;
  description?: string;
  category?: string;
  is_active?: boolean;
}

export function updateTool(id: string, patch: UpdateToolInput) {
  return api.patch<Tool>(`/admin/tools/${id}`, patch);
}

export function getPermissionSets() {
  return api.get<PermissionSet[]>("/admin/permission-sets");
}

export interface CreatePermissionSetInput {
  name: string;
  description: string;
  tool_ids: string[];
}

export function createPermissionSet(input: CreatePermissionSetInput) {
  return api.post<PermissionSet>("/admin/permission-sets", input);
}

export interface UpdatePermissionSetInput {
  name?: string;
  description?: string;
  tool_ids?: string[];
}

export function updatePermissionSet(id: string, patch: UpdatePermissionSetInput) {
  return api.patch<PermissionSet>(`/admin/permission-sets/${id}`, patch);
}
