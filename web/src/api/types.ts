export type ParticipantType =
  | "client"
  | "warehouse"
  | "carrier"
  | "driver"
  | "broker"
  | "customs_rep";

export type UserStatus = "pending" | "active" | "blocked" | "rejected";
export type LegalForm = "individual" | "legal_entity";

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
  legal_form: LegalForm;
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
  // Цена инструмента, ₸/мес; 0 = бесплатный (задаётся админом).
  price_kzt: number;
}

export interface PermissionSet {
  id: string;
  name: string;
  description: string;
  // Цена тарифа, ₸/мес (ТЗ §19.5); 0 = бесплатный.
  price_kzt: number;
  tool_ids: string[];
}

export interface Admin {
  id: string;
  email: string;
  role: AdminRole;
  created_at: string;
}

// Only the short-lived access token is returned in the body now; the refresh
// token lives in an httpOnly cookie the browser manages and JS can't read.
export interface TokenPair {
  access_token: string;
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
  // Иерархия «компания → сотрудники» (ТЗ §13.1).
  parent_company?: { id: string; company_name: string; email: string } | null;
  employees?: CompanyEmployeeRef[] | null;
}

export interface CompanyEmployeeRef {
  id: string;
  email: string;
  phone: string;
  status: UserStatus;
  created_at: string;
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
export type CargoCategory =
  | "chemicals"
  | "equipment"
  | "building_materials"
  | "home_appliances"
  | "furniture"
  | "food"
  | "textiles"
  | "auto_parts"
  | "metals"
  | "timber"
  | "medical_goods"
  | "agricultural_goods"
  | "plastics"
  | "dangerous_goods"
  | "other";

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
  // Подпись на языках интерфейса (ru/en/zh) из геокодера — чтобы адрес
  // показывался на языке пользователя. Отсутствует → есть только label.
  labels?: Record<string, string>;
}

export type CargoPackaging = "packaged" | "bulk";

export interface CargoRequestItem {
  id?: string;
  position?: number;
  length_m: number;
  width_m: number;
  height_m: number;
}

export interface CargoRequest {
  id: string;
  client_id: string;
  origin: GeoPoint;
  destination: GeoPoint;
  volume_m3: number;
  weight_kg: number;
  category: CargoCategory;
  description: string;
  status: CargoRequestStatus;
  created_at: string;
  // Логистика: упаковка/россыпью, места+габариты, штабелируемость, АДР.
  packaging: CargoPackaging;
  places_count: number;
  stackable: boolean;
  adr_required: boolean;
  items: CargoRequestItem[];
}

export type OfferStatus = "submitted" | "selected" | "rejected" | "withdrawn";

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
  dispatch_remaining_m3?: number | null;
  dispatch_date?: string | null;
  dispatch_status?: DispatchPlanStatus | null;
  price: number;
  currency: string;
  status: OfferStatus;
}

export interface VehicleDestination {
  id: string;
  point: GeoPoint;
}

export type VehicleTripStatus = "planned" | "loading" | "departed" | "completed";
export type VehicleVerificationStatus = "not_submitted" | "pending" | "verified" | "rejected";
export type VehicleDocumentType =
  | "registration_certificate"
  | "identity_document"
  | "insurance"
  | "photo_front"
  | "photo_back"
  | "photo_left"
  | "photo_right";

export interface VehicleTrip {
  id: string;
  vehicle_id: string;
  origin: GeoPoint;
  destination: GeoPoint;
  waypoints: GeoPoint[];
  can_pickup_en_route: boolean;
  departure_date: string;
  loaded_weight_kg: number;
  loaded_volume_m3: number;
  status: VehicleTripStatus;
  created_at: string;
  updated_at: string;
}

export interface Vehicle {
  id: string;
  user_id: string;
  name: string;
  axles: number;
  capacity_kg: number;
  capacity_m3: number;
  length_m: number;
  width_m: number;
  height_m: number;
  body_type: string;
  registration_country: string;
  plate_number: string;
  vin: string;
  verification_status: VehicleVerificationStatus;
  verification_reject_reason?: string | null;
  uploaded_document_types: VehicleDocumentType[];
  trust_percent: number;
  documents_verified: boolean;
  has_completed_trips: boolean;
  masked_plate: string;
  // Местонахождение координатами (по карте), опционально — «откуда».
  location?: GeoPoint | null;
  // Ноль или несколько назначений (координатами) — «куда».
  destinations: VehicleDestination[];
  // Загрузка относится к конкретному датированному рейсу.
  trips: VehicleTrip[];
  created_at: string;
}

export interface VehicleDocumentView {
  id: string;
  vehicle_id: string;
  type: VehicleDocumentType;
  original_name: string;
  content_type: string;
  uploaded_at: string;
  view_url: string;
}

export interface VehicleVerificationQueueItem {
  vehicle_id: string;
  user_id: string;
  company_name: string;
  email: string;
  plate_number: string;
  vin: string;
  status: VehicleVerificationStatus;
  created_at: string;
}

export interface VehicleVerificationDetail {
  vehicle: Vehicle;
  user: User;
  documents: VehicleDocumentView[];
}

export type DispatchPlanStatus = "collecting" | "ready" | "paused" | "dispatched";

