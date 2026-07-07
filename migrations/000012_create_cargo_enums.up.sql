CREATE TYPE cargo_request_status AS ENUM (
    'open',
    'matched',
    'closed'
);

CREATE TYPE offer_status AS ENUM (
    'submitted',
    'selected',
    'rejected'
);
