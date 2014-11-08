CREATE TABLE sites (
  site TEXT PRIMARY KEY
);

CREATE TABLE rules (
  site TEXT, path TEXT, method TEXT, headers TEXT,
  delay BIGINT, response_status INT, response_body TEXT,
  PRIMARY KEY(site, path, method),
  FOREIGN KEY(site) REFERENCES sites(site)
);
