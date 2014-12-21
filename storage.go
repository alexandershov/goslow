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

// regexp POSTGRES_PLACEHOLDERS matches strings '$1'', '$2', '$3', ...
var POSTGRES_PLACEHOLDERS *regexp.Regexp = regexp.MustCompile("\\$\\d+")

// Storage is a window to the SQL world.
type Storage struct {
	driver     string
	dataSource string
	db         *sql.DB
}

// NewStorage returns a new storage with the given driver and dataSource.
func NewStorage(driver string, dataSource string) (*Storage, error) {
	db, err := sql.Open(driver, dataSource)
	if err != nil {
		return nil, err
	}
	storage := &Storage{driver: driver, dataSource: dataSource, db: db}
	_, err = storage.db.Exec(storage.dialectifySchema(CREATE_SCHEMA_IF_NOT_EXISTS_SQL))
	return storage, err
}

// Storage.FindEndpoint returns a endpoint matching a given site and HTTP request.
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
	rows, err := storage.db.Query(storage.dialectify(GET_SITE_ENDPOINTS_SQL), site)
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

func (storage *Storage) dialectify(sql string) string {
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
	// it's not pretty, but it covered by tests
	return strings.Replace(sql, " BYTEA,", " BLOB,", -1)
}

func makeEndpoint(rows *sql.Rows) (*Endpoint, error) {
	endpoint := new(Endpoint)
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

// Storage.SaveEndpoint saves a given endpoint into the database.
// If the endpoint doesn't exist in a database, Storage.SaveEndpoint creates a new endpoint.
func (storage *Storage) SaveEndpoint(endpoint *Endpoint) error {
	tx, err := storage.db.Begin()
	if err != nil {
		return err
	}
	// if tx gets commited then tx.Rollback() basically has no effect
	// if there's some error, then we always want to rollback
	defer tx.Rollback()
	// upsert as delete-n-insert isn't correct in all cases
	// (e.g concurrent upserts of the same endpoint will lead to "duplicate key value violates unique constraint")
	// but is practical enough because concurrent upserts of the same endpoint are going to be extremely rare
	_, err = tx.Exec(storage.dialectify(DELETE_ENDPOINT_SQL), endpoint.Site, endpoint.Path, endpoint.Method)
	if err != nil {
		return err
	}
	headersJson, err := stringMapToJson(endpoint.Headers)
	if err != nil {
		return err
	}
	_, err = tx.Exec(storage.dialectify(INSERT_ENDPOINT_SQL), endpoint.Site, endpoint.Path, endpoint.Method,
		headersJson, int64(endpoint.Delay), endpoint.StatusCode, endpoint.Response)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func stringMapToJson(m map[string]string) (js string, err error) {
	b, err := json.Marshal(m)
	return string(b), err
}

// Storage.CreateSite creates a new site.
// Storage.CreateSite returns an error if the given site already exists in a database.
func (storage *Storage) CreateSite(site string) error {
	_, err := storage.db.Exec(storage.dialectify(INSERT_SITE_SQL), site)
	return err
}

// Storage.SiteExists returns true if the given site exists in a database.
func (storage *Storage) SiteExists(site string) (bool, error) {
	rows, err := storage.db.Query(storage.dialectify(GET_SITE_SQL), site)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	siteExists := rows.Next()
	return siteExists, rows.Err()
}
