package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// regexp POSTGRES_PLACEHOLDERS matches strings '$1', '$2', '$3', ...
var POSTGRES_PLACEHOLDERS *regexp.Regexp = regexp.MustCompile("\\$\\d+")

// Storage is the interface to the SQL database.
type Storage struct {
	driver     string
	dataSource string
	db         *sql.DB
}

// TODO: move call to CREATE_SCHEMA_IF_NOT_EXISTS_SQL to NewServer
func NewStorage(driver string, dataSource string) (*Storage, error) {
	db, err := sql.Open(driver, dataSource)
	if err != nil {
		return nil, err
	}
	storage := &Storage{driver: driver, dataSource: dataSource, db: db}
	_, err = storage.db.Exec(storage.dialectifySchema(CREATE_SCHEMA_IF_NOT_EXISTS_SQL))
	return storage, err
}

// Storage.FindEndpoint returns an endpoint matching the given site and HTTP request.
func (storage *Storage) FindEndpoint(site string, req *http.Request) (endpoint *Endpoint, found bool, err error) {
	endpoints, err := storage.getEndpoints(site)
	if err != nil {
		return nil, false, err
	}
	for _, endpoint := range endpoints {
		if endpoint.Matches(req) {
			return endpoint, true, nil
		}
	}
	return nil, false, nil
}

func (storage *Storage) getEndpoints(site string) ([]*Endpoint, error) {
	endpoints := make([]*Endpoint, 0)
	rows, err := storage.db.Query(storage.dialectifyQuery(GET_SITE_ENDPOINTS_SQL), site)
	if err != nil {
		return endpoints, err
	}
	defer rows.Close()

	for rows.Next() {
		endpoint, err := makeEndpoint(rows)
		if err != nil {
			return endpoints, err
		}
		endpoints = append(endpoints, endpoint)
	}
	return endpoints, rows.Err()
}

func (storage *Storage) dialectifyQuery(sql string) string {
	if storage.isPostgres() {
		return sql
	}
	return POSTGRES_PLACEHOLDERS.ReplaceAllString(sql, "?")
}

func (storage *Storage) isPostgres() bool {
	return storage.driver == "postgres"
}

func (storage *Storage) dialectifySchema(sql string) string {
	if storage.isPostgres() {
		return sql
	}
	// it's not pretty, but it's covered by tests
	REPLACE_ALL := -1
	return strings.Replace(sql, " BYTEA,", " BLOB,", REPLACE_ALL)
}

func makeEndpoint(rows *sql.Rows) (*Endpoint, error) {
	endpoint := &Endpoint{}
	var headersJson string
	var delay int64
	err := rows.Scan(&endpoint.Site, &endpoint.Path, &endpoint.Method, &headersJson, &delay,
		&endpoint.StatusCode, &endpoint.Response)
	if err != nil {
		return endpoint, err
	}
	endpoint.Delay = time.Duration(delay)
	endpoint.Headers, err = jsonToStringMap(headersJson)
	return endpoint, err
}

func jsonToStringMap(js string) (map[string]string, error) {
	object := make(map[string]interface{})
	err := json.Unmarshal([]byte(js), &object)
	if err != nil {
		return nil, err
	}
	return objectToStringMap(object)
}

func objectToStringMap(object map[string]interface{}) (map[string]string, error) {
	m := make(map[string]string)
	for key, value := range object {
		switch value.(type) {
		case string:
			m[key] = value.(string)
		default:
			return nil, fmt.Errorf("Expecting string, got %+v", value)
		}
	}
	return m, nil
}

// Storage.SaveEndpoint upserts the given endpoint into a database.
func (storage *Storage) SaveEndpoint(endpoint *Endpoint) error {
	tx, err := storage.db.Begin()
	if err != nil {
		return err
	}
	// If tx is commited, then tx.Rollback() basically has no effect.
	// If there's some error and tx isn't commited, then we want to rollback.
	defer tx.Rollback()
	// upsert as delete-and-insert isn't correct in all cases
	// (e.g concurrent upserts of the same endpoint will lead to "duplicate key value violates unique constraint")
	// but is practical enough, because concurrent upserts of the same endpoint are going to be extremely rare
	_, err = tx.Exec(storage.dialectifyQuery(DELETE_ENDPOINT_SQL),
		endpoint.Site, endpoint.Path, endpoint.Method)
	if err != nil {
		return err
	}
	headersJson, err := stringMapToJson(endpoint.Headers)
	if err != nil {
		return err
	}
	_, err = tx.Exec(storage.dialectifyQuery(INSERT_ENDPOINT_SQL),
		endpoint.Site, endpoint.Path, endpoint.Method,
		headersJson, int64(endpoint.Delay), endpoint.StatusCode, endpoint.Response)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func stringMapToJson(m map[string]string) (string, error) {
	jsonBytes, err := json.Marshal(m)
	return string(jsonBytes), err
}

// Storage.CreateSite returns an error if the given site already exists in a database.
func (storage *Storage) CreateSite(site string) error {
	_, err := storage.db.Exec(storage.dialectifyQuery(INSERT_SITE_SQL), site)
	return err
}

// TODO: generalize (e.g storage.HasResults(sql string))
func (storage *Storage) SiteExists(site string) (bool, error) {
	rows, err := storage.db.Query(storage.dialectifyQuery(GET_SITE_SQL), site)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	siteExists := rows.Next()
	return siteExists, rows.Err()
}
