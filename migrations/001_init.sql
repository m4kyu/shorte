CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS links (
  id BIGSERIAL PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  owner_user_id BIGINT NOT NULL REFERENCES users(id),
  long_url TEXT NOT NULL,
  is_active BOOLEAN NOT NULL DEFAULT true,
  expires_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_links_owner_created_at ON links(owner_user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_links_active_expires ON links(is_active, expires_at);

CREATE TABLE IF NOT EXISTS link_daily_stats (
  code TEXT NOT NULL,
  day DATE NOT NULL,
  click_count BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (code, day)
);
