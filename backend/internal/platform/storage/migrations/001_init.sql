-- 001_init.sql
-- 单词库 MVP 初始 schema。字段范围严格对齐原型 app.js：
-- text / meaning_zh / phonetic / word_type / status (+ 软删与审计)。

-- 词库（MVP 固定单库，name 字段预留，不向用户暴露编辑）
CREATE TABLE libraries (
    id            TEXT PRIMARY KEY,
    name          TEXT NOT NULL DEFAULT '默认词库',
    created_at    TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 单词
CREATE TABLE words (
    id               TEXT PRIMARY KEY,
    library_id       TEXT NOT NULL REFERENCES libraries(id),
    text             TEXT NOT NULL,
    meaning_zh       TEXT NOT NULL DEFAULT '',
    phonetic         TEXT NOT NULL DEFAULT '',
    word_type        TEXT NOT NULL DEFAULT 'new'
                     CHECK (word_type IN ('new','mistake')),
    status           TEXT NOT NULL DEFAULT 'unlearned'
                     CHECK (status IN ('unlearned','learning','reinforce','mastered')),
    -- 学习进度预留（MVP 不写，留给孩子端）
    first_learned_at TEXT,
    last_reviewed_at TEXT,
    review_count     INTEGER NOT NULL DEFAULT 0,
    -- 审计
    created_at       TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at       TEXT NOT NULL DEFAULT (datetime('now')),
    last_edited_at   TEXT,
    source           TEXT NOT NULL DEFAULT 'manual',
    deleted_at       TEXT
);

-- 重复判定：词库内 text + meaning_zh 唯一（仅未软删行）
CREATE UNIQUE INDEX uniq_word_text_meaning
    ON words(library_id, text, meaning_zh)
    WHERE deleted_at IS NULL;

CREATE INDEX idx_words_type_status
    ON words(library_id, word_type, status)
    WHERE deleted_at IS NULL;

-- OCR 导入原图记录（拍照导入能力支撑，留底排查用）
CREATE TABLE import_images (
    id            TEXT PRIMARY KEY,
    library_id    TEXT NOT NULL REFERENCES libraries(id),
    file_path     TEXT NOT NULL,
    mime_type     TEXT NOT NULL,
    uploaded_at   TEXT NOT NULL DEFAULT (datetime('now')),
    ocr_raw_text  TEXT,
    provider      TEXT,
    status        TEXT NOT NULL DEFAULT 'pending'
);

-- Schema 版本表由迁移框架统一管理（见 storage.migrate），此处不再创建。

-- 默认词库（固定 id，便于 ensure 逻辑幂等）
INSERT INTO libraries(id, name) VALUES ('main-library', '默认词库')
    ON CONFLICT(id) DO NOTHING;

-- 预置原型 app.js 中的示例单词，方便前端联调首屏有内容
INSERT INTO words(id, library_id, text, meaning_zh, phonetic, word_type, status) VALUES
    ('w-1', 'main-library', 'apple',  '苹果',     '/ˈæpl/',    'new',     'unlearned'),
    ('w-2', 'main-library', 'read',   '阅读',     '/riːd/',    'mistake', 'reinforce'),
    ('w-3', 'main-library', 'desk',   '书桌',     '/desk/',    'new',     'learning'),
    ('w-4', 'main-library', 'climb',  '攀爬',     '/klaɪm/',   'mistake', 'reinforce'),
    ('w-5', 'main-library', 'water',  '水',       '/ˈwɔːtər/', 'new',     'mastered'),
    ('w-6', 'main-library', 'their',  '他们的',   '/ðer/',     'mistake', 'reinforce')
ON CONFLICT(id) DO NOTHING;
