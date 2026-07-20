import { useMemo, useState } from "react";
import { useAsync, type AsyncState } from "../hooks/useAsync";
import {
  createPermissionSet,
  createTool,
  getPermissionSets,
  getTools,
  updatePermissionSet,
  updateTool,
} from "../api/admin";
import { LoadingState } from "../components/common/LoadingState";
import { ErrorState } from "../components/common/ErrorState";
import { EmptyState } from "../components/common/EmptyState";
import { ApiError } from "../api/client";
import { t } from "../i18n";
import type { PermissionSet, Tool } from "../api/types";
import { toolCategoryLabel } from "../utils/toolSections";

export function ToolsPage() {
  const tools = useAsync(getTools, []);
  const sets = useAsync(getPermissionSets, []);

  return (
    <div className="page">
      <h1 className="page__title">{t("tools.title")}</h1>
      <div className="tools-layout">
        <ToolsSection tools={tools} />
        <PermissionSetsSection tools={tools} sets={sets} />
      </div>
    </div>
  );
}

function ToolsSection({ tools }: { tools: AsyncState<Tool[]> }) {
  const [isCreating, setIsCreating] = useState(false);
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [category, setCategory] = useState("");
  const [price, setPrice] = useState("0");
  const [error, setError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  const grouped = useMemo(() => {
    const map = new Map<string, Tool[]>();
    for (const tool of tools.data ?? []) {
      const cat = tool.category || t("tools.uncategorized");
      if (!map.has(cat)) map.set(cat, []);
      map.get(cat)?.push(tool);
    }
    return Array.from(map.entries());
  }, [tools.data]);

  function resetForm() {
    setKey("");
    setName("");
    setDescription("");
    setCategory("");
    setPrice("0");
  }

  async function handleCreate() {
    if (!key.trim() || !name.trim()) {
      setError(t("tools.keyNameRequired"));
      return;
    }
    setError(null);
    setIsSaving(true);
    try {
      await createTool({
        key: key.trim(),
        name: name.trim(),
        description,
        category,
        price_kzt: Number(price) || 0,
      });
      resetForm();
      setIsCreating(false);
      tools.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  async function handleToggleActive(tool: Tool) {
    setError(null);
    try {
      await updateTool(tool.id, { is_active: !tool.is_active });
      tools.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    }
  }

  return (
    <section className="panel">
      <div className="panel__header">
        <h2 className="panel__title">{t("tools.toolsTitle")}</h2>
        <button
          className="btn btn--secondary btn--sm"
          onClick={() => setIsCreating((v) => !v)}
        >
          {isCreating ? t("common.cancel") : t("tools.newTool")}
        </button>
      </div>

      {isCreating && (
        <div className="inline-form inline-form--stacked">
          <input placeholder={t("tools.key")} value={key} onChange={(e) => setKey(e.target.value)} />
          <input
            placeholder={t("tools.name")}
            value={name}
            onChange={(e) => setName(e.target.value)}
          />
          <input
            placeholder={t("tools.category")}
            value={category}
            onChange={(e) => setCategory(e.target.value)}
          />
          <textarea
            placeholder={t("tools.description")}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <label className="field">
            <span className="field__label">{t("tools.priceLabelTool")}</span>
            <input type="number" min={0} step="any" value={price} onChange={(e) => setPrice(e.target.value)} />
          </label>
          <button className="btn btn--primary btn--sm" onClick={handleCreate} disabled={isSaving}>
            {t("common.create")}
          </button>
        </div>
      )}
      {error && <div className="form-error">{error}</div>}

      {tools.isLoading && <LoadingState />}
      {tools.error && <ErrorState message={tools.error} onRetry={tools.reload} />}
      {tools.data && tools.data.length === 0 && <EmptyState message={t("tools.empty")} />}

      {grouped.map(([category, list]) => (
        <div key={category} className="tool-group">
          <h3 className="tool-group__title">{toolCategoryLabel(category)}</h3>
          <ul className="tool-group__list">
            {list.map((tool) => (
              <li key={tool.id} className="tool-row">
                <div>
                  <div className="tool-row__name">
                    {tool.name}
                    <span
                      className={tool.price_kzt > 0 ? "pill pill--yellow" : "pill pill--green"}
                      style={{ marginLeft: 8 }}
                    >
                      {tool.price_kzt > 0
                        ? `${tool.price_kzt.toLocaleString("ru-RU")} ₸/${t("register.perMonth")}`
                        : t("register.free")}
                    </span>
                  </div>
                  <div className="tool-row__key">{tool.key}</div>
                  {tool.description && (
                    <div className="tool-row__description">{tool.description}</div>
                  )}
                  {tool.category !== "admin" && (
                    <ToolPriceEditor tool={tool} onSaved={tools.reload} />
                  )}
                </div>
                <button
                  className={"btn btn--sm " + (tool.is_active ? "btn--secondary" : "btn--ghost")}
                  onClick={() => handleToggleActive(tool)}
                >
                  {tool.is_active ? t("tools.deactivate") : t("tools.activate")}
                </button>
              </li>
            ))}
          </ul>
        </div>
      ))}
    </section>
  );
}

function PermissionSetsSection({
  tools,
  sets,
}: {
  tools: AsyncState<Tool[]>;
  sets: AsyncState<PermissionSet[]>;
}) {
  const [isCreating, setIsCreating] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [price, setPrice] = useState("0");
  const [toolIds, setToolIds] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);

  function toggleTool(id: string) {
    setToolIds((prev) => (prev.includes(id) ? prev.filter((x) => x !== id) : [...prev, id]));
  }

  function startCreate() {
    setEditingId(null);
    setName("");
    setDescription("");
    setPrice("0");
    setToolIds([]);
    setIsCreating(true);
  }

  function startEdit(set: PermissionSet) {
    setEditingId(set.id);
    setName(set.name);
    setDescription(set.description);
    setPrice(String(set.price_kzt));
    setToolIds(set.tool_ids);
    setIsCreating(true);
  }

  async function handleSave() {
    if (!name.trim()) {
      setError(t("tools.nameRequired"));
      return;
    }
    const priceNum = Number(price);
    if (!Number.isFinite(priceNum) || priceNum < 0) {
      setError(t("tools.priceInvalid"));
      return;
    }
    setError(null);
    setIsSaving(true);
    try {
      if (editingId) {
        await updatePermissionSet(editingId, {
          name: name.trim(),
          description,
          price_kzt: priceNum,
          tool_ids: toolIds,
        });
      } else {
        await createPermissionSet({
          name: name.trim(),
          description,
          price_kzt: priceNum,
          tool_ids: toolIds,
        });
      }
      setIsCreating(false);
      setEditingId(null);
      sets.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <section className="panel">
      <div className="panel__header">
        <h2 className="panel__title">{t("tools.setsTitle")}</h2>
        <button
          className="btn btn--secondary btn--sm"
          onClick={isCreating ? () => setIsCreating(false) : startCreate}
        >
          {isCreating ? t("common.cancel") : t("tools.newSet")}
        </button>
      </div>

      {isCreating && (
        <div className="inline-form inline-form--stacked">
          <input placeholder={t("tools.name")} value={name} onChange={(e) => setName(e.target.value)} />
          <textarea
            placeholder={t("tools.description")}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <label className="field">
            <span className="field__label">{t("tools.priceLabel")}</span>
            <input
              type="number"
              min={0}
              step="any"
              value={price}
              onChange={(e) => setPrice(e.target.value)}
            />
          </label>
          <div className="tool-checklist">
            {(tools.data ?? []).map((tool) => (
              <label key={tool.id} className="tool-checklist__item">
                <input
                  type="checkbox"
                  checked={toolIds.includes(tool.id)}
                  onChange={() => toggleTool(tool.id)}
                />
                <span>{tool.name}</span>
              </label>
            ))}
          </div>
          <button className="btn btn--primary btn--sm" onClick={handleSave} disabled={isSaving}>
            {editingId ? t("common.save") : t("common.create")}
          </button>
        </div>
      )}
      {error && <div className="form-error">{error}</div>}

      {sets.isLoading && <LoadingState />}
      {sets.error && <ErrorState message={sets.error} onRetry={sets.reload} />}
      {sets.data && sets.data.length === 0 && <EmptyState message={t("tools.noSets")} />}

      <ul className="set-list">
        {(sets.data ?? []).map((set) => (
          <li key={set.id} className="set-row" onClick={() => startEdit(set)}>
            <div className="set-row__name">
              {set.name}
              <span className="pill pill--green" style={{ marginLeft: 8 }}>
                {set.price_kzt > 0
                  ? `${set.price_kzt.toLocaleString("ru-RU")} ₸/${t("tools.perMonth")}`
                  : t("tools.freePlan")}
              </span>
            </div>
            {set.description && <div className="set-row__description">{set.description}</div>}
            <div className="set-row__count">
              {t("tools.toolsCountLabel")}: {set.tool_ids.length}
            </div>
          </li>
        ))}
      </ul>

      {tools.data && sets.data && sets.data.length > 0 && (
        <ComparisonTable tools={tools.data} sets={sets.data} />
      )}
    </section>
  );
}

// Таблица сравнения тарифов (ТЗ §19.5): какие инструменты входят в каждый
// набор — галочки и прочерки.
function ComparisonTable({ tools, sets }: { tools: Tool[]; sets: PermissionSet[] }) {
  return (
    <>
      <h3 className="detail-panel__subtitle" style={{ marginTop: 16 }}>
        {t("tools.comparisonTitle")}
      </h3>
      <div className="table-scroll">
        <table className="table table--compact">
          <thead>
            <tr>
              <th>{t("tools.toolsTitle")}</th>
              {sets.map((s) => (
                <th key={s.id}>
                  {s.name}
                  <div className="tool-row__key">
                    {s.price_kzt > 0
                      ? `${s.price_kzt.toLocaleString("ru-RU")} ₸/${t("tools.perMonth")}`
                      : t("tools.freePlan")}
                  </div>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {tools.map((tool) => (
              <tr key={tool.id}>
                <td>{tool.name}</td>
                {sets.map((s) => (
                  <td key={s.id} style={{ textAlign: "center" }}>
                    {s.tool_ids.includes(tool.id) ? "✓" : "—"}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

// ToolPriceEditor — быстрое изменение цены участнического инструмента
// прямо в списке (админ задаёт ₸/мес; 0 = бесплатный).
function ToolPriceEditor({ tool, onSaved }: { tool: Tool; onSaved: () => void }) {
  const [price, setPrice] = useState(String(tool.price_kzt));
  const [isSaving, setIsSaving] = useState(false);

  async function save() {
    setIsSaving(true);
    try {
      await updateTool(tool.id, { price_kzt: Number(price) || 0 });
      onSaved();
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <div className="inline-form" style={{ marginTop: 6 }}>
      <input
        type="number"
        min={0}
        step="any"
        value={price}
        onChange={(e) => setPrice(e.target.value)}
        style={{ maxWidth: 120 }}
      />
      <button className="btn btn--ghost btn--sm" onClick={() => void save()} disabled={isSaving}>
        {isSaving ? t("common.loading") : t("common.save")}
      </button>
    </div>
  );
}
