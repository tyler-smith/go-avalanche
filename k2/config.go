package main

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultNodeCount = 1
)

type Config struct {
	NodeCount int

	BCHD BCHDConfig

	RedisHost string
	MySQLDSN  string

	Logging bool
}

type BCHDConfig struct {
	Host     string
	User     string
	Password string
}

func NewConfigFromEnv() (Config, error) {
	nodeCount, err := envInt("K2_NODE_COUNT", defaultNodeCount)
	if err != nil {
		return Config{}, err
	}

	return Config{
		NodeCount: nodeCount,

		BCHD: BCHDConfig{
			Host:     envString("K2_BCHD_HOST", "localhost:8334"),
			User:     envString("K2_BCHD_USER", "rpc"),
			Password: envString("K2_BCHD_PASSWORD", "rpc"),
		},

		RedisHost: envString("K2_REDIS_HOST", "localhost:6379"),
		MySQLDSN:  envString("K2_MYSQL_DSN", "root:password@tcp(localhost:3306)/k2?parseTime=true"),
		Logging:   envBool("K2_LOGGING", true),
	}, nil
}

func envString(name string, defaultVal string) string {
	val := os.Getenv(name)
	if val == "" {
		return defaultVal
	}
	return val
}

func envInt(name string, defaultVal int) (int, error) {
	val := os.Getenv(name)
	if val == "" {
		return defaultVal, nil
	}

	return strconv.Atoi(val)
}

func envBool(name string, defaultVal bool) bool {
	val := os.Getenv(name)
	if val == "" {
		return defaultVal
	}
	switch strings.ToLower(val) {
	case "0":
		return false
	case "false":
		return false
	}
	return true
}
