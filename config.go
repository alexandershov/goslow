package main

import (
	"flag"
	"log"
)

// Config stores command line arguments.
type Config struct {
	listenOn           string
	endpoint           string
	driver             string
	dataSource         string
	minSiteLength      int
	siteSalt           string
	createDefaultRules bool
	adminUrlPathPrefix string
}

var DEFAULT_CONFIG = Config{
	listenOn:           ":5103",
	endpoint:           "localhost:5103",
	driver:             "sqlite3",
	dataSource:         "file::memory:?cache=shared", // we need cache=shared in the case of reconnect which happens sometines under load
	minSiteLength:      6,
	siteSalt:           "",
	createDefaultRules: false,
	adminUrlPathPrefix: "/goslow",
}

// NewConfigFromArgs returns a new config from command line arguments.
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
	flag.StringVar(&config.dataSource, "data-source", DEFAULT_CONFIG.dataSource,
		`data source name. E.g: postgres://user:password@localhost/dbname for postgres
	or /path/to/sqlite3/db for sqlite3`)
	flag.IntVar(&config.minSiteLength, "min-site-length", DEFAULT_CONFIG.minSiteLength,
		"minimum length of the randomly generated site names. E.g: 8")
	flag.StringVar(&config.siteSalt, "site-salt", DEFAULT_CONFIG.siteSalt,
		"random names generator salt. Keep it secret. E.g: kj8ioIxZ")
	flag.BoolVar(&config.createDefaultRules, "create-default-rules", DEFAULT_CONFIG.createDefaultRules,
		"Create default rules?")
	flag.StringVar(&config.adminUrlPathPrefix, "admin-url-path-prefix", DEFAULT_CONFIG.adminUrlPathPrefix,
		`If not an empty string: run in single domain mode
	and use url http://LISTEN-ON/ADMIN-URL-PATH-PREFIX to configurate responses`)
}

func (config *Config) validate() {
	if config.createDefaultRules && config.adminUrlPathPrefix != "" {
		log.Fatal("You can't use both --admin-url-path-prefix and --create-default-rules options")
	}

}
