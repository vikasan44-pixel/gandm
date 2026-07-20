import { useState, type FormEvent } from "react";
import { useAsync } from "../../hooks/useAsync";
import {
  getMyCargoCompetitionResponses,
  getMyCustomsCompetitionResponses,
  getMyDriverCompetitionResponses,
  updateCustomsOffer,
  updateDriverBid,
  updateOffer,
  withdrawCustomsOffer,
  withdrawDriverBid,
  withdrawOffer,
} from "../../api/participant";
import { LoadingState } from "../../components/common/LoadingState";
import { CurrencySelect } from "../../components/common/CurrencySelect";
import { Money } from "../../components/common/Money";
import { formatDateTime } from "../../utils/date";
import { DEFAULT_CURRENCY } from "../../utils/currency";
import { t } from "../../i18n";
import { ApiError } from "../../api/client";
import { MultilingualCargoCategory, MultilingualRoute } from "../../components/common/MultilingualLabels";
import { useConfirm } from "../../components/common/ConfirmDialog";
import { compactDirectionLabel } from "../../utils/locationLabel";

export function MyCompetitionsPage() {
  const cargo = useAsync(getMyCargoCompetitionResponses, []);
  const transport = useAsync(getMyDriverCompetitionResponses, []);
  const customs = useAsync(getMyCustomsCompetitionResponses, []);
  const transportResponses = (transport.data ?? []).filter((item) => item.my_bid);
  const customsResponses = (customs.data ?? []).filter((item) => item.my_offer);

  return (
    <div className="page competitions-page">
      <div>
        <h1 className="page__title">{t("myCompetitions.title")}</h1>
        <p className="marketplace-page__hint">{t("myCompetitions.hint")}</p>
      </div>

      {(cargo.isLoading || transport.isLoading || customs.isLoading) && <LoadingState />}

      <CompetitionSection title={t("myCompetitions.cargo")} empty={(cargo.data ?? []).length === 0}>
        {(cargo.data ?? []).map((item) => (
          <article className="competition-response" key={item.offer.id}>
            <div className="competition-response__header">
              <strong>{item.origin && item.destination ? <MultilingualRoute origin={item.origin} destination={item.destination} /> : compactDirectionLabel(item.direction_label)}</strong>
              <Status status={item.offer.status} />
            </div>
            {item.category && <p><MultilingualCargoCategory category={item.category} /></p>}
            <p>
              {item.volume_m3} м³ · {item.weight_kg.toLocaleString("ru-RU")} кг
              {item.is_consolidated ? ` · ${t("myCompetitions.consolidated")}` : ""}
            </p>
            <p>{t("myCompetitions.myOffer")}: <Money amount={item.offer.price} currency={item.offer.currency} /></p>
            {item.offer.conditions && <p>{t("myCompetitions.conditions")}: {item.offer.conditions}</p>}
            <time>{formatDateTime(item.offer.created_at)}</time>
            {item.offer.status === "submitted" && (
              <OfferActions
                initialPrice={item.offer.price}
                initialCurrency={item.offer.currency}
                initialConditions={item.offer.conditions}
                initialFillPercent={item.offer.warehouse_fill_percent}
                supportsFill
                onSave={(price, currency, conditions, fill) => updateOffer(item.offer.id, {
                  price,
                  currency,
                  conditions,
                  warehouse_fill_percent: fill,
                })}
                onWithdraw={() => withdrawOffer(item.offer.id)}
                onChanged={cargo.reload}
              />
            )}
          </article>
        ))}
      </CompetitionSection>

      <CompetitionSection title={t("myCompetitions.transport")} empty={transportResponses.length === 0}>
        {transportResponses.map((item) => {
          const bid = item.my_bid!;
          return (
            <article className="competition-response" key={item.competition_id}>
              <div className="competition-response__header">
                <strong>{compactDirectionLabel(item.direction_label)}</strong>
                <Status status={bid.status} />
              </div>
              <p>{item.volume_m3} м³{item.dispatch_date ? ` · ${item.dispatch_date}` : ""}</p>
              <p>{t("myCompetitions.myOffer")}: <Money amount={bid.price} currency={bid.currency} /></p>
              {bid.comment && <p>{t("myCompetitions.conditions")}: {bid.comment}</p>}
              <time>{formatDateTime(bid.created_at)}</time>
              {bid.status === "submitted" && (
                <OfferActions
                  initialPrice={bid.price}
                  initialCurrency={bid.currency}
                  initialConditions={bid.comment}
                  onSave={(price, currency, comment) => updateDriverBid(bid.id, price, currency, comment)}
                  onWithdraw={() => withdrawDriverBid(bid.id)}
                  onChanged={transport.reload}
                />
              )}
            </article>
          );
        })}
      </CompetitionSection>

      <CompetitionSection title={t("myCompetitions.warehouse")} empty>
        {null}
      </CompetitionSection>

      <CompetitionSection title={t("myCompetitions.customs")} empty={customsResponses.length === 0}>
        {customsResponses.map((item) => {
          const offer = item.my_offer!;
          return (
            <article className="competition-response" key={item.consolidated_request_id}>
              <div className="competition-response__header">
                <strong>{compactDirectionLabel(item.direction_label)}</strong>
                <Status status={offer.status} />
              </div>
              <p>{item.total_volume_m3} м³ · {item.total_weight_kg.toLocaleString("ru-RU")} кг</p>
              <p>{t("myCompetitions.myOffer")}: <Money amount={offer.price} currency={offer.currency} /></p>
              {offer.conditions && <p>{t("myCompetitions.conditions")}: {offer.conditions}</p>}
              <time>{formatDateTime(offer.created_at)}</time>
              {offer.status === "submitted" && (
                <OfferActions
                  initialPrice={offer.price}
                  initialCurrency={offer.currency}
                  initialConditions={offer.conditions}
                  onSave={(price, currency, conditions) => updateCustomsOffer(offer.id, price, currency, conditions)}
                  onWithdraw={() => withdrawCustomsOffer(offer.id)}
                  onChanged={customs.reload}
                />
              )}
            </article>
          );
        })}
      </CompetitionSection>
    </div>
  );
}

