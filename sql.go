package main

// TODO: solve BYTEA/BLOB problem
const (
	CREATE_SCHEMA_IF_NOT_EXISTS_SQL = `
CREATE TABLE IF NOT EXISTS sites(
  site TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS rules (
  site TEXT, path TEXT, method TEXT, headers TEXT,
  delay BIGINT, response_status INT, response_body BYTEA,
  PRIMARY KEY(site, path, method),
  FOREIGN KEY(site) REFERENCES sites(site)
  -- TODO: check that foreign key works on postgres
);
`

	DELETE_RULE_SQL = `
DELETE FROM rules
WHERE site = $1 AND path = $2 AND method = $3
`

	CREATE_RULE_SQL = `
INSERT INTO rules
(site, path, method, headers, delay, response_status, response_body)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

	GET_SITE_RULES_SQL = `
SELECT site, path, method, headers, delay, response_status, response_body
FROM rules
WHERE site = $1
ORDER BY LENGTH(path) DESC, LENGTH(method) DESC
`

	CREATE_SITE_SQL = `
INSERT INTO sites
(site)
VALUES ($1)
`

	GET_SITE_SQL = `
SELECT *
FROM sites
WHERE site = $1
`
)