export interface DispatchThreshold {
  route_id: string;
  warehouse_id?: string | null;
  threshold_m3: number;
  accrued_m3: number;
  platform_accrued_m3: number;
  manual_accrued_m3: number;
  remaining_m3: number;
  estimated_dispatch_date?: string | null;
  status: DispatchPlanStatus;
  updated_at: string;
}

export interface RouteWithThreshold {
  warehouse_id: string;
  route: ParticipantRoute;
  active_cargo_m3: number;
  threshold?: DispatchThreshold | null;
}

export type CustomsOfferStatus = "submitted" | "selected" | "rejected" | "withdrawn";

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
  cargo_items: Array<{ category: CargoCategory; description: string }>;
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
export type DriverBidStatus = "submitted" | "selected" | "rejected" | "withdrawn";

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
  // Группа (ТЗ §4.2 «два клиента и более»): участников всего / согласилось.
  members_count: number;
  agreed_count: number;
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
  members_count: number;
  accepted_count: number;
  counterpart?: ClientContact | null;
  counterparts?: ClientContact[] | null;
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

export type WarehouseStatus = "draft" | "published" | "paused";

export interface Warehouse {
  id: string;
  user_id: string;
  name: string;
  address: GeoPoint;
  contact_name: string;
  contact_phone: string;
  description: string;
  work_hours: string;
  covered_area_m2: number;
  open_area_m2: number;
  available_covered_area_m2: number;
  available_open_area_m2: number;
  max_weight_kg: number;
  max_volume_m3: number;
  services: string[];
  consolidation_enabled: boolean;
  consolidation_min_volume_m3: number;
  consolidation_frequency: string;
  pickup_enabled: boolean;
  pickup_cities: GeoPoint[];
  pickup_radius_km: number;
  own_transport: boolean;
  pickup_max_weight_kg: number;
  pickup_max_volume_m3: number;
  pickup_price_mode: string;
  dispatch_routes: Array<{
    id?: string;
    origin: GeoPoint;
    destination: GeoPoint;
  }>;
  status: WarehouseStatus;
  created_at: string;
  updated_at: string;
}

export interface NotificationItem {
  id: string;
  user_id: string;
  type: string;
  payload?: {
    cargo_request_id?: string;
    origin_label?: string;
    destination_label?: string;
    // Уведомления конкурсов (водители, таможня) несут направление одной
    // строкой; прочие поля payload читаются по типу уведомления.
    direction_label?: string;
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

// --- Прямые предложения перевозчику (торг из поиска транспорта) ---

export interface TransportProposalItem {
  id?: string;
  position?: number;
  length_m: number;
  width_m: number;
  height_m: number;
}

export type TransportProposalStatus =
  | "sent"
  | "carrier_quoted"
  | "client_countered"
  | "carrier_final"
  | "agreed"
  | "rejected";

export interface TransportProposal {
  id: string;
  client_id: string;
  vehicle_id: string;
  carrier_id: string;
  cargo_request_id?: string;
  origin: GeoPoint;
  destination: GeoPoint;
  cargo_name: string;
  volume_m3: number;
  weight_kg: number;
  places_count: number;
  pickup_date: string;
  status: TransportProposalStatus;
  current_price?: number;
  last_price_by?: "carrier" | "client";
  currency: string;
  chat_id?: string;
  created_at: string;
  updated_at: string;
  items: TransportProposalItem[];
}

// --- Склады как ставщики цены на груз (Фаза 2) ---

export interface WarehouseContact {
  warehouse_name: string;
  contact_name: string;
  contact_phone: string;
  email: string;
}

export interface WarehouseOffer {
  id: string;
  cargo_request_id: string;
  warehouse_id: string;
  warehouse_owner_id: string;
  price: number;
  currency: string;
  conditions: string;
  status: "submitted" | "selected" | "rejected";
  chat_id?: string;
  created_at: string;
  updated_at: string;
}

export interface WarehouseOfferView extends WarehouseOffer {
  warehouse_name: string;
  warehouse_address: GeoPoint;
  covered_area_m2: number;
  max_weight_kg: number;
  max_volume_m3: number;
  // Контакт приходит только после выбора этого предложения.
  contact?: WarehouseContact;
}

export interface WarehouseSelectResult {
  contact: WarehouseContact;
  chat_id: string;
}

// Склад в результатах поиска — БЕЗ контактов (контакты по подписке).
export interface PublicWarehouseCard {
  id: string;
  name: string;
  address: GeoPoint;
  description: string;
  work_hours: string;
  covered_area_m2: number;
  open_area_m2: number;
  available_covered_area_m2: number;
  available_open_area_m2: number;
  max_weight_kg: number;
  max_volume_m3: number;
  services: string[];
  consolidation_enabled: boolean;
  pickup_enabled: boolean;
  pickup_radius_km: number;
  own_transport: boolean;
  dispatch_routes: { id: string; origin: GeoPoint; destination: GeoPoint }[];
}

export interface TransportProposalView extends TransportProposal {
  viewer_role: "client" | "carrier";
  // Контакты контрагента — приходят только после «договорились».
  counterpart?: { company_name: string; email: string; phone: string };
  counterpart_id?: string;
}
export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
}
