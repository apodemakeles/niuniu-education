-- 002_student_word_task.sql
-- 学生端背单词任务前台。对齐 design/docs/student-word-task.md。
--   - word_learning:  单词词组学习主记录（1:1 关联 words），承载学习状态/排程字段。
--   - daily_tasks:    今日任务明细（每词每天一条），锁定当天首次默写结果。
--   - reading_passages: 延伸阅读 AI 短文（每天一条）。
--   - checkins:       打卡记录（每自然日一条）。
--   - review_strategy: 排程策略参数（单行 id=1），对齐模拟推荐参数。

-- 单词词组学习主记录。words 表本身不改（家长端字段范围不变），
-- 学习进度/排程独立存储，避免污染跨天复习。
CREATE TABLE word_learning (
    word_id               TEXT PRIMARY KEY REFERENCES words(id) ON DELETE CASCADE,
    library_id            TEXT NOT NULL DEFAULT 'main-library',
    learning_status       TEXT NOT NULL DEFAULT 'unlearned'
                          CHECK (learning_status IN ('unlearned','learning','reinforce','mastered')),
    success_count         INTEGER NOT NULL DEFAULT 0,        -- 有效成功次数，封顶 5
    studied_dates         TEXT NOT NULL DEFAULT '[]',         -- JSON 数组，记录学习过的日期(YYYY-MM-DD)
    success_dates         TEXT NOT NULL DEFAULT '[]',         -- JSON 数组，有效成功日期
    last_studied_date     TEXT,                               -- YYYY-MM-DD
    last_success_date     TEXT,                               -- YYYY-MM-DD
    next_due_date         TEXT,                               -- YYYY-MM-DD；未学/无值为空
    mastered_review_index INTEGER NOT NULL DEFAULT 0,         -- 已掌握抽查阶段 0/1/2/3 -> 60/90/150/240 天
    first_mastered_date   TEXT,                               -- 首次达到已掌握日期
    updated_at            TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_word_learning_status_due
    ON word_learning(library_id, learning_status, next_due_date);

-- 今日任务明细：某天某个单词词组的任务记录。锁定当天首次默写结果。
CREATE TABLE daily_tasks (
    id                       TEXT PRIMARY KEY,
    task_date                TEXT NOT NULL,                   -- YYYY-MM-DD
    word_id                  TEXT NOT NULL REFERENCES words(id) ON DELETE CASCADE,
    library_id               TEXT NOT NULL DEFAULT 'main-library',
    pool_type                TEXT NOT NULL                    -- first(新词池) / review(非新词池)
                             CHECK (pool_type IN ('first','review')),
    display_order            INTEGER NOT NULL DEFAULT 0,      -- 任务生成后固定，不随状态变化
    read_status              TEXT NOT NULL DEFAULT 'not_started'
                             CHECK (read_status IN ('not_started','listened','read_done','hard')),
    first_dictation_answer   TEXT NOT NULL DEFAULT '',        -- 首次默写答案
    first_dictation_result   TEXT NOT NULL DEFAULT 'pending'  -- pending/correct/wrong/blank
                             CHECK (first_dictation_result IN ('pending','correct','wrong','blank')),
    used_hint                INTEGER NOT NULL DEFAULT 0,      -- 0/1
    submission_locked        INTEGER NOT NULL DEFAULT 0,      -- 首次默写提交后 1，回看与订正不改首次结果
    correction_answer        TEXT NOT NULL DEFAULT '',        -- 订正答案
    correction_result        TEXT NOT NULL DEFAULT 'not_submitted' -- not_submitted/correct/wrong/blank
                             CHECK (correction_result IN ('not_submitted','correct','wrong','blank')),
    created_at               TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at               TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE (task_date, word_id)
);

CREATE INDEX idx_daily_tasks_date ON daily_tasks(library_id, task_date, display_order);

-- 延伸阅读 AI 短文：一天一条。
CREATE TABLE reading_passages (
    task_date              TEXT PRIMARY KEY,                  -- YYYY-MM-DD
    library_id             TEXT NOT NULL DEFAULT 'main-library',
    reading_title          TEXT NOT NULL DEFAULT '',
    reading_text           TEXT NOT NULL DEFAULT '',          -- 80~140 英文词
    covered_word_ids       TEXT NOT NULL DEFAULT '[]',        -- JSON: 今日任务词 id 数组
    word_occurrence_counts TEXT NOT NULL DEFAULT '{}',        -- JSON: {wordId: 出现次数}
    reading_scene_hint     TEXT NOT NULL DEFAULT '',          -- 中文场景提示
    ai_generation_status   TEXT NOT NULL DEFAULT 'pending'   -- pending/success/failed
                           CHECK (ai_generation_status IN ('pending','success','failed')),
    reading_started_at     TEXT,                              -- 进入阅读页时间(ISO)
    reading_completed_at   TEXT,                              -- 点击读完时间(ISO)
    reading_min_seconds    INTEGER NOT NULL DEFAULT 120,
    created_at             TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at             TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 打卡记录：每自然日一条。
CREATE TABLE checkins (
    task_date   TEXT PRIMARY KEY,                             -- YYYY-MM-DD
    library_id  TEXT NOT NULL DEFAULT 'main-library',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- 排程策略参数：单行 id=1。默认值对齐 PRD 模拟推荐参数。
CREATE TABLE review_strategy (
    id                          INTEGER PRIMARY KEY CHECK (id = 1),
    new_pool_count              INTEGER NOT NULL DEFAULT 3,
    review_pool_max_count       INTEGER NOT NULL DEFAULT 9,
    review_pool_min_score       INTEGER NOT NULL DEFAULT 70,
    reinforce_base_score        INTEGER NOT NULL DEFAULT 90,
    learning_base_score         INTEGER NOT NULL DEFAULT 70,
    mastered_base_score         INTEGER NOT NULL DEFAULT 10,
    due_today_bonus             INTEGER NOT NULL DEFAULT 40,
    overdue_daily_bonus         INTEGER NOT NULL DEFAULT 8,
    overdue_bonus_cap           INTEGER NOT NULL DEFAULT 56,
    mastered_due_sample_ratio   INTEGER NOT NULL DEFAULT 1,   -- 百分比，1 表示 1%
    mastered_sample_stop_day    INTEGER NOT NULL DEFAULT 120, -- 学期第 N 天后停止抽查已掌握
    reading_min_seconds         INTEGER NOT NULL DEFAULT 120,
    updated_at                  TEXT NOT NULL DEFAULT (datetime('now'))
);

INSERT INTO review_strategy(id) VALUES (1)
    ON CONFLICT(id) DO NOTHING;

-- 记录迁移版本（迁移框架据此判幂等）
INSERT INTO schema_migrations(version) VALUES ('002_student_word_task.sql')
    ON CONFLICT(version) DO NOTHING;
