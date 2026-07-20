import { lazy, Suspense } from "react";
import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import { HomeRedirect, RequireAdmin, RequireMember } from "./components/RequireAuth";
import { AdminShell, MemberShell } from "./components/layout/AppShell";
import { ConfirmProvider } from "./components/common/ConfirmDialog";
import { RatesProvider } from "./money/RatesContext";

const LoginPage = lazy(() => import("./pages/LoginPage").then((m) => ({ default: m.LoginPage })));
const UserLoginPage = lazy(() => import("./pages/UserLoginPage").then((m) => ({ default: m.UserLoginPage })));
const RegisterPage = lazy(() => import("./pages/RegisterPage").then((m) => ({ default: m.RegisterPage })));
const LandingPage = lazy(() => import("./pages/LandingPage").then((m) => ({ default: m.LandingPage })));
const DashboardPage = lazy(() => import("./pages/DashboardPage").then((m) => ({ default: m.DashboardPage })));
const AnalyticsPage = lazy(() => import("./pages/AnalyticsPage").then((m) => ({ default: m.AnalyticsPage })));
const ModeratorsPage = lazy(() => import("./pages/ModeratorsPage").then((m) => ({ default: m.ModeratorsPage })));
const VerificationPage = lazy(() => import("./pages/VerificationPage").then((m) => ({ default: m.VerificationPage })));
const UsersPage = lazy(() => import("./pages/UsersPage").then((m) => ({ default: m.UsersPage })));
const ToolsPage = lazy(() => import("./pages/ToolsPage").then((m) => ({ default: m.ToolsPage })));
const SettingsPage = lazy(() => import("./pages/SettingsPage").then((m) => ({ default: m.SettingsPage })));
const ClientCargoPage = lazy(() => import("./pages/client/ClientCargoPage").then((m) => ({ default: m.ClientCargoPage })));
const PartnerCargoPage = lazy(() => import("./pages/partner/PartnerCargoPage").then((m) => ({ default: m.PartnerCargoPage })));
const RoutesPage = lazy(() => import("./pages/partner/RoutesPage").then((m) => ({ default: m.RoutesPage })));
const NotificationsPage = lazy(() => import("./pages/partner/NotificationsPage").then((m) => ({ default: m.NotificationsPage })));
const ChatsPage = lazy(() => import("./pages/chat/ChatsPage").then((m) => ({ default: m.ChatsPage })));
const RatingPage = lazy(() => import("./pages/RatingPage").then((m) => ({ default: m.RatingPage })));
const FleetPage = lazy(() => import("./pages/partner/FleetPage").then((m) => ({ default: m.FleetPage })));
const CustomsPage = lazy(() => import("./pages/partner/CustomsPage").then((m) => ({ default: m.CustomsPage })));
const DriverCompetitionsPage = lazy(() => import("./pages/partner/DriverCompetitionsPage").then((m) => ({ default: m.DriverCompetitionsPage })));
const EmployeesPage = lazy(() => import("./pages/partner/EmployeesPage").then((m) => ({ default: m.EmployeesPage })));
const MyToolsPage = lazy(() => import("./pages/MyToolsPage").then((m) => ({ default: m.MyToolsPage })));
const CabinetPage = lazy(() => import("./pages/CabinetPage").then((m) => ({ default: m.CabinetPage })));
const TransportSearchPage = lazy(() => import("./pages/marketplace/TransportSearchPage").then((m) => ({ default: m.TransportSearchPage })));
const TransportProposalsPage = lazy(() => import("./pages/marketplace/TransportProposalsPage").then((m) => ({ default: m.TransportProposalsPage })));
const WarehouseSearchPage = lazy(() => import("./pages/marketplace/WarehouseSearchPage").then((m) => ({ default: m.WarehouseSearchPage })));
const WarehouseCargoPage = lazy(() => import("./pages/partner/WarehouseCargoPage").then((m) => ({ default: m.WarehouseCargoPage })));
const CustomsCatalogPage = lazy(() => import("./pages/marketplace/CustomsCatalogPage").then((m) => ({ default: m.CustomsCatalogPage })));
const MyCompetitionsPage = lazy(() => import("./pages/partner/MyCompetitionsPage").then((m) => ({ default: m.MyCompetitionsPage })));
const WarehousesPage = lazy(() => import("./pages/partner/WarehousesPage").then((m) => ({ default: m.WarehousesPage })));

export default function App() {
  return (
    <AuthProvider>
      <RatesProvider>
      <ConfirmProvider>
        <BrowserRouter future={{ v7_startTransition: true, v7_relativeSplatPath: true }}>
        <Suspense fallback={<div className="page" aria-busy="true">…</div>}>
        <Routes>
          <Route path="/login" element={<UserLoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/admin/login" element={<LoginPage />} />

          <Route element={<RequireAdmin />}>
            <Route element={<AdminShell />}>
              <Route path="/admin/dashboard" element={<DashboardPage />} />
              <Route path="/admin/verification" element={<VerificationPage />} />
              <Route path="/admin/users" element={<UsersPage />} />
              <Route path="/admin/tools" element={<ToolsPage />} />
              <Route path="/admin/analytics" element={<AnalyticsPage />} />
              <Route path="/admin/moderators" element={<ModeratorsPage />} />
              <Route path="/admin/settings" element={<SettingsPage />} />
            </Route>
          </Route>

          {/* Единый кабинет участника — роли нет, разделы по инструментам. */}
          <Route element={<RequireMember />}>
            <Route element={<MemberShell />}>
              <Route path="/app/cargo" element={<ClientCargoPage />} />
              <Route path="/app/cabinet" element={<CabinetPage />} />
              <Route path="/app/available" element={<PartnerCargoPage />} />
              <Route path="/app/search/transport" element={<TransportSearchPage />} />
              <Route path="/app/proposals" element={<TransportProposalsPage />} />
              <Route path="/app/search/warehouses" element={<WarehouseSearchPage />} />
              <Route path="/app/warehouse-cargo" element={<WarehouseCargoPage />} />
              <Route path="/app/warehouses" element={<WarehousesPage />} />
              <Route path="/app/search/customs" element={<CustomsCatalogPage />} />
              <Route path="/app/competitions" element={<MyCompetitionsPage />} />
              <Route path="/app/routes" element={<RoutesPage />} />
              <Route path="/app/fleet" element={<FleetPage />} />
              <Route path="/app/driver-competitions" element={<DriverCompetitionsPage />} />
              <Route path="/app/customs" element={<CustomsPage />} />
              <Route path="/app/employees" element={<EmployeesPage />} />
              <Route path="/app/chats" element={<ChatsPage />} />
              <Route path="/app/rating" element={<RatingPage />} />
              <Route path="/app/my-tools" element={<MyToolsPage />} />
              <Route path="/app/notifications" element={<NotificationsPage />} />
            </Route>
          </Route>

          {/* Лендинг на трёх адресах для SEO (язык из URL, см. i18n). */}
          <Route path="/" element={<LandingPage />} />
          <Route path="/en" element={<LandingPage />} />
          <Route path="/zh" element={<LandingPage />} />
          <Route path="*" element={<HomeRedirect />} />
        </Routes>
        </Suspense>
        </BrowserRouter>
      </ConfirmProvider>
      </RatesProvider>
    </AuthProvider>
  );
}
