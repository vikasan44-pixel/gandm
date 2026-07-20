import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { t } from "../../i18n";

type ConfirmOptions = {
  title?: string;
  message: string;
  confirmLabel?: string;
  danger?: boolean;
};

type PendingConfirmation = ConfirmOptions & { resolve: (confirmed: boolean) => void };
type ConfirmFunction = (options: ConfirmOptions | string) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFunction | null>(null);

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [pending, setPending] = useState<PendingConfirmation | null>(null);
  const cancelButtonRef = useRef<HTMLButtonElement>(null);

  const confirm = useCallback<ConfirmFunction>((options) => new Promise((resolve) => {
    const normalized = typeof options === "string" ? { message: options } : options;
    setPending((current) => {
      current?.resolve(false);
      return { ...normalized, resolve };
    });
  }), []);

  const settle = useCallback((confirmed: boolean) => {
    setPending((current) => {
      current?.resolve(confirmed);
      return null;
    });
  }, []);

  useEffect(() => {
    if (!pending) return;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") settle(false);
    };
    window.addEventListener("keydown", handleKeyDown);
    requestAnimationFrame(() => cancelButtonRef.current?.focus());
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [pending, settle]);

  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {pending && createPortal(
        <div className="confirm-modal" role="presentation" onMouseDown={(event) => {
          if (event.target === event.currentTarget) settle(false);
        }}>
          <section className="confirm-modal__dialog" role="alertdialog" aria-modal="true" aria-labelledby="confirm-dialog-title" aria-describedby="confirm-dialog-message">
            <div className={`confirm-modal__icon${pending.danger === false ? " confirm-modal__icon--neutral" : ""}`} aria-hidden="true">!</div>
            <div className="confirm-modal__copy">
              <h2 id="confirm-dialog-title">{pending.title ?? t("common.confirmTitle")}</h2>
              <p id="confirm-dialog-message">{pending.message}</p>
            </div>
            <div className="confirm-modal__actions">
              <button ref={cancelButtonRef} className="btn btn--secondary" type="button" onClick={() => settle(false)}>{t("common.cancel")}</button>
              <button className={pending.danger === false ? "btn btn--primary" : "btn btn--danger"} type="button" onClick={() => settle(true)}>{pending.confirmLabel ?? t("common.delete")}</button>
            </div>
          </section>
        </div>,
        document.body,
      )}
    </ConfirmContext.Provider>
  );
}

export function useConfirm() {
  const confirm = useContext(ConfirmContext);
  if (!confirm) throw new Error("useConfirm must be used inside ConfirmProvider");
  return confirm;
}
