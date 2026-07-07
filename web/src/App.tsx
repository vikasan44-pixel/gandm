import { BrowserRouter, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import {
  HomeRedirect,
  RequireAdmin,
  RequireClient,
  RequirePartner,
} from "./components/RequireAuth";
import { AdminShell, ClientShell, PartnerShell } from "./components/layout/AppShell";
import { LoginPage } from "./pages/LoginPage";
import { UserLoginPage } from "./pages/UserLoginPage";
import { DashboardPage } from "./pages/DashboardPage";
import { VerificationPage } from "./pages/VerificationPage";
import { UsersPage } from "./pages/UsersPage";
import { ToolsPage } from "./pages/ToolsPage";
import { SettingsPage } from "./pages/SettingsPage";
import { ClientCargoPage } from "./pages/client/ClientCargoPage";
import { PartnerCargoPage } from "./pages/partner/PartnerCargoPage";
import { RoutesPage } from "./pages/partner/RoutesPage";
import { NotificationsPage } from "./pages/partner/NotificationsPage";
import { ChatsPage } from "./pages/chat/ChatsPage";

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/login" element={<UserLoginPage />} />
          <Route path="/admin/login" element={<LoginPage />} />

          <Route element={<RequireAdmin />}>
            <Route element={<AdminShell />}>
              <Route path="/admin/dashboard" element={<DashboardPage />} />
              <Route path="/admin/verification" element={<VerificationPage />} />
              <Route path="/admin/users" element={<UsersPage />} />
              <Route path="/admin/tools" element={<ToolsPage />} />
              <Route path="/admin/settings" element={<SettingsPage />} />
            </Route>
          </Route>

          <Route element={<RequireClient />}>
            <Route element={<ClientShell />}>
              <Route path="/client/cargo" element={<ClientCargoPage />} />
              <Route path="/client/chats" element={<ChatsPage />} />
            </Route>
          </Route>

          <Route element={<RequirePartner />}>
            <Route element={<PartnerShell />}>
              <Route path="/partner/cargo" element={<PartnerCargoPage />} />
              <Route path="/partner/routes" element={<RoutesPage />} />
              <Route path="/partner/notifications" element={<NotificationsPage />} />
              <Route path="/partner/chats" element={<ChatsPage />} />
            </Route>
          </Route>

          <Route path="/" element={<HomeRedirect />} />
          <Route path="*" element={<HomeRedirect />} />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  );
}
