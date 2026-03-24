CREATE TABLE delivery_attempts (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id UUID NOT NULL REFERENCES notifications(id),
    attempt_number  SMALLINT NOT NULL,
    status          VARCHAR(20) NOT NULL CHECK (status IN ('success', 'failure')),
    provider_msg_id VARCHAR(255),
    status_code     SMALLINT,
    response_body   TEXT,
    error_message   TEXT,
    duration_ms     INTEGER,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_delivery_attempts_notification ON delivery_attempts(notification_id);
