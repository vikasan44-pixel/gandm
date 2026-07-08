import { useEffect, useRef, useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import { getChatMessages, getMyChats, sendChatMessage } from "../../api/participant";
import { useAuth } from "../../auth/AuthContext";
import { LoadingState } from "../../components/common/LoadingState";
import { ErrorState } from "../../components/common/ErrorState";
import { EmptyState } from "../../components/common/EmptyState";
import { RatingForm } from "../../components/rating/RatingForm";
import { ApiError } from "../../api/client";
import { formatDateTime } from "../../utils/date";
import { t } from "../../i18n";
import type { ChatMessage, ChatView } from "../../api/types";

// Poll cadence for the open chat window. One interval per open chat,
// in-flight guard, cleared on unmount/chat switch — same discipline as the
// notifications poller, scoped to the window because only one chat is open
// at a time.
export const CHAT_POLL_MS = 5000;

export function ChatsPage() {
  const chats = useAsync(getMyChats, []);
  const [selected, setSelected] = useState<ChatView | null>(null);

  return (
    <div className="page page--split">
      <div className="page__list">
        <h1 className="page__title">{t("chats.title")}</h1>
        {chats.isLoading && <LoadingState />}
        {chats.error && <ErrorState message={chats.error} onRetry={chats.reload} />}
        {chats.data && chats.data.length === 0 && <EmptyState message={t("chats.empty")} />}
        {chats.data && chats.data.length > 0 && (
          <ul className="queue-list">
            {chats.data.map((chat) => (
              <li
                key={chat.id}
                className={
                  "queue-list__item" +
                  (selected?.id === chat.id ? " queue-list__item--active" : "")
                }
                onClick={() => setSelected(chat)}
              >
                <div className="queue-list__main">
                  <div className="queue-list__name">{chat.counterpart_label}</div>
                  <div className="queue-list__meta">
                    {chat.origin_label} → {chat.destination_label}
                  </div>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="page__detail">
        {selected ? (
          // key remounts the window per chat: fresh message state and a
          // fresh poll interval, the old one cleaned up by the effect.
          <ChatWindow key={selected.id} chat={selected} />
        ) : (
          <EmptyState message={t("chats.selectHint")} />
        )}
      </div>
    </div>
  );
}

function ChatWindow({ chat }: { chat: ChatView }) {
  const { user } = useAuth();
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [sendError, setSendError] = useState<string | null>(null);
  const [body, setBody] = useState("");
  const [attachmentUrl, setAttachmentUrl] = useState("");
  const [isSending, setIsSending] = useState(false);

  const lastMessageIdRef = useRef<string | null>(null);
  const inFlightRef = useRef(false);
  const scrollAnchorRef = useRef<HTMLDivElement | null>(null);

  function appendMessages(items: ChatMessage[]) {
    if (items.length === 0) return;
    setMessages((prev) => {
      // Dedupe against a poll that raced a just-sent message.
      const known = new Set(prev.map((m) => m.id));
      const fresh = items.filter((m) => !known.has(m.id));
      return fresh.length ? [...prev, ...fresh] : prev;
    });
    lastMessageIdRef.current = items[items.length - 1].id;
  }

  useEffect(() => {
    let cancelled = false;

    async function load(initial: boolean) {
      if (inFlightRef.current) return;
      inFlightRef.current = true;
      try {
        const after = initial ? undefined : (lastMessageIdRef.current ?? undefined);
        const items = await getChatMessages(chat.id, after);
        if (cancelled) return;
        if (initial) {
          setMessages(items);
          if (items.length > 0) {
            lastMessageIdRef.current = items[items.length - 1].id;
          }
        } else {
          appendMessages(items);
        }
        setLoadError(null);
      } catch (err) {
        if (!cancelled && initial) {
          setLoadError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
        }
        // Poll failures are silent — the next tick retries.
      } finally {
        inFlightRef.current = false;
        if (initial && !cancelled) setIsLoading(false);
      }
    }

    void load(true);
    const timer = setInterval(() => void load(false), CHAT_POLL_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [chat.id]);

  useEffect(() => {
    scrollAnchorRef.current?.scrollIntoView({ block: "end" });
  }, [messages.length]);

  async function handleSend(e: FormEvent) {
    e.preventDefault();
    setSendError(null);
    if (!body.trim() && !attachmentUrl.trim()) {
      setSendError(t("chats.bodyRequired"));
      return;
    }
    setIsSending(true);
    try {
      const msg = await sendChatMessage(chat.id, body.trim(), attachmentUrl.trim() || undefined);
      appendMessages([msg]);
      setBody("");
      setAttachmentUrl("");
    } catch (err) {
      setSendError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsSending(false);
    }
  }

  return (
    <div className="chat-window">
      <h2 className="detail-panel__title">
        {chat.counterpart_label} · {chat.origin_label} → {chat.destination_label}
      </h2>

      {isLoading && <LoadingState />}
      {loadError && <ErrorState message={loadError} />}

      <div className="chat-messages">
        {messages.map((msg) => (
          <div
            key={msg.id}
            className={
              "chat-msg" + (msg.sender_id === user?.id ? " chat-msg--own" : "")
            }
          >
            <div className="chat-msg__body">{msg.body}</div>
            {msg.attachment_url && (
              <a
                className="chat-msg__attachment"
                href={msg.attachment_url}
                target="_blank"
                rel="noreferrer"
              >
                {t("chats.attachment")}
              </a>
            )}
            <div className="chat-msg__time">{formatDateTime(msg.created_at)}</div>
          </div>
        ))}
        <div ref={scrollAnchorRef} />
      </div>

      {chat.counterpart_user_id && (
        <RatingForm ratedUserId={chat.counterpart_user_id} dealId={chat.deal_id} />
      )}

      <form className="chat-form" onSubmit={handleSend}>
        <textarea
          placeholder={t("chats.inputPlaceholder")}
          value={body}
          onChange={(e) => setBody(e.target.value)}
        />
        <input
          placeholder={t("chats.attachmentPlaceholder")}
          value={attachmentUrl}
          onChange={(e) => setAttachmentUrl(e.target.value)}
        />
        {sendError && <div className="form-error">{sendError}</div>}
        <button className="btn btn--primary btn--sm" type="submit" disabled={isSending}>
          {isSending ? t("common.loading") : t("chats.send")}
        </button>
      </form>
    </div>
  );
}
