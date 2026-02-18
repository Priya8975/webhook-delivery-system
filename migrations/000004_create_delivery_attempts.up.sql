CREATE TABLE delivery_attempts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id),
    subscriber_id UUID NOT NULL REFERENCES subscribers(id),
    attempt_number INT NOT NULL DEFAULT 1,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    http_status_code INT,
    response_body TEXT,
    response_time_ms INT,
    error_message TEXT,
    next_retry_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE INDEX idx_delivery_status ON delivery_attempts(status);
CREATE INDEX idx_delivery_retry ON delivery_attempts(next_retry_at) WHERE status = 'failed';
CREATE INDEX idx_delivery_event ON delivery_attempts(event_id);
CREATE INDEX idx_delivery_subscriber ON delivery_attempts(subscriber_id);
