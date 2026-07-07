import { useEffect, useState, type FormEvent } from "react";
import { useAsync } from "../hooks/useAsync";
import { getPlatformSettings, updatePlatformSettings } from "../api/admin";
import { LoadingState } from "../components/common/LoadingState";
import { ErrorState } from "../components/common/ErrorState";
import { ApiError } from "../api/client";
import { t } from "../i18n";

// Consolidation capacity limits — stored in the DB (platform_settings), so
// changes apply immediately without restarting anything.
export function SettingsPage() {
  const settings = useAsync(getPlatformSettings, []);
  const [volume, setVolume] = useState("");
  const [weight, setWeight] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [savedNote, setSavedNote] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  useEffect(() => {
    if (settings.data) {
      setVolume(String(settings.data.max_volume_m3));
      setWeight(String(settings.data.max_weight_kg));
    }
  }, [settings.data]);

  async function handleSave(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSavedNote(false);

    const volumeNum = Number(volume);
    const weightNum = Number(weight);
    if (
      !Number.isFinite(volumeNum) ||
      volumeNum <= 0 ||
      !Number.isFinite(weightNum) ||
      weightNum <= 0
    ) {
      setError(t("settings.positiveRequired"));
      return;
    }

    setIsSaving(true);
    try {
      await updatePlatformSettings({ max_volume_m3: volumeNum, max_weight_kg: weightNum });
      setSavedNote(true);
      settings.reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <div className="page">
      <h1 className="page__title">{t("settings.title")}</h1>

      <section className="panel">
        <h2 className="panel__title">{t("settings.capacityTitle")}</h2>
        {settings.isLoading && <LoadingState />}
        {settings.error && <ErrorState message={settings.error} onRetry={settings.reload} />}
        {settings.data && (
          <form className="inline-form inline-form--stacked" onSubmit={handleSave}>
            <label className="field">
              <span className="field__label">{t("settings.maxVolume")}</span>
              <input
                type="number"
                min="0"
                step="1"
                value={volume}
                onChange={(e) => setVolume(e.target.value)}
              />
            </label>
            <label className="field">
              <span className="field__label">{t("settings.maxWeight")}</span>
              <input
                type="number"
                min="0"
                step="1"
                value={weight}
                onChange={(e) => setWeight(e.target.value)}
              />
            </label>
            {error && <div className="form-error">{error}</div>}
            {savedNote && <div className="detail-panel__resolved">{t("settings.saved")}</div>}
            <button className="btn btn--primary btn--sm" type="submit" disabled={isSaving}>
              {isSaving ? t("common.loading") : t("settings.save")}
            </button>
          </form>
        )}
      </section>
    </div>
  );
}
