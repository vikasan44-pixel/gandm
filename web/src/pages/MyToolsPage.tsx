import { useEffect, useState } from "react";
import { getMyTools, getToolCatalog, setMyTools } from "../api/participant";
import { LoadingState } from "../components/common/LoadingState";
import { ApiError } from "../api/client";
import { t } from "../i18n";
import type { Tool } from "../api/types";

// MyToolsPage — участник сам включает/выключает инструменты после
// регистрации (роли нет). Изменения применяются кнопкой «Сохранить»;
// внизу — итог платных инструментов в месяц.
export function MyToolsPage() {
  const [catalog, setCatalog] = useState<Tool[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  async function load() {
    setIsLoading(true);
    try {
      const [cat, mine] = await Promise.all([getToolCatalog(), getMyTools()]);
      setCatalog(cat);
      setSelected(new Set(mine.map((tl) => tl.id)));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsLoading(false);
    }
  }

  useEffect(() => {
    void load();
  }, []);

  function toggle(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  async function handleSave() {
    setError(null);
    setNotice(null);
    setIsSaving(true);
    try {
      await setMyTools([...selected]);
      setNotice(t("myTools.saved"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  const freeTools = catalog.filter((tl) => tl.price_kzt === 0);
  const paidTools = catalog.filter((tl) => tl.price_kzt > 0);
  const monthlyTotal = catalog
    .filter((tl) => selected.has(tl.id) && tl.price_kzt > 0)
    .reduce((sum, tl) => sum + tl.price_kzt, 0);

  return (
    <div className="page">
      <h1 className="page__title">{t("myTools.title")}</h1>
      <p className="panel__hint">{t("myTools.hint")}</p>

      {isLoading && <LoadingState />}
      {error && <div className="form-error">{error}</div>}

      {!isLoading && (
        <section className="panel">
          {freeTools.length > 0 && <div className="tools-pick__group">{t("register.toolsFree")}</div>}
          {freeTools.map((tl) => (
            <ToolRow key={tl.id} tool={tl} checked={selected.has(tl.id)} onToggle={toggle} />
          ))}
          {paidTools.length > 0 && <div className="tools-pick__group">{t("register.toolsPaid")}</div>}
          {paidTools.map((tl) => (
            <ToolRow key={tl.id} tool={tl} checked={selected.has(tl.id)} onToggle={toggle} />
          ))}

          <div className="tools-pick__total">
            {t("register.monthlyTotal")}:{" "}
            <strong>
              {monthlyTotal > 0
                ? `${monthlyTotal.toLocaleString("ru-RU")} ₸/${t("register.perMonth")}`
                : t("register.free")}
            </strong>
          </div>

          {notice && <p className="panel__hint">{notice}</p>}
          <button className="btn btn--primary btn--sm" onClick={() => void handleSave()} disabled={isSaving}>
            {isSaving ? t("common.loading") : t("myTools.save")}
          </button>
        </section>
      )}
    </div>
  );
}

function ToolRow({
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
          {tool.name}
          <span className={tool.price_kzt > 0 ? "pill pill--yellow" : "pill pill--green"}>
            {tool.price_kzt > 0
              ? `${tool.price_kzt.toLocaleString("ru-RU")} ₸/${t("register.perMonth")}`
              : t("register.free")}
          </span>
        </span>
        <span className="tool-pick__desc">{tool.description}</span>
      </span>
    </label>
  );
}
