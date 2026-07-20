import type { Tool } from "../api/types";
import { t } from "../i18n";

export type ToolSectionKey =
  | "cargo"
  | "warehouse"
  | "transport"
  | "customs"
  | "account"
  | "other";

export const TOOL_SECTION_ORDER: ToolSectionKey[] = [
  "cargo",
  "warehouse",
  "transport",
  "customs",
  "account",
  "other",
];

// The backend still authorizes individual tools. This map is presentation
// metadata only: it gives the catalog and navigation the same human-friendly
// structure without bringing rigid participant roles back.
const TOOL_SECTIONS: Record<string, ToolSectionKey> = {
  create_cargo_request: "cargo",
  view_cargo_requests: "cargo",
  manage_warehouse_slots: "warehouse",
  submit_fill_report: "warehouse",
  receive_cargo_by_route: "transport",
  submit_offer: "transport",
  manage_fleet: "transport",
  manage_customs_docs: "customs",
  manage_employees: "account",
};

export function toolSection(tool: Tool): ToolSectionKey {
  const explicit = TOOL_SECTIONS[tool.key];
  if (explicit) return explicit;

  switch (tool.category) {
    case "warehouse":
      return "warehouse";
    case "carrier":
      return "transport";
    case "customs":
      return "customs";
    case "cargo":
      return "cargo";
    default:
      return "other";
  }
}

export function groupToolsBySection(tools: Tool[]) {
  return TOOL_SECTION_ORDER.map((key) => ({
    key,
    tools: tools.filter((tool) => toolSection(tool) === key),
  })).filter((section) => section.tools.length > 0);
}

export function localizedToolText(tool: Tool, field: "name" | "description") {
  const key = `toolCatalog.${tool.key}.${field}`;
  const value = t(key);
  return value === key ? tool[field] : value;
}

export function toolCategoryLabel(category: string) {
  if (category === "admin") return t("navSections.administration");

  const key = `toolSections.${category}`;
  const value = t(key);
  return value === key ? t("toolSections.other") : value;
}
