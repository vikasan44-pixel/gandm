import { api } from "./client";
import type {
  AnonymizedCustomsOffer,
  AnonymizedOffer,
  CargoRequest,
  CustomsCompetition,
  CustomsOffer,
  CustomsSelectResult,
  DispatchThreshold,
  DriverCompetition,
  DriverCompetitionBid,
  DriverCompetitionView,
  DriverSelectResult,
  OpenDriverCompetition,
  RouteWithThreshold,
  Vehicle,
  Warehouse,
  VehicleDestination,
  VehicleDocumentType,
  VehicleTrip,
  VehicleTripStatus,
  ChatMessage,
  ChatView,
  ConsolidatedRequest,
  ConsolidatedSelectResult,
  ConsolidatedStatusView,
  ConsolidationView,
  FillReport,
  GeoPoint,
  MeResponse,
  NotificationItem,
  ParticipantRoute,
  Rating,
  SelectOfferResult,
  Tool,
  TransportProposal,
  TransportProposalView,
  PublicWarehouseCard,
  WarehouseOffer,
  WarehouseOfferView,
  WarehouseSelectResult,
  UserLoginResponse,
  UserRatingSummary,
} from "./types";

// --- Склады как ставщики цены на груз (Фаза 2) ---

// Склад-владелец: груз, который могут забрать его склады.
export function getAvailableCargoForWarehouses() {
  return api.get<CargoRequest[]>("/warehouse/available-cargo");
}

export function submitWarehouseOffer(cargoId: string, input: { warehouse_id: string; price: number; currency: string; conditions: string }) {
  return api.post<WarehouseOffer>(`/cargo/${cargoId}/warehouse-offers`, input);
}

// Клиент: предложения складов на его груз.
export function getWarehouseOffersForCargo(cargoId: string) {
  return api.get<WarehouseOfferView[]>(`/cargo/${cargoId}/warehouse-offers`);
}

export function selectWarehouseOffer(cargoId: string, offerId: string) {
  return api.post<WarehouseSelectResult>(`/cargo/${cargoId}/warehouse-offers/${offerId}/select`);
}

// --- Фаза 3: склады ставят цену на консолидированную заявку ---

export function getAvailableConsolidatedForWarehouses() {
  return api.get<ConsolidatedRequest[]>("/warehouse/available-consolidated");
}

export function submitWarehouseOfferForConsolidated(consolidatedId: string, input: { warehouse_id: string; price: number; currency: string; conditions: string }) {
  return api.post<WarehouseOffer>(`/consolidated/${consolidatedId}/warehouse-offers`, input);
}

export function getWarehouseOffersForConsolidated(consolidatedId: string) {
  return api.get<WarehouseOfferView[]>(`/consolidated/${consolidatedId}/warehouse-offers`);
}

export function selectWarehouseOfferForConsolidated(consolidatedId: string, offerId: string) {
  return api.post<WarehouseSelectResult>(`/consolidated/${consolidatedId}/warehouse-offers/${offerId}/select`);
}

// Поздний до-запрос: подходящие объединения для груза + присоединение.
export function getMatchingConsolidations(cargoId: string) {
  return api.get<ConsolidatedRequest[]>(`/cargo/${cargoId}/matching-consolidations`);
}

export function joinConsolidation(consolidatedId: string, cargoId: string) {
  return api.post<ConsolidatedRequest>(`/consolidated/${consolidatedId}/join`, { cargo_request_id: cargoId });
}

export function searchWarehouses(point: GeoPoint, radiusKm: number) {
  const p = new URLSearchParams({
    lat: String(point.lat),
    lng: String(point.lng),
    radius_km: String(radiusKm),
  });
  return api.get<PublicWarehouseCard[]>(`/warehouses/search?${p.toString()}`);
}

export function loginUser(email: string, password: string) {
  return api.post<UserLoginResponse>("/login", { email, password });
}

