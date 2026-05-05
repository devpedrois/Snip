ALTER TABLE urls
  ADD COLUMN vt_status     ENUM('pending','clean','malicious','unverified') NOT NULL DEFAULT 'pending' AFTER expires_at,
  ADD COLUMN vt_scanned_at TIMESTAMP NULL AFTER vt_status,
  ADD COLUMN vt_positives  TINYINT UNSIGNED NULL AFTER vt_scanned_at,
  ADD COLUMN vt_permalink  VARCHAR(512) NULL AFTER vt_positives,
  ADD INDEX  idx_vt_status (vt_status);
