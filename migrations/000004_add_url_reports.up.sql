CREATE TABLE url_reports (
  id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
  url_id      BIGINT UNSIGNED NOT NULL,
  reporter_ip VARCHAR(45) NOT NULL,
  reason      ENUM('phishing','malware','spam','illegal','other') NOT NULL,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  FOREIGN KEY (url_id) REFERENCES urls(id) ON DELETE CASCADE,
  INDEX idx_url_id (url_id),
  UNIQUE KEY uk_url_reporter (url_id, reporter_ip)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
