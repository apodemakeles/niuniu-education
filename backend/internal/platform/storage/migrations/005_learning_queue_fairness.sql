-- 005_learning_queue_fairness.sql
-- 复习积压时收紧新词引入，避免持续加新词导致到期复习被挤压。

ALTER TABLE review_strategy ADD COLUMN review_overdue_reduce_new INTEGER NOT NULL DEFAULT 3;
ALTER TABLE review_strategy ADD COLUMN review_overdue_pause_new INTEGER NOT NULL DEFAULT 6;

INSERT INTO schema_migrations(version) VALUES ('005_learning_queue_fairness.sql')
    ON CONFLICT(version) DO NOTHING;
