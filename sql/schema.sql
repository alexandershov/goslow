CREATE EXTENSION hstore;

CREATE TABLE rules (host TEXT, path TEXT, method TEXT, header HSTORE, delay BIGINT, response_status INT, response TEXT);

