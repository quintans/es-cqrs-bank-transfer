CREATE TABLE IF NOT EXISTS events(
    id VARCHAR (50) PRIMARY KEY,
    aggregate_id VARCHAR (50) NOT NULL,
    aggregate_version INTEGER NOT NULL,
    aggregate_type VARCHAR (50) NOT NULL,
    kind VARCHAR (50) NOT NULL,
    body JSONB NOT NULL,
    idempotency_key VARCHAR (50),
    labels JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()::TIMESTAMP,
    UNIQUE (aggregate_id, aggregate_version)
);
CREATE INDEX aggregate_idx ON events (aggregate_id, aggregate_version);
CREATE INDEX idempotency_key_idx ON events (idempotency_key, aggregate_id);
CREATE INDEX labels_idx ON events USING GIN (labels jsonb_path_ops);

CREATE TABLE IF NOT EXISTS snapshots(
    id VARCHAR (50) PRIMARY KEY,
    aggregate_id VARCHAR (50) NOT NULL,
    aggregate_version INTEGER NOT NULL,
    body JSONB NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()::TIMESTAMP,
    FOREIGN KEY (id) REFERENCES events (id)
    );
    CREATE INDEX aggregate_id_idx ON snapshots (aggregate_id);

CREATE OR REPLACE FUNCTION notify_event() RETURNS TRIGGER AS $FN$
    DECLARE 
        notification json;
    BEGIN
        notification = row_to_json(NEW);
        PERFORM pg_notify('events_channel', notification::text);
        
        -- Result is ignored since this is an AFTER trigger
        RETURN NULL; 
    END;
$FN$ LANGUAGE plpgsql;

CREATE TRIGGER events_notify_event
AFTER INSERT ON events
    FOR EACH ROW EXECUTE PROCEDURE notify_event();
