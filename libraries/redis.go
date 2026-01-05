package libraries

import (
	"context"
	"log"
	"os"
	"sync"

	"github.com/go-redis/redis/v8"
)

var lock = &sync.Mutex{}

type Singleton struct {
	client *redis.Client
}

var instance *Singleton

func GetInstance() *redis.Client {
	if instance == nil {
		lock.Lock()
		defer lock.Unlock()
		if instance == nil {
			instance = &Singleton{
				client: GetClient(),
			}
		}
	}
	return instance.client
}

func GetClient() *redis.Client {

	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "localhost:6379" // default address
	}

	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		Password: "",
		DB: 0,
	})

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Print("SERVER - Connected to Redis successfully")
	return client
}