export interface RegisterInput {
  email: string;
  phone: string;
  company_name: string;
  legal_form: "individual" | "legal_entity";
  password: string;
  // Роли больше нет — участник выбирает инструменты сам.
  tool_ids: string[];
}

export function updateMyProfile(name: string, legalForm: "individual" | "legal_entity") {
  return api.patch<MeResponse>("/me/profile", { name, legal_form: legalForm });
}

export function registerUser(input: RegisterInput) {
  return api.post<UserLoginResponse>("/register", input);
}

// Каталог участнических инструментов (публичный) — для экрана регистрации.
export function getToolCatalog() {
  return api.get<Tool[]>("/tools/catalog");
}

export function getMyTools() {
  return api.get<Tool[]>("/my/tools");
}

export function setMyTools(toolIds: string[]) {
  return api.put<Tool[]>("/my/tools", { tool_ids: toolIds });
}

// uploadRegistrationDocument sends one verification document; requires the
// session issued right after registration.
export function uploadRegistrationDocument(docType: string, file: File) {
  const form = new FormData();
  form.append("type", docType);
  form.append("file", file);
  return api.postForm<{ id: string; type: string; original_name: string }>(
    "/register/documents",
    form
  );
}

export function getMe() {
  return api.get<MeResponse>("/me");
}

export interface CreateCargoInput {
  origin: GeoPoint;
  destination: GeoPoint;
  volume_m3: number;
  weight_kg: number;
  category: CargoRequest["category"];
  description: string;
  packaging: "packaged" | "bulk";
  places_count: number;
  stackable: boolean;
  adr_required: boolean;
  items: { length_m: number; width_m: number; height_m: number }[];
}

export function createCargo(input: CreateCargoInput) {
  return api.post<CargoRequest>("/cargo", input);
}

export function updateCargo(id: string, input: CreateCargoInput) {
  return api.put<CargoRequest>(`/cargo/${id}`, input);
}

export function cancelCargo(id: string) {
  return api.del<{ status: string }>(`/cargo/${id}`);
}

export function getMyCargo() {
  return api.get<CargoRequest[]>("/cargo/mine");
}

export interface CargoCompetitionResponse {
  offer: {
    id: string;
    cargo_request_id?: string | null;
    consolidated_request_id?: string | null;
    price: number;
    currency: string;
    conditions: string;
    warehouse_fill_percent?: number | null;
    status: "submitted" | "selected" | "rejected" | "withdrawn";
    created_at: string;
  };
  direction_label: string;
  origin?: GeoPoint;
  destination?: GeoPoint;
  category?: CargoRequest["category"];
  volume_m3: number;
  weight_kg: number;
  is_consolidated: boolean;
}

export function getMyCargoCompetitionResponses() {
  return api.get<CargoCompetitionResponse[]>("/offers/mine");
}

export function getAvailableCargo(from: GeoPoint | null = null, to: GeoPoint | null = null) {
  const params = new URLSearchParams();
  if (from) {
    params.set("from_lat", String(from.lat));
    params.set("from_lng", String(from.lng));
    params.set("from_country", from.country ?? "");
    params.set("from_label", from.label ?? "");
  }
  if (to) {
    params.set("to_lat", String(to.lat));
    params.set("to_lng", String(to.lng));
    params.set("to_country", to.country ?? "");
    params.set("to_label", to.label ?? "");
  }
  const query = params.toString();
  return api.get<CargoRequest[]>(`/cargo/available${query ? `?${query}` : ""}`);
}

export function getCargoOffers(cargoId: string) {
  return api.get<AnonymizedOffer[]>(`/cargo/${cargoId}/offers`);
}

export interface CreateOfferInput {
  price: number;
  currency: string;
  conditions: string;
  warehouse_fill_percent?: number | null;
}

export function createOffer(cargoId: string, input: CreateOfferInput) {
  return api.post<unknown>(`/cargo/${cargoId}/offers`, input);
}

