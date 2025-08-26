package internal

import (
	"os"

	"github.com/redis/go-redis/v9"
)

var RDB = redis.NewClient(&redis.Options{
	Addr:     os.Getenv("REDIS_ADDRESS"),
	Password: os.Getenv("REDIS_PASSWORD"),
	DB:       0,
})
