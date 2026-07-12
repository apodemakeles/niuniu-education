-- 006_word_examples.sql
-- 每个单词最多维护 3 条 AI 学习例句；独立表便于单句重生成和随机展示。

CREATE TABLE word_examples (
    id            TEXT PRIMARY KEY,
    word_id       TEXT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    sentence      TEXT NOT NULL,
    display_order INTEGER NOT NULL CHECK (display_order BETWEEN 0 AND 2),
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(word_id, display_order)
);

CREATE INDEX idx_word_examples_word_order ON word_examples(word_id, display_order);

INSERT INTO schema_migrations(version) VALUES ('006_word_examples.sql')
    ON CONFLICT(version) DO NOTHING;