export function updateOffer(offerId: string, input: CreateOfferInput) {
  return api.put<unknown>(`/offers/${offerId}`, input);
}

export function withdrawOffer(offerId: string) {
  return api.del<unknown>(`/offers/${offerId}`);
}

export function getRoutes() {
  return api.get<ParticipantRoute[]>("/routes");
}

export function addRoute(origin: GeoPoint, destination: GeoPoint) {
  return api.post<ParticipantRoute>("/routes", { origin, destination });
}

export function deleteRoute(routeId: string) {
  return api.del<{ status: string }>(`/routes/${routeId}`);
}

// --- автопарк (manage_fleet) ---

export interface VehicleInput {
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
  privacy_consent: boolean;
  // Опциональное местонахождение координатами (по карте) — «откуда».
  location?: GeoPoint | null;
  // Ноль или несколько назначений (координатами) — «куда».
  destinations: GeoPoint[];
}

export type VehicleDetailsInput = Pick<VehicleInput,
  "name" | "axles" | "capacity_kg" | "capacity_m3" | "length_m" | "width_m" | "height_m" | "body_type"
>;

export function getVehicles() {
  return api.get<Vehicle[]>("/fleet");
}

export function addVehicle(input: VehicleInput) {
  return api.post<Vehicle>("/fleet", input);
}

export function updateVehicleName(vehicleId: string, name: string) {
  return api.patch<Vehicle>(`/fleet/${vehicleId}/name`, { name });
}

export function updateVehicleDetails(vehicleId: string, input: VehicleDetailsInput) {
  return api.patch<Vehicle>(`/fleet/${vehicleId}`, input);
}

// null очищает местонахождение.
export function updateVehicleLocation(vehicleId: string, location: GeoPoint | null) {
  return api.patch<Vehicle>(`/fleet/${vehicleId}/location`, { location });
}

export function addVehicleDestination(vehicleId: string, point: GeoPoint) {
  return api.post<VehicleDestination>(`/fleet/${vehicleId}/destinations`, { point });
}

export function deleteVehicleDestination(vehicleId: string, destId: string) {
  return api.del<{ status: string }>(`/fleet/${vehicleId}/destinations/${destId}`);
}

export function deleteVehicle(vehicleId: string) {
  return api.del<{ status: string }>(`/fleet/${vehicleId}`);
}

export function updateVehicleRegistration(vehicleId: string, input: {
  registration_country: string;
  plate_number: string;
  vin: string;
  privacy_consent: boolean;
}) {
  return api.patch<Vehicle>(`/fleet/${vehicleId}/registration`, input);
}

export function uploadVehicleDocument(vehicleId: string, type: VehicleDocumentType, file: File) {
  const form = new FormData();
  form.set("type", type);
  form.set("file", file);
  return api.postForm<Vehicle>(`/fleet/${vehicleId}/documents`, form);
}

export interface VehicleTripInput {
  origin: GeoPoint;
  destination: GeoPoint;
  waypoints: GeoPoint[];
  can_pickup_en_route: boolean;
  departure_date: string;
  loaded_weight_kg: number;
  loaded_volume_m3: number;
  status: VehicleTripStatus;
}

export function createVehicleTrip(vehicleId: string, input: VehicleTripInput) {
  return api.post<VehicleTrip>(`/fleet/${vehicleId}/trips`, input);
}

export function updateVehicleTrip(vehicleId: string, tripId: string, input: VehicleTripInput) {
  return api.put<VehicleTrip>(`/fleet/${vehicleId}/trips/${tripId}`, input);
}

export function deleteVehicleTrip(vehicleId: string, tripId: string) {
  return api.del<{ status: string }>(`/fleet/${vehicleId}/trips/${tripId}`);
}

export type WarehouseInput = Omit<Warehouse, "id" | "user_id" | "created_at" | "updated_at">;

