CREATE TABLE documents (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    type          document_type NOT NULL,
    file_url      text NOT NULL,
    original_name text NOT NULL,
    uploaded_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_documents_user_id ON documents (user_id);
