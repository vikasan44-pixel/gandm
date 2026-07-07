CREATE TYPE participant_type AS ENUM (
    'client',
    'warehouse',
    'carrier',
    'driver',
    'broker',
    'customs_rep'
);

CREATE TYPE user_status AS ENUM (
    'pending',
    'active',
    'blocked',
    'rejected'
);

CREATE TYPE document_type AS ENUM (
    'id_card',
    'founding_docs',
    'business_license',
    'employment_contract',
    'vehicle_doc'
);

CREATE TYPE verification_status AS ENUM (
    'pending',
    'approved',
    'rejected'
);

CREATE TYPE admin_role AS ENUM (
    'admin',
    'moderator'
);