function CompetitionSection({ title, empty, children }: { title: string; empty: boolean; children: React.ReactNode }) {
  return (
    <section className="panel competition-section">
      <h2 className="panel__title">{title}</h2>
      {empty ? <p className="panel__hint">{t("myCompetitions.empty")}</p> : <div className="competition-responses">{children}</div>}
    </section>
  );
}

function Status({ status }: { status: string }) {
  const label = t(`offerStatus.${status}`);
  return <span className={`pill ${status === "selected" ? "pill--green" : status === "rejected" ? "pill--red" : status === "withdrawn" ? "pill--neutral" : "pill--yellow"}`}>{label}</span>;
}

function OfferActions({
  initialPrice,
  initialCurrency,
  initialConditions,
  initialFillPercent,
  supportsFill = false,
  onSave,
  onWithdraw,
  onChanged,
}: {
  initialPrice: number;
  initialCurrency: string;
  initialConditions: string;
  initialFillPercent?: number | null;
  supportsFill?: boolean;
  onSave: (price: number, currency: string, conditions: string, fillPercent: number | null) => Promise<unknown>;
  onWithdraw: () => Promise<unknown>;
  onChanged: () => void;
}) {
  const confirm = useConfirm();
  const [isEditing, setIsEditing] = useState(false);
  const [price, setPrice] = useState(String(initialPrice));
  const [currency, setCurrency] = useState<string>(initialCurrency || DEFAULT_CURRENCY);
  const [conditions, setConditions] = useState(initialConditions);
  const [fillPercent, setFillPercent] = useState(initialFillPercent == null ? "" : String(initialFillPercent));
  const [isBusy, setIsBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function handleSave(event: FormEvent) {
    event.preventDefault();
    setError(null);
    const priceValue = Number(price);
    if (!Number.isFinite(priceValue) || priceValue <= 0) {
      setError(t("partner.offerPricePositive"));
      return;
    }
    let fillValue: number | null = null;
    if (supportsFill && fillPercent.trim()) {
      fillValue = Number(fillPercent);
      if (!Number.isFinite(fillValue) || fillValue < 0 || fillValue > 100) {
        setError(t("partner.offerFillRange"));
        return;
      }
    }
    setIsBusy(true);
    try {
      await onSave(priceValue, currency, conditions.trim(), fillValue);
      setIsEditing(false);
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  async function handleWithdraw() {
    if (!await confirm({ message: t("myCompetitions.withdrawConfirm"), confirmLabel: t("myCompetitions.withdraw") })) return;
    setError(null);
    setIsBusy(true);
    try {
      await onWithdraw();
      onChanged();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.unexpectedError"));
    } finally {
      setIsBusy(false);
    }
  }

  return (
    <div className="competition-response__actions">
      {isEditing ? (
        <form className="inline-form" onSubmit={handleSave}>
          <input type="number" min={1} step="any" value={price} onChange={(event) => setPrice(event.target.value)} />
          <CurrencySelect value={currency} onChange={setCurrency} ariaLabel={t("common.currency")} />
          <input value={conditions} onChange={(event) => setConditions(event.target.value)} placeholder={t("myCompetitions.conditions")} />
          {supportsFill && (
            <input type="number" min={0} max={100} value={fillPercent} onChange={(event) => setFillPercent(event.target.value)} placeholder={t("partner.offerFill")} />
          )}
          <button className="btn btn--primary btn--sm" type="submit" disabled={isBusy}>{t("common.save")}</button>
          <button className="btn btn--ghost btn--sm" type="button" disabled={isBusy} onClick={() => setIsEditing(false)}>{t("common.cancel")}</button>
        </form>
      ) : (
        <>
          <button className="btn btn--secondary btn--sm" type="button" disabled={isBusy} onClick={() => setIsEditing(true)}>{t("common.edit")}</button>
          <button className="btn btn--ghost btn--sm" type="button" disabled={isBusy} onClick={() => void handleWithdraw()}>{t("myCompetitions.withdraw")}</button>
        </>
      )}
      {error && <div className="form-error">{error}</div>}
    </div>
  );
}
