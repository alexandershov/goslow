CREATE TABLE rules (
  host TEXT, path TEXT, method TEXT, header TEXT,
  delay BIGINT, response_status INT, response TEXT,
  PRIMARY KEY(host, path, method)
);

CREATE TABLE domains (
  domain TEXT PRIMARY KEY
);

