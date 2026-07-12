import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import { HomeRedirect, RequireAdmin, RequireMember } from "./components/RequireAuth";
import { AdminShell, MemberShell } from "./components/layout/AppShell";
import { LoginPage } from "./pages/LoginPage";
import { UserLoginPage } from "./pages/UserLoginPage";
import { RegisterPage } from "./pages/RegisterPage";
import { LandingPage } from "./pages/LandingPage";
import { DashboardPage } from "./pages/DashboardPage";
import { AnalyticsPage } from "./pages/AnalyticsPage";
import { ModeratorsPage } from "./pages/ModeratorsPage";
import { VerificationPage } from "./pages/VerificationPage";
import { UsersPage } from "./pages/UsersPage";
import { ToolsPage } from "./pages/ToolsPage";
import { SettingsPage } from "./pages/SettingsPage";
import { ClientCargoPage } from "./pages/client/ClientCargoPage";
import { PartnerCargoPage } from "./pages/partner/PartnerCargoPage";
import { RoutesPage } from "./pages/partner/RoutesPage";
import { NotificationsPage } from "./pages/partner/NotificationsPage";
import { ChatsPage } from "./pages/chat/ChatsPage";
import { RatingPage } from "./pages/RatingPage";
import { FillReportsPage } from "./pages/partner/FillReportsPage";
import { FleetPage } from "./pages/partner/FleetPage";
import { CustomsPage } from "./pages/partner/CustomsPage";
import { DriverCompetitionsPage } from "./pages/partner/DriverCompetitionsPage";
import { EmployeesPage } from "./pages/partner/EmployeesPage";
import { MyToolsPage } from "./pages/MyToolsPage";

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
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
              <Route path="/app/available" element={<PartnerCargoPage />} />
              <Route path="/app/routes" element={<RoutesPage />} />
              <Route path="/app/fill-reports" element={<FillReportsPage />} />
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
      </BrowserRouter>
    </AuthProvider>
  );
}
