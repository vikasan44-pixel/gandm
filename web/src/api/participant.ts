import { api } from "./client";
import type {
  AnonymizedOffer,
  CargoRequest,
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
  UserLoginResponse,
  UserRatingSummary,
} from "./types";

export function loginUser(email: string, password: string) {
  return api.post<UserLoginResponse>("/login", { email, password });
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

export function getNotifications() {
  return api.get<NotificationItem[]>("/notifications");
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
