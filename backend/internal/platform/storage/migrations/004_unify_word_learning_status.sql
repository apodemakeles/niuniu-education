-- 004_unify_word_learning_status.sql
-- words.status 是早期词库字段；学习状态从此统一由 word_learning 承载。
-- 对尚无学习记录的历史非未学词做一次迁移，已有真实学习记录绝不覆盖。

INSERT INTO word_learning(
    word_id, library_id, learning_status, success_count, next_due_date, first_mastered_date
)
SELECT
    id,
    library_id,
    status,
    CASE WHEN status = 'mastered' THEN 3 ELSE 0 END,
    CASE
        WHEN status IN ('learning', 'reinforce') THEN date('now', 'localtime')
        WHEN status = 'mastered' THEN date('now', 'localtime', '+60 days')
        ELSE NULL
    END,
    CASE WHEN status = 'mastered' THEN date('now', 'localtime') ELSE NULL END
FROM words
WHERE deleted_at IS NULL
  AND status <> 'unlearned'
  AND id NOT IN (SELECT word_id FROM word_learning);

INSERT INTO schema_migrations(version) VALUES ('004_unify_word_learning_status.sql')
    ON CONFLICT(version) DO NOTHING;
