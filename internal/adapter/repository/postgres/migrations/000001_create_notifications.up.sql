CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    batch_id        UUID,
    idempotency_key VARCHAR(255) UNIQUE,
    channel         VARCHAR(20) NOT NULL CHECK (channel IN ('sms', 'email', 'push')),
    recipient       VARCHAR(500) NOT NULL,
    subject         VARCHAR(500),
    content         TEXT NOT NULL,
    priority        SMALLINT NOT NULL DEFAULT 1 CHECK (priority IN (0, 1, 2)),
    status          VARCHAR(20) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','scheduled','queued','processing','delivered','failed','cancelled')),
    scheduled_at    TIMESTAMPTZ,
    sent_at         TIMESTAMPTZ,
    provider_msg_id VARCHAR(255),
    retry_count     SMALLINT NOT NULL DEFAULT 0,
    max_retries     SMALLINT NOT NULL DEFAULT 3,
    next_retry_at   TIMESTAMPTZ,
    metadata        JSONB DEFAULT '{}',
    template_id     UUID,
    template_vars   JSONB DEFAULT '{}',
    error_message   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_batch_id ON notifications(batch_id) WHERE batch_id IS NOT NULL;
CREATE INDEX idx_notifications_scheduled ON notifications(scheduled_at)
    WHERE status = 'scheduled' AND scheduled_at IS NOT NULL;
CREATE INDEX idx_notifications_retry ON notifications(next_retry_at)
    WHERE status = 'failed' AND next_retry_at IS NOT NULL AND retry_count < max_retries;
CREATE INDEX idx_notifications_channel_status ON notifications(channel, status);
CREATE INDEX idx_notifications_created_at ON notifications(created_at);
