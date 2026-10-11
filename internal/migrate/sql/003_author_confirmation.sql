-- Author confirmation is separate from approval. A reviewer decision does not
-- queue device writes. The author sets this flag by confirming the plan hash.
ALTER TABLE change_requests
    ADD COLUMN author_confirmed boolean NOT NULL DEFAULT false,
    ADD COLUMN confirmed_by uuid REFERENCES users (id),
    ADD COLUMN confirmed_at timestamptz;
