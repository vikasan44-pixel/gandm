import type {
  CargoRequestStatus,
  OfferStatus,
  UserStatus,
  VerificationStatus,
} from "../../api/types";
import { t } from "../../i18n";
import type { Urgency } from "../../utils/date";

type Kind = "neutral" | "green" | "yellow" | "red";

const userStatusKind: Record<UserStatus, Kind> = {
  pending: "yellow",
  active: "green",
  blocked: "red",
  rejected: "red",
};

const verificationStatusKind: Record<VerificationStatus, Kind> = {
  pending: "yellow",
  approved: "green",
  rejected: "red",
};

const urgencyKind: Record<Urgency, Kind> = {
  fresh: "green",
  waiting: "yellow",
  urgent: "red",
};

export function UserStatusPill({ status }: { status: UserStatus }) {
  return (
    <span className={`pill pill--${userStatusKind[status]}`}>
      {t(`status.user.${status}`)}
    </span>
  );
}

export function VerificationStatusPill({ status }: { status: VerificationStatus }) {
  return (
    <span className={`pill pill--${verificationStatusKind[status]}`}>
      {t(`status.verification.${status}`)}
    </span>
  );
}

export function UrgencyPill({ urgency }: { urgency: Urgency }) {
  return (
    <span className={`pill pill--${urgencyKind[urgency]}`}>{t(`urgency.${urgency}`)}</span>
  );
}

const cargoStatusKind: Record<CargoRequestStatus, Kind> = {
  open: "green",
  matched: "yellow",
  closed: "neutral",
};

export function CargoStatusPill({ status }: { status: CargoRequestStatus }) {
  return (
    <span className={`pill pill--${cargoStatusKind[status]}`}>
      {t(`cargoStatus.${status}`)}
    </span>
  );
}

const offerStatusKind: Record<OfferStatus, Kind> = {
  submitted: "yellow",
  selected: "green",
  rejected: "red",
  withdrawn: "neutral",
};

export function OfferStatusPill({ status }: { status: OfferStatus }) {
  return (
    <span className={`pill pill--${offerStatusKind[status]}`}>
      {t(`offerStatus.${status}`)}
    </span>
  );
}
