-- +goose Up
ALTER TABLE oauth_states
    ALTER COLUMN verifier_ciphertext DROP NOT NULL,
    ADD COLUMN client_kind text NOT NULL DEFAULT 'browser'
        CHECK (client_kind IN ('browser', 'mobile')),
    ADD COLUMN mobile_pkce_challenge text;

ALTER TABLE oauth_states
    ADD CONSTRAINT oauth_states_pkce_shape CHECK (
        (client_kind = 'browser' AND verifier_ciphertext IS NOT NULL AND mobile_pkce_challenge IS NULL)
        OR
        (client_kind = 'mobile' AND verifier_ciphertext IS NULL AND mobile_pkce_challenge IS NOT NULL)
    );

-- +goose Down
ALTER TABLE oauth_states DROP CONSTRAINT IF EXISTS oauth_states_pkce_shape;
ALTER TABLE oauth_states DROP COLUMN IF EXISTS mobile_pkce_challenge;
ALTER TABLE oauth_states DROP COLUMN IF EXISTS client_kind;
DELETE FROM oauth_states WHERE verifier_ciphertext IS NULL;
ALTER TABLE oauth_states ALTER COLUMN verifier_ciphertext SET NOT NULL;
