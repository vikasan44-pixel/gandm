import { useEffect, useRef, type MouseEvent, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { t } from "../../i18n";

export function DetailModal({ onClose, children, wide = false }: { onClose: () => void; children: ReactNode; wide?: boolean }) {
  const dialogRef = useRef<HTMLElement>(null);
  useEffect(() => {
    const previousOverflow = document.body.style.overflow;
    const previousFocus = document.activeElement as HTMLElement | null;
    document.body.style.overflow = "hidden";
    const handleKeyDown = (event: KeyboardEvent) => {
	  if (event.key === "Escape") onClose();
	  if (event.key === "Tab") {
		const focusable = dialogRef.current?.querySelectorAll<HTMLElement>('button:not([disabled]), a[href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])');
		if (!focusable?.length) return;
		const first = focusable[0];
		const last = focusable[focusable.length - 1];
		if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
		else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
	  }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", handleKeyDown);
      previousFocus?.focus();
    };
  }, [onClose]);

  return createPortal(
    <div className="cargo-modal" role="presentation" onMouseDown={(event: MouseEvent<HTMLDivElement>) => {
      if (event.target === event.currentTarget) onClose();
    }}>
      <section ref={dialogRef} className={`cargo-modal__dialog${wide ? " cargo-modal__dialog--wide" : ""}`} role="dialog" aria-modal="true" aria-label={t("common.details")}>
        <button className="cargo-modal__close" type="button" aria-label={t("common.close")} onClick={onClose} autoFocus>×</button>
        {children}
      </section>
    </div>,
    document.body,
  );
}
