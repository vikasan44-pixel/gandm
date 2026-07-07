import { getNotifications } from "../api/participant";

// Module-level singleton poller for the unread-notifications badge.
//
// Why a singleton and not a per-component setInterval: no matter how many
// components mount/unmount (StrictMode double-mounts, HMR remounts, route
// churn), the app holds AT MOST ONE interval and never stacks parallel
// requests — an in-flight guard skips a tick if the previous request hasn't
// finished. The interval starts with the first subscriber and stops with
// the last one, so nothing polls while no partner UI is on screen.

export const NOTIFICATIONS_POLL_MS = 30_000;

type Subscriber = (unreadCount: number) => void;

const subscribers = new Set<Subscriber>();
let timer: number | null = null;
let inFlight = false;
let lastCount = 0;

async function poll(): Promise<void> {
  if (inFlight) return;
  inFlight = true;
  try {
    const items = await getNotifications();
    lastCount = items.filter((n) => !n.is_read).length;
    subscribers.forEach((cb) => cb(lastCount));
  } catch {
    // A failed badge refresh isn't worth surfacing; the notifications page
    // shows the real error. Next tick retries.
  } finally {
    inFlight = false;
  }
}

// subscribeUnreadCount registers a listener and returns an unsubscribe
// function — shaped for direct use as a useEffect body.
export function subscribeUnreadCount(cb: Subscriber): () => void {
  subscribers.add(cb);
  cb(lastCount);
  if (timer === null) {
    void poll();
    timer = window.setInterval(() => void poll(), NOTIFICATIONS_POLL_MS);
  }
  return () => {
    subscribers.delete(cb);
    if (subscribers.size === 0 && timer !== null) {
      window.clearInterval(timer);
      timer = null;
    }
  };
}

// refreshUnreadCount forces an immediate poll outside the schedule — e.g.
// right after notifications are marked read, so the badge clears without
// waiting up to NOTIFICATIONS_POLL_MS.
export function refreshUnreadCount(): void {
  void poll();
}
