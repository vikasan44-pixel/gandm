import { t } from "../../i18n";

export function LoadingState({ label }: { label?: string }) {
  return <div className="state state--loading">{label ?? t("common.loading")}</div>;
}
