CREATE TABLE clicks (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  url_id      BIGINT UNSIGNED NOT NULL,
  accessed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  user_agent  VARCHAR(512) NULL,
  ip          VARCHAR(45) NULL,
  FOREIGN KEY (url_id) REFERENCES urls(id) ON DELETE CASCADE,
  INDEX idx_url_id (url_id),
  INDEX idx_accessed_at (accessed_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
