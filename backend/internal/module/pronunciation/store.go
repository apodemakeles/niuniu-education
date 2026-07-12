package pronunciation

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrCacheMiss = errors.New("发音缓存不存在")

type Record struct {
	WordID, Locale, Status, Provider, Phonetic, FilePath, MIMEType string
	SourceURL, LicenseName, LicenseURL, Attribution                string
	RetryAfter                                                     time.Time
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

func (s *Store) Get(ctx context.Context, wordID, locale string) (Record, error) {
	var r Record
	var retry sql.NullTime
	err := s.db.QueryRowContext(ctx, `SELECT word_id,locale,status,provider,phonetic,file_path,mime_type,source_url,license_name,license_url,attribution,retry_after FROM pronunciation_audio WHERE word_id=? AND locale=?`, wordID, locale).
		Scan(&r.WordID, &r.Locale, &r.Status, &r.Provider, &r.Phonetic, &r.FilePath, &r.MIMEType, &r.SourceURL, &r.LicenseName, &r.LicenseURL, &r.Attribution, &retry)
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrCacheMiss
	}
	if err != nil {
		return Record{}, err
	}
	if retry.Valid {
		r.RetryAfter = retry.Time
	}
	return r, nil
}

func (s *Store) UpsertReady(ctx context.Context, r Record) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO pronunciation_audio(word_id,locale,status,provider,phonetic,file_path,mime_type,source_url,license_name,license_url,attribution,checked_at,retry_after,updated_at)
	VALUES(?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP,NULL,CURRENT_TIMESTAMP)
	ON CONFLICT(word_id,locale) DO UPDATE SET status='ready',provider=excluded.provider,phonetic=excluded.phonetic,file_path=excluded.file_path,mime_type=excluded.mime_type,source_url=excluded.source_url,license_name=excluded.license_name,license_url=excluded.license_url,attribution=excluded.attribution,checked_at=CURRENT_TIMESTAMP,retry_after=NULL,updated_at=CURRENT_TIMESTAMP`, r.WordID, r.Locale, "ready", r.Provider, r.Phonetic, r.FilePath, r.MIMEType, r.SourceURL, r.LicenseName, r.LicenseURL, r.Attribution)
	return err
}
func (s *Store) UpsertMissing(ctx context.Context, wordID, locale string, retry time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO pronunciation_audio(word_id,locale,status,retry_after,checked_at,updated_at) VALUES(?,?, 'missing',?,CURRENT_TIMESTAMP,CURRENT_TIMESTAMP)
	ON CONFLICT(word_id,locale) DO UPDATE SET status='missing',retry_after=excluded.retry_after,checked_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP`, wordID, locale, retry)
	return err
}
