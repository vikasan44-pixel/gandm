import { useState, type FormEvent } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { cabinetPathFor, useAuth } from "../auth/AuthContext";
import { ApiError } from "../api/client";
import { uploadRegistrationDocument } from "../api/participant";
import { t } from "../i18n";

const PARTICIPANT_TYPES = ["client", "warehouse", "carrier", "driver", "broker", "customs_rep"];
const DOCUMENT_TYPES = [
  "id_card",
  "founding_docs",
  "business_license",
  "employment_contract",
  "vehicle_doc",
];

interface UploadedDoc {
  id: string;
  type: string;
  name: string;
}

// RegisterPage: шаг 1 — данные компании и аккаунт, шаг 2 — загрузка
// документов на проверку. Аккаунт создаётся сразу (статус pending), сессия
// применяется — поэтому документы грузятся под собственным токеном.
export function RegisterPage() {
  const { registerUser, kind, user } = useAuth();
  const navigate = useNavigate();

  const [companyName, setCompanyName] = useState("");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [participantType, setParticipantType] = useState("client");
  const [password, setPassword] = useState("");
  const [passwordConfirm, setPasswordConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const [registered, setRegistered] = useState(false);
  const [docType, setDocType] = useState(DOCUMENT_TYPES[0]);
  const [docFile, setDocFile] = useState<File | null>(null);
  const [docError, setDocError] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [uploaded, setUploaded] = useState<UploadedDoc[]>([]);

  // Уже залогиненный пользователь попадает в свой кабинет — кроме момента,
  // когда он только что зарегистрировался и грузит документы (registered).
  if (!registered && kind === "user" && user) {
    return <Navigate to={cabinetPathFor(user)} replace />;
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    if (password.length < 8) {
      setError(t("register.passwordTooShort"));
      return;
    }
    if (password !== passwordConfirm) {
      setError(t("register.passwordMismatch"));
      return;
    }
    setIsSubmitting(true);
    try {
      await registerUser({
        email,
        phone,
        company_name: companyName,
        participant_type: participantType,
        password,
      });
      setRegistered(true);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSubmitting(false);
    }
  }

  async function handleUpload(e: FormEvent) {
    e.preventDefault();
    setDocError(null);
    if (!docFile) {
      setDocError(t("register.docFileRequired"));
      return;
    }
    setIsUploading(true);
    try {
      const doc = await uploadRegistrationDocument(docType, docFile);
      setUploaded((prev) => [...prev, { id: doc.id, type: docType, name: docFile.name }]);
      setDocFile(null);
    } catch (err) {
      setDocError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsUploading(false);
    }
  }

  if (registered && user) {
    return (
      <div className="login-screen">
        <div className="login-card login-card--wide">
          <h1 className="login-card__title">{t("register.docsTitle")}</h1>
          <p className="register-hint">{t("register.docsHint")}</p>

          <form onSubmit={handleUpload} className="register-doc-form">
            <label className="field">
              <span className="field__label">{t("register.docType")}</span>
              <select value={docType} onChange={(e) => setDocType(e.target.value)}>
                {DOCUMENT_TYPES.map((dt) => (
                  <option key={dt} value={dt}>
                    {t(`documentType.${dt}`)}
                  </option>
                ))}
              </select>
            </label>
            <label className="field">
              <span className="field__label">{t("register.docFile")}</span>
              <input
                type="file"
                accept=".pdf,image/jpeg,image/png"
                onChange={(e) => setDocFile(e.target.files?.[0] ?? null)}
              />
            </label>
            {docError && <div className="form-error">{docError}</div>}
            <button className="btn btn--secondary" type="submit" disabled={isUploading}>
              {isUploading ? t("common.loading") : t("register.docUpload")}
            </button>
          </form>

          {uploaded.length > 0 ? (
            <ul className="register-doc-list">
              {uploaded.map((d) => (
                <li key={d.id}>
                  ✓ {t(`documentType.${d.type}`)} — {d.name}
                </li>
              ))}
            </ul>
          ) : (
            <p className="register-hint">{t("register.docsEmpty")}</p>
          )}

          <p className="register-hint">{t("register.pendingNote")}</p>
          <button
            className="btn btn--primary"
            type="button"
            onClick={() => navigate(cabinetPathFor(user), { replace: true })}
          >
            {t("register.finish")}
          </button>
          <p className="register-hint register-hint--center">{t("register.finishHint")}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="login-screen">
      <form className="login-card login-card--wide" onSubmit={handleSubmit}>
        <h1 className="login-card__title">{t("register.title")}</h1>
        <label className="field">
          <span className="field__label">{t("register.companyName")}</span>
          <input
            value={companyName}
            onChange={(e) => setCompanyName(e.target.value)}
            autoFocus
            required
          />
        </label>
        <label className="field">
          <span className="field__label">{t("register.email")}</span>
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
        <label className="field">
          <span className="field__label">{t("register.phone")}</span>
          <input type="tel" value={phone} onChange={(e) => setPhone(e.target.value)} required />
        </label>
        <label className="field">
          <span className="field__label">{t("register.participantType")}</span>
          <select value={participantType} onChange={(e) => setParticipantType(e.target.value)}>
            {PARTICIPANT_TYPES.map((pt) => (
              <option key={pt} value={pt}>
                {t(`participantType.${pt}`)}
              </option>
            ))}
          </select>
        </label>
        <label className="field">
          <span className="field__label">{t("register.password")}</span>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={8}
          />
        </label>
        <label className="field">
          <span className="field__label">{t("register.passwordConfirm")}</span>
          <input
            type="password"
            value={passwordConfirm}
            onChange={(e) => setPasswordConfirm(e.target.value)}
            required
          />
        </label>
        {error && <div className="form-error">{error}</div>}
        <button className="btn btn--primary" type="submit" disabled={isSubmitting}>
          {isSubmitting ? t("common.loading") : t("register.submit")}
        </button>
        <Link className="login-card__switch" to="/login">
          {t("register.toLogin")}
        </Link>
      </form>
    </div>
  );
}
