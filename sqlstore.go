package main

import (
	"database/sql"
	_ "github.com/lib/pq"
	"github.com/lib/pq/hstore"
	"log"
	"net/http"
	"time"
)

const INSERT_RULE_SQL = `
INSERT INTO rules
(host, path, method, header, delay, response_status, response)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`

const GET_RULES_SQL = `
SELECT host, path, method, header, delay, response_status, response
FROM rules
WHERE host = $1
ORDER BY path DESC
`

type SqlStore struct {
	Db *sql.DB
}

func NewSqlStore(uri string) Store {
	db, err := sql.Open("postgres", uri)
	if err != nil {
		log.Fatal(err)
	}
	return &SqlStore{Db: db}
}

func (store *SqlStore) AddRule(rule *Rule) {
	_, err := store.Db.Query(INSERT_RULE_SQL, rule.Host, rule.Path, rule.Method,
		MapToHstore(rule.Header), rule.Delay, rule.ResponseStatus, rule.Response)
	if err != nil {
		log.Fatal(err)
	}
}

func (store *SqlStore) GetHostRules(host string) []*Rule {
	rows, err := store.Db.Query(GET_RULES_SQL, host)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	rules := make([]*Rule, 0)
	for rows.Next() {
		rules = append(rules, ReadRule(rows))
	}
	return rules
}

func ReadRule(rows *sql.Rows) *Rule {
	rule := new(Rule)
	header := hstore.Hstore{make(map[string]sql.NullString)}
	var delay int64
	rows.Scan(&rule.Host, &rule.Path, &rule.Method, &header, &delay, &rule.ResponseStatus,
		&rule.Response)
	rule.Header = HstoreToMap(header)
	rule.Delay = time.Duration(delay)
	return rule
}

func HstoreToMap(hs hstore.Hstore) map[string]string {
	m := make(map[string]string)
	for key, value := range hs.Map {
		if !value.Valid {
			log.Fatalf("NULL value for hstore key <%s>", key)
		}
		m[key] = value.String
	}
	return m
}

func HeaderToMap(header http.Header) map[string]string {
	m := make(map[string]string)
	for key, values := range header {
		if len(values) != 1 {
			log.Fatalf("multiple values %s for header key %s", key, values)
		}
		m[key] = values[0]
	}
	return m
}

func MapToHstore(m map[string]string) hstore.Hstore {
	nullMap := make(map[string]sql.NullString)
	for key, value := range m {
		nullMap[key] = sql.NullString{String: value, Valid: true}
	}
	return hstore.Hstore{nullMap}
}

func MapToHeader(m map[string]string) http.Header {
	header := make(map[string][]string)
	for key, value := range m {
		header[key] = []string{value}
	}
	return header
}
