ALTER TABLE urls
  DROP INDEX idx_vt_status,
  DROP COLUMN vt_permalink,
  DROP COLUMN vt_positives,
  DROP COLUMN vt_scanned_at,
  DROP COLUMN vt_status;
