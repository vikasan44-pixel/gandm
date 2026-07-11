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
  VehicleDestination,
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
  UserLoginResponse,
  UserRatingSummary,
} from "./types";

export function loginUser(email: string, password: string) {
  return api.post<UserLoginResponse>("/login", { email, password });
}

export interface RegisterInput {
  email: string;
  phone: string;
  company_name: string;
  password: string;
  // Роли больше нет — участник выбирает инструменты сам.
  tool_ids: string[];
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
  description: string;
}

export function createCargo(input: CreateCargoInput) {
  return api.post<CargoRequest>("/cargo", input);
}

export function getMyCargo() {
  return api.get<CargoRequest[]>("/cargo/mine");
}

export function getAvailableCargo() {
  return api.get<CargoRequest[]>("/cargo/available");
}

export function getCargoOffers(cargoId: string) {
  return api.get<AnonymizedOffer[]>(`/cargo/${cargoId}/offers`);
}

export interface CreateOfferInput {
  price: number;
  conditions: string;
  warehouse_fill_percent?: number | null;
}

export function createOffer(cargoId: string, input: CreateOfferInput) {
  return api.post<unknown>(`/cargo/${cargoId}/offers`, input);
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
  axles: number;
  capacity_kg: number;
  capacity_m3: number;
  length_m: number;
  width_m: number;
  height_m: number;
  body_type: string;
  // Опциональное местонахождение координатами (по карте) — «откуда».
  location?: GeoPoint | null;
  // Ноль или несколько назначений (координатами) — «куда».
  destinations: GeoPoint[];
}

export function getVehicles() {
  return api.get<Vehicle[]>("/fleet");
}

export function addVehicle(input: VehicleInput) {
  return api.post<Vehicle>("/fleet", input);
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

// --- пороги отправки склада (manage_warehouse_slots) ---

export function getDispatchThresholds() {
  return api.get<RouteWithThreshold[]>("/dispatch-thresholds");
}

export function setDispatchThreshold(routeId: string, thresholdM3: number, accruedM3: number) {
  return api.put<DispatchThreshold>(`/routes/${routeId}/dispatch-threshold`, {
    threshold_m3: thresholdM3,
    accrued_m3: accruedM3,
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

export function getMyDriverCompetitions() {
  return api.get<DriverCompetitionView[]>("/driver-competitions/mine");
}

export function getOpenDriverCompetitions() {
  return api.get<OpenDriverCompetition[]>("/driver-competitions/open");
}

export function createDriverBid(competitionId: string, price: number, comment: string) {
  return api.post<DriverCompetitionBid>(`/driver-competitions/${competitionId}/bids`, {
    price,
    comment,
  });
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

export function createCustomsOffer(consolidatedId: string, price: number, conditions: string) {
  return api.post<CustomsOffer>(`/consolidated/${consolidatedId}/customs-offers`, {
    price,
    conditions,
  });
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
