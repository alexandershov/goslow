package main

import (
	"flag"
	"log"
)

// Config stores command line arguments.
type Config struct {
	listenOn               string
	deployedOn             string // doesn't affect the listening address, used in response help texts only
	driver                 string
	dataSource             string
	minSiteLength          int
	siteSalt               string
	createDefaultEndpoints bool
	adminPathPrefix        string
}

var DEFAULT_CONFIG = Config{
	listenOn:      ":5103",
	deployedOn:    "localhost:5103",
	driver:        "sqlite3",
	dataSource:    "file::memory:?cache=shared", // we need cache=shared in the case of reconnect which happens sometines under load
	minSiteLength: 6,
	siteSalt:      "",
	// DEFAULT_CONFIG describes a server running in a single site mode.
	// Single site mode doesn't need default endpoints.
	createDefaultEndpoints: false,
	adminPathPrefix:        "/goslow",
}

// NewConfigFromArgs returns a new config from command line arguments.
func NewConfigFromArgs() *Config {
	config := new(Config)
	config.defineFlags()
	config.parseFlags()
	config.validate()
	return config
}

func (config *Config) defineFlags() {
	flag.StringVar(&config.listenOn, "listen-on", DEFAULT_CONFIG.listenOn,
		"address to listen on. E.g: 0.0.0.0:8000")

	flag.StringVar(&config.deployedOn, "deployed-on", DEFAULT_CONFIG.deployedOn,
		`url at which this instance of goslow is deployed and visible to the world.
	Used only in response help texts, doesn't affect the listening address. E.g: goslow.link`)

	// attention: cmd arg is called --db to avoid exposing golang lingo to the public interface
	flag.StringVar(&config.driver, "db", DEFAULT_CONFIG.driver,
		"database driver. Possible values: sqlite3, postgres.")

	flag.StringVar(&config.dataSource, "data-source", DEFAULT_CONFIG.dataSource,
		`data source name. E.g: postgres://user:password@localhost/dbname for postgres
	or /path/to/sqlite3/db for sqlite3`)

	flag.IntVar(&config.minSiteLength, "min-site-length", DEFAULT_CONFIG.minSiteLength,
		"minimum length of the randomly generated site names. E.g: 8")

	flag.StringVar(&config.siteSalt, "site-salt", DEFAULT_CONFIG.siteSalt,
		"random names generator salt. Keep it secret. E.g: kj8ioIxZ")

	flag.BoolVar(&config.createDefaultEndpoints, "create-default-endpoints", DEFAULT_CONFIG.createDefaultEndpoints,
		`If true, then create default endpoints. E.g when deployed at localhost:5103,
		then create endpoints 0.localhost:5103,	1.localhost:5103, ..., and 599.localhost:5103
		before starting the server.`)

	flag.StringVar(&config.adminPathPrefix, "admin-path-prefix", DEFAULT_CONFIG.adminPathPrefix,
		`If not an empty string: run in single domain mode
	and use the endpoint http://LISTEN-ON/ADMIN-PATH-PREFIX (default is http://localhost:5103/goslow)
	to configurate responses`)
}

func (config *Config) parseFlags() {
	flag.Parse()
}

func (config *Config) validate() {
	if config.createDefaultEndpoints && config.isInSingleSiteMode() {
		log.Fatal("You can't use both --admin-path-prefix and --create-default-endpoints options")
	}
}

func (config *Config) isInSingleSiteMode() bool {
	return config.adminPathPrefix != ""
}
