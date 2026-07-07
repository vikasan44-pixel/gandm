import { t } from "../../i18n";

export function ErrorState({
  message,
  onRetry,
}: {
  message: string;
  onRetry?: () => void;
}) {
  return (
    <div className="state state--error">
      <p>{message}</p>
      {onRetry && (
        <button className="btn btn--secondary btn--sm" onClick={onRetry}>
          {t("common.retry")}
        </button>
      )}
    </div>
  );
}
