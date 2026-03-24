CREATE TABLE templates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL UNIQUE,
    channel     VARCHAR(20) NOT NULL CHECK (channel IN ('sms', 'email', 'push')),
    subject     VARCHAR(500),
    content     TEXT NOT NULL,
    variables   JSONB NOT NULL DEFAULT '[]',
    active      BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE notifications ADD CONSTRAINT fk_notifications_template
    FOREIGN KEY (template_id) REFERENCES templates(id);