export function getMyWarehouses() {
  return api.get<Warehouse[]>("/warehouses/mine");
}

export function createWarehouse(input: WarehouseInput) {
  return api.post<Warehouse>("/warehouses", input);
}

export function updateWarehouse(id: string, input: WarehouseInput) {
  return api.put<Warehouse>(`/warehouses/${id}`, input);
}

export function deleteWarehouse(id: string) {
  return api.del<{ status: string }>(`/warehouses/${id}`);
}

// --- пороги отправки склада (manage_warehouse_slots) ---

export function getDispatchThresholds() {
  return api.get<RouteWithThreshold[]>("/dispatch-thresholds");
}

export interface DispatchThresholdInput {
  threshold_m3: number;
  manual_accrued_m3: number;
  warehouse_id?: string | null;
  estimated_dispatch_date?: string;
  status?: DispatchThreshold["status"];
}

export function setDispatchThreshold(routeId: string, input: DispatchThresholdInput) {
  return api.put<DispatchThreshold>(`/routes/${routeId}/dispatch-threshold`, {
    ...input,
  });
}

export function deleteDispatchThreshold(routeId: string) {
  return api.del<{ status: string }>(`/routes/${routeId}/dispatch-threshold`);
}

// --- сотрудники компании (ТЗ §13.1) ---

export interface CompanyEmployee {
  id: string;
  email: string;
  phone: string;
  status: string;
  created_at: string;
}

export function getEmployees() {
  return api.get<CompanyEmployee[]>("/employees");
}

export function createEmployee(email: string, phone: string, password: string) {
  return api.post<CompanyEmployee>("/employees", { email, phone, password });
}

export function setEmployeeBlocked(employeeId: string, blocked: boolean) {
  return api.post<CompanyEmployee>(`/employees/${employeeId}/block`, { blocked });
}

// --- антинакрутка (ТЗ §6.2): избранное и документы сделок ---

export interface FavoriteEntry {
  participant_id: string;
  company_name: string;
  created_at: string;
}

export function getFavorites() {
  return api.get<FavoriteEntry[]>("/favorites");
}

export function addFavorite(participantId: string) {
  return api.post<{ status: string }>("/favorites", { participant_id: participantId });
}

export function removeFavorite(participantId: string) {
  return api.del<{ status: string }>(`/favorites/${participantId}`);
}

export interface DealDocument {
  id: string;
  deal_id: string;
  uploader_id: string;
  original_name: string;
  uploaded_at: string;
  view_url: string;
}

export function getDealDocuments(dealId: string) {
  return api.get<DealDocument[]>(`/deals/${dealId}/documents`);
}

export function uploadDealDocument(dealId: string, file: File) {
  const form = new FormData();
  form.append("file", file);
  return api.postForm<DealDocument>(`/deals/${dealId}/documents`, form);
}

// --- конкурс водителей (ТЗ §11.4) ---

export function createDriverCompetition(routeId: string, volumeM3: number, dispatchDate: string) {
  return api.post<DriverCompetition>("/driver-competitions", {
    route_id: routeId,
    volume_m3: volumeM3,
    dispatch_date: dispatchDate,
  });
}

export function updateDriverCompetition(id: string, routeId: string, volumeM3: number, dispatchDate: string) {
  return api.put<DriverCompetition>(`/driver-competitions/${id}`, {
    route_id: routeId,
    volume_m3: volumeM3,
    dispatch_date: dispatchDate,
  });
}

export function cancelDriverCompetition(id: string) {
  return api.del<{ status: string }>(`/driver-competitions/${id}`);
}

export function getMyDriverCompetitions() {
  return api.get<DriverCompetitionView[]>("/driver-competitions/mine");
}

export function getOpenDriverCompetitions() {
  return api.get<OpenDriverCompetition[]>("/driver-competitions/open");
}

export function getMyDriverCompetitionResponses() {
  return api.get<OpenDriverCompetition[]>("/driver-competitions/responses");
}

