package rdb

import (
	"os"

	"github.com/redis/go-redis/v9"
)

var DB = redis.NewClient(&redis.Options{
	Addr:     os.Getenv("REDIS_ADDRESS"),
	Password: os.Getenv("REDIS_PASSWORD"),
	DB:       0,
})
