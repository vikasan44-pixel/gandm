import { useState } from "react";
import { createRating } from "../../api/participant";
import { ApiError } from "../../api/client";
import { t } from "../../i18n";

// RatingForm: 1-5 stars + optional comment, tied to a specific counterparty
// and (optionally) a specific deal. Shown wherever a completed deal reveals
// the counterparty. After a successful submit (or a 409 "already rated")
// it collapses into a status line.
export function RatingForm({ ratedUserId, dealId }: { ratedUserId: string; dealId?: string }) {
  const [score, setScore] = useState(0);
  const [comment, setComment] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isDone, setIsDone] = useState(false);

  async function handleSubmit() {
    setError(null);
    if (score < 1 || score > 5) {
      setError(t("rating.scoreRequired"));
      return;
    }
    setIsSubmitting(true);
    try {
      await createRating({
        rated_user_id: ratedUserId,
        score,
        comment: comment.trim() || undefined,
        deal_id: dealId,
      });
      setIsDone(true);
    } catch (err) {
      if (err instanceof ApiError && err.code === "already_rated") {
        setIsDone(true);
      } else {
        setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
      }
    } finally {
      setIsSubmitting(false);
    }
  }

  if (isDone) {
    return <p className="panel__hint">{t("rating.submitted")}</p>;
  }

  return (
    <div className="rating-form">
      <span className="field__label">{t("rating.formTitle")}</span>
      <div className="rating-form__stars">
        {[1, 2, 3, 4, 5].map((value) => (
          <button
            key={value}
            type="button"
            className={
              "rating-form__star" + (value <= score ? " rating-form__star--active" : "")
            }
            onClick={() => setScore(value)}
            aria-label={`${value}`}
          >
            ★
          </button>
        ))}
      </div>
      <input
        placeholder={t("rating.commentPlaceholder")}
        value={comment}
        onChange={(e) => setComment(e.target.value)}
      />
      {error && <div className="form-error">{error}</div>}
      <button
        className="btn btn--secondary btn--sm"
        disabled={isSubmitting}
        onClick={handleSubmit}
      >
        {isSubmitting ? t("common.loading") : t("rating.submit")}
      </button>
    </div>
  );
}
