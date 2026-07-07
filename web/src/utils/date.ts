export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("ru-RU", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}

export type Urgency = "fresh" | "waiting" | "urgent";

// "3+ дней" -> urgent (red), "сегодня" -> fresh (green, just arrived),
// otherwise -> waiting (yellow). The source wording ("сегодня жёлтый, свежие
// зелёный") reads ambiguously in isolation; this mapping is the one that
// keeps colors severity-consistent (red = oldest, green = newest) — flagged
// for the team to confirm against actual intent.
export function verificationUrgency(createdAtIso: string): Urgency {
  const created = new Date(createdAtIso);
  const now = new Date();
  const msPerDay = 24 * 60 * 60 * 1000;
  const daysElapsed = (now.getTime() - created.getTime()) / msPerDay;

  const isToday =
    created.getFullYear() === now.getFullYear() &&
    created.getMonth() === now.getMonth() &&
    created.getDate() === now.getDate();

  if (daysElapsed >= 3) return "urgent";
  if (isToday) return "fresh";
  return "waiting";
}