export function createDriverBid(competitionId: string, price: number, currency: string, comment: string) {
  return api.post<DriverCompetitionBid>(`/driver-competitions/${competitionId}/bids`, {
    price,
    currency,
    comment,
  });
}

export function updateDriverBid(bidId: string, price: number, currency: string, comment: string) {
  return api.put<DriverCompetitionBid>(`/driver-bids/${bidId}`, { price, currency, comment });
}

export function withdrawDriverBid(bidId: string) {
  return api.del<DriverCompetitionBid>(`/driver-bids/${bidId}`);
}

export function selectDriverBid(competitionId: string, bidId: string) {
  return api.post<DriverSelectResult>(
    `/driver-competitions/${competitionId}/bids/${bidId}/select`
  );
}

// --- конкурс таможенных представителей (manage_customs_docs) ---

export function getCustomsCompetitions() {
  return api.get<CustomsCompetition[]>("/customs/competitions");
}

export function getMyCustomsCompetitionResponses() {
  return api.get<CustomsCompetition[]>("/customs/competitions/responses");
}

export function createCustomsOffer(consolidatedId: string, price: number, currency: string, conditions: string) {
  return api.post<CustomsOffer>(`/consolidated/${consolidatedId}/customs-offers`, {
    price,
    currency,
    conditions,
  });
}

export function updateCustomsOffer(offerId: string, price: number, currency: string, conditions: string) {
  return api.put<CustomsOffer>(`/customs-offers/${offerId}`, { price, currency, conditions });
}

export function withdrawCustomsOffer(offerId: string) {
  return api.del<CustomsOffer>(`/customs-offers/${offerId}`);
}

export function getCustomsOffers(consolidatedId: string) {
  return api.get<AnonymizedCustomsOffer[]>(`/consolidated/${consolidatedId}/customs-offers`);
}

export function selectCustomsOffer(consolidatedId: string, offerId: string) {
  return api.post<CustomsSelectResult>(
    `/consolidated/${consolidatedId}/customs-offers/${offerId}/select`
  );
}

export function getNotifications() {
  return api.get<NotificationItem[]>("/notifications");
}

export function getUnreadNotificationCount() {
  return api.get<{ unread: number }>("/notifications/unread-count");
}

export function markNotificationsRead() {
  return api.post<{ status: string }>("/notifications/read");
}

export function selectOffer(cargoId: string, offerId: string) {
  return api.post<SelectOfferResult>(`/cargo/${cargoId}/select`, { offer_id: offerId });
}

export function getConsolidation(cargoId: string) {
  return api.get<ConsolidationView | null>(`/cargo/${cargoId}/consolidation`);
}

export function agreeConsolidation(cargoId: string, suggestionId: string) {
  return api.post<unknown>(`/cargo/${cargoId}/consolidation/${suggestionId}/agree`);
}

export function declineConsolidation(cargoId: string, suggestionId: string) {
  return api.post<unknown>(`/cargo/${cargoId}/consolidation/${suggestionId}/decline`);
}

export function getMyConsolidated() {
  return api.get<ConsolidatedRequest[]>("/consolidated/mine");
}

export function getAvailableConsolidated() {
  return api.get<ConsolidatedRequest[]>("/cargo/available/consolidated");
}

export function createConsolidatedOffer(consolidatedId: string, input: CreateOfferInput) {
  return api.post<unknown>(`/consolidated/${consolidatedId}/offers`, input);
}

export function getConsolidatedOffers(consolidatedId: string) {
  return api.get<AnonymizedOffer[]>(`/consolidated/${consolidatedId}/offers`);
}

export function getConsolidatedStatus(consolidatedId: string) {
  return api.get<ConsolidatedStatusView>(`/consolidated/${consolidatedId}`);
}

export function inviteConsolidated(consolidatedId: string) {
  return api.post<{ status: string }>(`/consolidated/${consolidatedId}/invite`);
}

