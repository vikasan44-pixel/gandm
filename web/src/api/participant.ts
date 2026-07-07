import { api } from "./client";
import type {
  AnonymizedOffer,
  CargoRequest,
  ChatMessage,
  ChatView,
  ConsolidatedRequest,
  ConsolidationView,
  GeoPoint,
  MeResponse,
  NotificationItem,
  ParticipantRoute,
  SelectOfferResult,
  UserLoginResponse,
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
