import { useEffect, useState, type FormEvent } from "react";
import { Link, Navigate, useNavigate } from "react-router-dom";
import { cabinetPathFor, useAuth } from "../auth/AuthContext";
import { ApiError } from "../api/client";
import { getToolCatalog, uploadRegistrationDocument } from "../api/participant";
import { t } from "../i18n";
import { useSeo } from "../utils/seo";
import type { LegalForm, Tool } from "../api/types";
import { groupToolsBySection, localizedToolText } from "../utils/toolSections";

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

// RegisterPage: шаг 1 — данные + выбор инструментов (роли больше нет),
// шаг 2 — загрузка документов на проверку. Аккаунт создаётся сразу
// (pending), сессия применяется — документы грузятся под своим токеном.
export function RegisterPage() {
  const { registerUser, kind, user } = useAuth();
  const navigate = useNavigate();
  useSeo({ title: t("register.title"), noindex: true });

  const [companyName, setCompanyName] = useState("");
  const [legalForm, setLegalForm] = useState<LegalForm>("individual");
  const [email, setEmail] = useState("");
  const [phone, setPhone] = useState("");
  const [password, setPassword] = useState("");
  const [passwordConfirm, setPasswordConfirm] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const [catalog, setCatalog] = useState<Tool[]>([]);
  const [selectedTools, setSelectedTools] = useState<Set<string>>(new Set());

  const [registered, setRegistered] = useState(false);
  const [docType, setDocType] = useState(DOCUMENT_TYPES[0]);
  const [docFile, setDocFile] = useState<File | null>(null);
  const [docError, setDocError] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);
  const [uploaded, setUploaded] = useState<UploadedDoc[]>([]);

  useEffect(() => {
    getToolCatalog().then(setCatalog).catch(() => setCatalog([]));
  }, []);

  // Уже залогиненный пользователь попадает в кабинет — кроме момента, когда
  // он только что зарегистрировался и грузит документы (registered).
  if (!registered && kind === "user" && user) {
    return <Navigate to={cabinetPathFor(user)} replace />;
  }

  const toolSections = groupToolsBySection(catalog);
  const monthlyTotal = catalog
    .filter((tl) => selectedTools.has(tl.id) && tl.price_kzt > 0)
    .reduce((sum, tl) => sum + tl.price_kzt, 0);
  const requiredDocumentTypes = legalForm === "individual"
    ? ["id_card"]
    : ["founding_docs", "business_license"];
  const canFinish = requiredDocumentTypes.every((type) => uploaded.some((doc) => doc.type === type));

  function toggleTool(id: string) {
    setSelectedTools((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
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
        legal_form: legalForm,
        password,
        tool_ids: [...selectedTools],
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
          <p className="register-hint"><strong>{t(`register.docsRequired.${legalForm}`)}</strong></p>

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
            disabled={!canFinish}
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
          <span className="field__label">{t("register.legalForm")}</span>
          <select value={legalForm} onChange={(e) => setLegalForm(e.target.value as LegalForm)}>
            <option value="individual">{t("legalForm.individual")}</option>
            <option value="legal_entity">{t("legalForm.legal_entity")}</option>
          </select>
        </label>
        <label className="field">
          <span className="field__label">{t(legalForm === "individual" ? "register.personName" : "register.companyName")}</span>
          <input value={companyName} onChange={(e) => setCompanyName(e.target.value)} autoFocus required />
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

        <div className="tools-pick">
          <span className="field__label">{t("register.toolsTitle")}</span>
          <p className="register-hint">{t("register.toolsHint")}</p>

          <div className="tools-pick--sections">
            {toolSections.map((section) => (
              <section className="tools-pick__section" key={section.key}>
                <h2 className="tools-pick__section-title">{t(`toolSections.${section.key}`)}</h2>
                <p className="tools-pick__section-hint">{t(`toolSectionHints.${section.key}`)}</p>
                {section.tools.map((tl) => (
                  <ToolCheck key={tl.id} tool={tl} checked={selectedTools.has(tl.id)} onToggle={toggleTool} />
                ))}
              </section>
            ))}
          </div>

          <div className="tools-pick__total">
            {t("register.monthlyTotal")}:{" "}
            <strong>
              {monthlyTotal > 0
                ? `${monthlyTotal.toLocaleString("ru-RU")} ₸/${t("register.perMonth")}`
                : t("register.free")}
            </strong>
          </div>
        </div>

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

function ToolCheck({
  tool,
  checked,
  onToggle,
}: {
  tool: Tool;
  checked: boolean;
  onToggle: (id: string) => void;
}) {
  return (
    <label className="tool-pick">
      <input type="checkbox" checked={checked} onChange={() => onToggle(tool.id)} />
      <span className="tool-pick__body">
        <span className="tool-pick__name">
          {localizedToolText(tool, "name")}
          <span className={tool.price_kzt > 0 ? "pill pill--yellow" : "pill pill--green"}>
            {tool.price_kzt > 0
              ? `${tool.price_kzt.toLocaleString("ru-RU")} ₸/${t("register.perMonth")}`
              : t("register.free")}
          </span>
        </span>
        <span className="tool-pick__desc">{localizedToolText(tool, "description")}</span>
      </span>
    </label>
  );
}
