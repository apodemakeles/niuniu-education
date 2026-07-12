-- 003_pronunciation_audio.sql
-- 单词发音缓存：保存录音来源、许可证与本地音频路径。

CREATE TABLE pronunciation_audio (
    word_id          TEXT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    locale           TEXT NOT NULL DEFAULT 'en-GB',
    status           TEXT NOT NULL CHECK (status IN ('ready','missing')),
    provider         TEXT NOT NULL DEFAULT '',
    phonetic         TEXT NOT NULL DEFAULT '',
    file_path        TEXT NOT NULL DEFAULT '',
    mime_type        TEXT NOT NULL DEFAULT '',
    source_url       TEXT NOT NULL DEFAULT '',
    license_name     TEXT NOT NULL DEFAULT '',
    license_url      TEXT NOT NULL DEFAULT '',
    attribution      TEXT NOT NULL DEFAULT '',
    checked_at       TEXT NOT NULL DEFAULT (datetime('now')),
    retry_after      TEXT,
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    PRIMARY KEY (word_id, locale)
);

CREATE INDEX idx_pronunciation_status_retry
    ON pronunciation_audio(status, retry_after);

INSERT INTO schema_migrations(version) VALUES ('003_pronunciation_audio.sql')
    ON CONFLICT(version) DO NOTHING;
