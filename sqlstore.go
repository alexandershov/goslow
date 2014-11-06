package main

import (
	"database/sql"
	"encoding/json"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"regexp"
	"time"
)

var AGNOSTIC_SQL *regexp.Regexp = regexp.MustCompile("\\$\\d+")

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
	Driver   string
	DataSource  string
	Db *sql.DB
}

func NewSqlStore(driver string, dataSource string) Store {
	db, err := sql.Open(driver, dataSource)
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
	return &SqlStore{Driver: driver, DataSource: dataSource, Db: db}
}

func (store *SqlStore) AddRule(rule *Rule) {
	_, err := store.Db.Exec(store.Agnostic(INSERT_RULE_SQL), rule.Host, rule.Path, rule.Method,
		MapToJson(rule.Header), rule.Delay, rule.ResponseStatus, rule.Response)
	if err != nil {
		log.Fatal(err)
	}
}

func (store *SqlStore) Agnostic(sql string) string {
	if store.Driver == "postgres" {
		return sql
	}
	return AGNOSTIC_SQL.ReplaceAllString(sql, "?")
}

func (store *SqlStore) GetHostRules(host string) []*Rule {
	rows, err := store.Db.Query(store.Agnostic(GET_RULES_SQL), host)
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
	var header string
	var delay int64
	rows.Scan(&rule.Host, &rule.Path, &rule.Method, &header, &delay, &rule.ResponseStatus,
		&rule.Response)
	rule.Header = JsonToMap(header)
	log.Println(rule.Header)
	rule.Delay = time.Duration(delay)
	return rule
}

func JsonToMap(j string) map[string]string {
	parsed := make(map[string]interface{})
	err := json.Unmarshal([]byte(j), &parsed)
	if err != nil {
		log.Fatal(err)
	}
	m := make(map[string]string)
	for key, value := range parsed {
		switch t := value.(type) {
		case string:
			m[key] = value.(string)
		default:
			log.Fatal("Expecting string, got %T", t)
		}
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

func MapToJson(m map[string]string) string {
	b, _ := json.Marshal(m)
	return string(b)
}

func MapToHeader(m map[string]string) http.Header {
	header := make(map[string][]string)
	for key, value := range m {
		header[key] = []string{value}
	}
	return header
}
