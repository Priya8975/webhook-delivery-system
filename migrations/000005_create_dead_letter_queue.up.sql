CREATE TABLE dead_letter_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id),
    subscriber_id UUID NOT NULL REFERENCES subscribers(id),
    total_attempts INT NOT NULL,
    last_error TEXT,
    last_http_status INT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    resolved_by VARCHAR(100)
);

CREATE INDEX idx_dlq_unresolved ON dead_letter_queue(created_at) WHERE resolved_at IS NULL;
CREATE INDEX idx_dlq_subscriber ON dead_letter_queue(subscriber_id);
