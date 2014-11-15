package main

import (
	"flag"
	"log"
)

type Config struct {
	listenOn            string
	endpoint            string
	driver              string
	dataSource          string
	minSiteLength       int
	siteSalt            string
	createDefaultRules  bool
	singleDomainUrlPath string // TODO: fix bad name
}

var DEFAULT_CONFIG = Config{
	listenOn:            ":5103",
	endpoint:            "localhost:5103",
	driver:              "sqlite3",
	dataSource:          ":memory:",
	minSiteLength:       6,
	siteSalt:            "",
	createDefaultRules:  false,
	singleDomainUrlPath: "/goslow",
}

func NewConfigFromArgs() *Config {
	config := new(Config)
	config.defineFlags()
	flag.Parse()
	config.validate()
	return config
}

func (config *Config) defineFlags() {
	flag.StringVar(&config.listenOn, "listen-on", DEFAULT_CONFIG.listenOn,
		"address to listen on. E.g: 0.0.0.0:8000")
	flag.StringVar(&config.endpoint, "endpoint", DEFAULT_CONFIG.endpoint,
		`url at which this instance of goslow is visible to the world.
	Used only in response texts, doesn't affect the listening address. E.g: goslow.link`)
	flag.StringVar(&config.driver, "driver", DEFAULT_CONFIG.driver,
		"database driver. Possible values: sqlite3, postgres.")
	// TODO: is db a better name for cmd arg?
	flag.StringVar(&config.dataSource, "data-source", DEFAULT_CONFIG.dataSource,
		`data source name. E.g: postgres://user:password@localhost/dbname for postgres
	or /path/to/sqlite3/db for sqlite3`)
	flag.IntVar(&config.minSiteLength, "min-site-length", DEFAULT_CONFIG.minSiteLength,
		"minimum length of the randomly generated site names. E.g: 8")
	flag.StringVar(&config.siteSalt, "site-salt", DEFAULT_CONFIG.siteSalt,
		"random names generator salt. Keep it secret. E.g: kj8ioIxZ")
	flag.BoolVar(&config.createDefaultRules, "create-default-rules", DEFAULT_CONFIG.createDefaultRules,
		"Create default rules?")
	flag.StringVar(&config.singleDomainUrlPath, "single-domain-url-path", DEFAULT_CONFIG.singleDomainUrlPath,
		`If not an empty string: run in single domain mode
	and use url -listen-on/-single-domain-url-path for configuration`)
}

func (config *Config) validate() {
	if config.createDefaultRules && config.singleDomainUrlPath != "" {
		log.Fatal("You can't use both --single-domain-url-path and --create-default-rules options")
	}

}
