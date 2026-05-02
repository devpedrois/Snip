CREATE TABLE urls (
  id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  hash            VARCHAR(10) NOT NULL UNIQUE,
  original_url    TEXT NOT NULL,
  created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  last_accessed_at TIMESTAMP NULL,
  expires_at      TIMESTAMP NULL,
  INDEX idx_hash (hash),
  INDEX idx_expires (expires_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
