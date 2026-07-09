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
  rating: UserRatingSummary;
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
// participant fields. rating: null = no ratings yet (show "—", not 0);
// latest_fill_* are the participant's newest fill report as bare numbers.
export interface AnonymizedOffer {
  offer_id: string;
  offer_number: number;
  rating: number | null;
  rating_count: number;
  fill_percent?: number | null;
  latest_fill_expected?: number | null;
  latest_fill_actual?: number | null;
  // Порог отправки склада по направлению груза (ТЗ §5.2): сколько м³ нужно
  // для отправки и сколько уже набрано — без раскрытия, чей это склад.
  dispatch_threshold_m3?: number | null;
  dispatch_accrued_m3?: number | null;
  price: number;
  currency: string;
  status: OfferStatus;
}

export interface Vehicle {
  id: string;
  user_id: string;
  axles: number;
  capacity_kg: number;
  length_m: number;
  width_m: number;
  height_m: number;
  body_type: string;
  current_location: string;
  created_at: string;
}

export interface DispatchThreshold {
  route_id: string;
  threshold_m3: number;
  accrued_m3: number;
  updated_at: string;
}

export interface RouteWithThreshold {
  route: ParticipantRoute;
  threshold?: DispatchThreshold | null;
}

export type CustomsOfferStatus = "submitted" | "selected" | "rejected";

export interface CustomsOffer {
  id: string;
  consolidated_request_id: string;
  customs_rep_id: string;
  price: number;
  currency: string;
  conditions: string;
  status: CustomsOfferStatus;
  created_at: string;
}

// Что видит таможенный представитель об открытом конкурсе: направление,
// объёмы и наименования грузов — без личных данных клиентов (ТЗ §10.2).
export interface CustomsCompetition {
  consolidated_request_id: string;
  direction_label: string;
  total_volume_m3: number;
  total_weight_kg: number;
  cargo_names: string[];
  created_at: string;
  my_offer?: CustomsOffer | null;
}

// Identity-free по той же политике, что AnonymizedOffer.
export interface AnonymizedCustomsOffer {
  offer_id: string;
  offer_number: number;
  rating: number | null;
  rating_count: number;
  price: number;
  currency: string;
  conditions: string;
  status: CustomsOfferStatus;
}

export interface CustomsSelectResult {
  contact: RevealedContact;
  customs_rep_id: string;
}

export type DriverCompetitionStatus = "open" | "closed";
export type DriverBidStatus = "submitted" | "selected" | "rejected";

export interface DriverCompetition {
  id: string;
  warehouse_id: string;
  route_id: string;
  volume_m3: number;
  dispatch_date: string;
  status: DriverCompetitionStatus;
  created_at: string;
}

export interface DriverCompetitionBid {
  id: string;
  competition_id: string;
  driver_id: string;
  price: number;
  currency: string;
  comment: string;
  status: DriverBidStatus;
  created_at: string;
}

// Ставка глазами склада — водитель не раскрывается до выбора.
export interface AnonymizedDriverBid {
  bid_id: string;
  bid_number: number;
  rating: number | null;
  rating_count: number;
  price: number;
  currency: string;
  comment: string;
  status: DriverBidStatus;
}

export interface DriverCompetitionView {
  competition: DriverCompetition;
  direction_label: string;
  bids: AnonymizedDriverBid[];
}

// Конкурс глазами водителя — без названия склада (ТЗ §11.4).
export interface OpenDriverCompetition {
  competition_id: string;
  direction_label: string;
  volume_m3: number;
  dispatch_date: string;
  created_at: string;
  my_bid?: DriverCompetitionBid | null;
}

export interface DriverSelectResult {
  contact: RevealedContact;
  driver_id: string;
  chat_id: string;
}

// Многофакторный рейтинг (ТЗ §8): composite — итоговая оценка 0–5 (null =
// нет сигнала), average/count — классическое среднее по отзывам, остальные
// поля — сырые компоненты для расшифровки «из чего складывается».
export interface UserRatingSummary {
  composite: number | null;
  average: number | null;
  count: number;
  days_on_platform: number;
  completed_deals: number;
  chat_messages: number;
  chats_total: number;
  chats_active: number;
}

export interface Rating {
  id: string;
  deal_id?: string | null;
  rated_user_id: string;
  rater_user_id: string;
  score: number;
  comment?: string | null;
  created_at: string;
}

export interface FillReport {
  id: string;
  user_id: string;
  expected_fill_percent: number;
  actual_fill_percent: number;
  photo_view_url?: string | null;
  report_date: string;
  created_at: string;
}

export interface RevealedContact {
  company_name: string;
  email: string;
  phone: string;
}

export interface SelectOfferResult {
  contact: RevealedContact;
  participant_id: string;
  chat_id: string;
  reveals_used: number;
  reveals_limit: number;
}

export interface ChatView {
  id: string;
  origin_label: string;
  destination_label: string;
  counterpart_label: string;
  // Set only for two-party chats — enables the in-chat rating form.
  counterpart_user_id?: string | null;
  deal_id: string;
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

export type ConsolidatedInviteStatus = "none" | "invited" | "accepted";

export interface ConsolidatedRequest {
  id: string;
  origin: GeoPoint;
  destination: GeoPoint;
  total_volume_m3: number;
  total_weight_kg: number;
  member_request_ids: string[];
  status: CargoRequestStatus;
  invite_status: ConsolidatedInviteStatus;
  initiator_client_id?: string | null;
  invited_client_id?: string | null;
  chat_id?: string | null;
  created_at: string;
}

export interface ClientContact {
  company_name: string;
  email: string;
  phone: string;
}

export type SelectionState = "none" | "waiting_other" | "mismatch" | "matched";

export interface ConsolidatedStatusView {
  consolidated: ConsolidatedRequest;
  am_initiator: boolean;
  am_invited: boolean;
  payment_done: boolean;
  counterpart?: ClientContact | null;
  my_offer_id?: string | null;
  other_has_chosen: boolean;
  selection_state: SelectionState;
  carrier_contact?: RevealedContact | null;
  carrier_id?: string | null;
}

export interface ConsolidatedSelectResult {
  selection_state: SelectionState;
  carrier_contact?: RevealedContact | null;
  carrier_id?: string | null;
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
