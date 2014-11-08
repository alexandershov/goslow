CREATE TABLE sites (
  site TEXT PRIMARY KEY
);

CREATE TABLE rules (
  site TEXT, path TEXT, method TEXT, header TEXT,
  delay BIGINT, response_status INT, response TEXT,
  PRIMARY KEY(site, path, method),
  FOREIGN KEY(site) REFERENCES sites(site)
);