export function payConsolidated(consolidatedId: string) {
  return api.post<unknown>(`/consolidated/${consolidatedId}/pay`);
}

export function acceptConsolidated(consolidatedId: string) {
  return api.post<{ status: string; chat_id: string }>(`/consolidated/${consolidatedId}/accept`);
}

export function selectConsolidatedOffer(consolidatedId: string, offerId: string) {
  return api.post<ConsolidatedSelectResult>(`/consolidated/${consolidatedId}/select`, {
    offer_id: offerId,
  });
}

export interface CreateRatingInput {
  rated_user_id: string;
  score: number;
  comment?: string;
  deal_id?: string;
}

export function createRating(input: CreateRatingInput) {
  return api.post<Rating>("/ratings", input);
}

export function getUserRating(userId: string) {
  return api.get<UserRatingSummary>(`/users/${userId}/rating`);
}

export function getMyReceivedRatings() {
  return api.get<Rating[]>("/ratings/mine");
}

export interface CreateFillReportInput {
  expectedFillPercent: number;
  actualFillPercent: number;
  reportDate: string; // YYYY-MM-DD
  photo?: File | null;
}

export function createFillReport(input: CreateFillReportInput) {
  const form = new FormData();
  form.set("expected_fill_percent", String(input.expectedFillPercent));
  form.set("actual_fill_percent", String(input.actualFillPercent));
  form.set("report_date", input.reportDate);
  if (input.photo) {
    form.set("photo", input.photo);
  }
  return api.postForm<FillReport>("/warehouse/fill-report", form);
}

export function getMyFillReports() {
  return api.get<FillReport[]>("/warehouse/fill-reports");
}

export function getLatestFillReport(userId: string) {
  return api.get<FillReport>(`/users/${userId}/fill-report`);
}

export function getMyChats() {
  return api.get<ChatView[]>("/chats/mine");
}

export function getChatMessages(chatId: string, after?: string) {
  const suffix = after ? `?after=${encodeURIComponent(after)}` : "";
  return api.get<ChatMessage[]>(`/chats/${chatId}/messages${suffix}`);
}

export function sendChatMessage(chatId: string, body: string, attachmentUrl?: string) {
  return api.post<ChatMessage>(`/chats/${chatId}/messages`, {
    body,
    attachment_url: attachmentUrl ?? "",
  });
}

// --- Прямые предложения перевозчику (торг из поиска транспорта) ---

export interface SendTransportProposalInput {
  cargo_request_id?: string;
  origin: GeoPoint;
  destination: GeoPoint;
  cargo_name: string;
  volume_m3: number;
  weight_kg: number;
  pickup_date: string;
  currency: string;
  items: { length_m: number; width_m: number; height_m: number }[];
}

export function sendTransportProposal(vehicleId: string, input: SendTransportProposalInput) {
  return api.post<TransportProposal>(`/transport/${vehicleId}/proposals`, input);
}

export function getMyTransportProposals() {
  return api.get<TransportProposalView[]>("/transport-proposals/mine");
}

export function getIncomingTransportProposals() {
  return api.get<TransportProposalView[]>("/transport-proposals/incoming");
}

export function quoteTransportProposal(id: string, price: number) {
  return api.post<TransportProposal>(`/transport-proposals/${id}/quote`, { price });
}

export function counterTransportProposal(id: string, price: number) {
  return api.post<TransportProposal>(`/transport-proposals/${id}/counter`, { price });
}

export function finalTransportProposal(id: string, price: number) {
  return api.post<TransportProposal>(`/transport-proposals/${id}/final`, { price });
}

export function acceptTransportProposal(id: string) {
  return api.post<TransportProposalView>(`/transport-proposals/${id}/accept`);
}

export function rejectTransportProposal(id: string) {
  return api.post<TransportProposal>(`/transport-proposals/${id}/reject`);
}
