CREATE TABLE dead_letter_queue (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id),
    reason          TEXT NOT NULL,
    last_error      TEXT,
    payload         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reprocessed_at  TIMESTAMPTZ
);

CREATE INDEX idx_dlq_notification ON dead_letter_queue(notification_id);
CREATE INDEX idx_dlq_created_at ON dead_letter_queue(created_at);
