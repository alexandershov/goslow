package main

// To make queries work with both sqlite3 and postgres:
// string " BYTEA," is replaced with " BLOB," in DDL statements
// strings "$1", "$2", "$3", ... are replaced with "? in DML statements
// when using sqlite3 driver.

const (
	CREATE_SCHEMA_IF_NOT_EXISTS_SQL = `
CREATE TABLE IF NOT EXISTS sites(
  site TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS endpoints (
  site        TEXT,
	path        TEXT,
	method      TEXT,
	headers     TEXT,
  delay       BIGINT,
	status_code INT,
	response    BYTEA,
  PRIMARY KEY(site, path, method),
  FOREIGN KEY(site) REFERENCES sites(site)
);
`

	DELETE_ENDPOINT_SQL = `
DELETE FROM endpoints
WHERE site = $1 AND path = $2 AND method = $3
`

	INSERT_ENDPOINT_SQL = `
INSERT INTO endpoints
(site, path, method, headers, delay, status_code, response)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

	GET_SITE_ENDPOINTS_SQL = `
SELECT site, path, method, headers, delay, status_code, response
FROM endpoints
WHERE site = $1
ORDER BY LENGTH(path) DESC, LENGTH(method) DESC
`

	INSERT_SITE_SQL = `
INSERT INTO sites
(site)
VALUES ($1)
`

	GET_SITE_SQL = `
SELECT site
FROM sites
WHERE site = $1
`
)
