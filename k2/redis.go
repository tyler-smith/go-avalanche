package main

import (
	"time"

	"github.com/gomodule/redigo/redis"
)

const (
	redisEndpointKey = "k2:endpoints"
	redisEndpointTTL = 3 * time.Minute
)

func setEndpoint(rConn redis.Conn, host string) error {
	_, err := rConn.Do("ZADD", redisEndpointKey, time.Now().Add(redisEndpointTTL).Unix(), host)
	return err
}

func remEndpoint(rConn redis.Conn, host string) error {
	_, err := rConn.Do("ZREM", redisEndpointKey, host)
	return err
}

func getEndpoints(rConn redis.Conn) ([]string, error) {
	return redis.Strings(rConn.Do("ZRANGEBYSCORE", redisEndpointKey, time.Now().Unix, "+inf"))
}
