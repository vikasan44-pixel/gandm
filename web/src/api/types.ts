export type ParticipantType =
  | "client"
  | "warehouse"
  | "carrier"
  | "driver"
  | "broker"
  | "customs_rep";

export type UserStatus = "pending" | "active" | "blocked" | "rejected";

export type VerificationStatus = "pending" | "approved" | "rejected";

export type DocumentType =
  | "id_card"
  | "founding_docs"
  | "business_license"
  | "employment_contract"
  | "vehicle_doc";

export type AdminRole = "admin" | "moderator";

export interface User {
  id: string;
  email: string;
  phone: string;
  company_name: string;
  participant_type: ParticipantType;
  status: UserStatus;
  has_subscription: boolean;
  language: string;
  created_at: string;
  last_active_at: string | null;
}

export interface VerificationRequest {
  id: string;
  user_id: string;
  status: VerificationStatus;
  reject_reason?: string | null;
  reviewed_by?: string | null;
  reviewed_at?: string | null;
  created_at: string;
}

export interface Document {
  id: string;
  user_id: string;
  type: DocumentType;
  file_url: string;
  original_name: string;
  uploaded_at: string;
}

export interface DocumentView extends Document {
  view_url: string;
}

export interface Tool {
  id: string;
  key: string;
  name: string;
  description: string;
  category: string;
  is_active: boolean;
}

export interface PermissionSet {
  id: string;
  name: string;
  description: string;
  tool_ids: string[];
}

export interface Admin {
  id: string;
  email: string;
  role: AdminRole;
  created_at: string;
}

export interface TokenPair {
  access_token: string;
  refresh_token: string;
}

export interface AdminLoginResponse {
  admin: Admin;
  tokens: TokenPair;
}

export interface DashboardStats {
  waiting_verification: number;
  new_today: number;
  active_users: number;
  visits: number;
}

export interface VerificationQueueItem {
  verification_id: string;
  user_id: string;
  email: string;
  company_name: string;
  participant_type: ParticipantType;
  status: VerificationStatus;
  created_at: string;
}

export interface VerificationDetail {
  verification: VerificationRequest;
  user: User;
  documents: DocumentView[];
}

export interface UserDetail {
  user: User;
  tools: Tool[];
  verification?: VerificationRequest;
}

export interface UserLoginResponse {
  user: User;
  tokens: TokenPair;
}

export interface MeResponse {
  user: User;
  verification: VerificationRequest | null;
}

export type CargoRequestStatus = "open" | "matched" | "closed";

export type CoordSource = "amap" | "osm";

// Always WGS-84 — Amap (GCJ-02) coordinates are converted in
// utils/gcj02.ts before being sent to the API. country is a lowercase
// ISO alpha-2 code from the geocoder ("cn", "kz", …); "" = unknown, which
// the backend treats as the default (non-China) matching radius.
export interface GeoPoint {
  lat: number;
  lng: number;
  label: string;
  source: CoordSource;
  country: string;
}

export interface CargoRequest {
  id: string;
  client_id: string;
  origin: GeoPoint;
  destination: GeoPoint;
  volume_m3: number;
  weight_kg: number;
  description: string;
  status: CargoRequestStatus;
  created_at: string;
}

export type OfferStatus = "submitted" | "selected" | "rejected";

// Deliberately identity-free: this is everything the client is allowed to
// see about an offer. offer_id is the offer's own uuid (needed for select)
// and reveals nothing about the participant. Never widen this with
// participant fields.
export interface AnonymizedOffer {
  offer_id: string;
  offer_number: number;
  rating: number;
  fill_percent?: number | null;
  price: number;
  currency: string;
  status: OfferStatus;
}

export interface RevealedContact {
  company_name: string;
  email: string;
  phone: string;
}

export interface SelectOfferResult {
  contact: RevealedContact;
  chat_id: string;
  reveals_used: number;
  reveals_limit: number;
}

export interface ChatView {
  id: string;
  cargo_request_id: string;
  origin_label: string;
  destination_label: string;
  counterpart_label: string;
  created_at: string;
}

export type ConsolidationStatus =
  | "suggested"
  | "a_agreed"
  | "b_agreed"
  | "both_agreed"
  | "declined";

// The other client's identity is deliberately absent — only the fact that
// a similar cargo exists, its size and the shared direction.
export interface ConsolidationView {
  suggestion_id: string;
  status: ConsolidationStatus;
  direction_label: string;
  other_volume_m3: number;
  other_weight_kg: number;
  my_side_agreed: boolean;
  other_side_agreed: boolean;
  created_at: string;
}

export interface ConsolidatedRequest {
  id: string;
  origin: GeoPoint;
  destination: GeoPoint;
  total_volume_m3: number;
  total_weight_kg: number;
  member_request_ids: string[];
  status: CargoRequestStatus;
  created_at: string;
}

export interface PlatformSettings {
  max_volume_m3: number;
  max_weight_kg: number;
}

export interface ChatMessage {
  id: string;
  chat_id: string;
  sender_id: string;
  body: string;
  attachment_url?: string | null;
  created_at: string;
}

export interface ParticipantRoute {
  id: string;
  user_id: string;
  origin: GeoPoint;
  destination: GeoPoint;
  created_at: string;
}

export interface NotificationItem {
  id: string;
  user_id: string;
  type: string;
  payload?: {
    cargo_request_id?: string;
    origin_label?: string;
    destination_label?: string;
  } | null;
  is_read: boolean;
  created_at: string;
}

export interface AuditLogEntry {
  id: string;
  admin_id: string;
  admin_email: string;
  action: string;
  target_user_id?: string | null;
  target_label?: string | null;
  details?: unknown;
  created_at: string;
}
