CREATE TABLE sync_tracker (
    source       TEXT NOT NULL,
    key          TEXT NOT NULL,
    captured_at  TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (source, key)
);
