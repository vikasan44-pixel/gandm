import ru from "./ru";

// Structured so adding "en"/"zh" later is just another entry here — see
// Block A: русский на старте, китайский и английский позже.
const dictionaries = { ru };
export type Locale = keyof typeof dictionaries;

const currentLocale: Locale = "ru";

export function t(path: string): string {
  const parts = path.split(".");
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let node: any = dictionaries[currentLocale];
  for (const part of parts) {
    node = node?.[part];
  }
  return typeof node === "string" ? node : path;
}
