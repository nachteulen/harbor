package main

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

var ctx = context.Background()

func getUserSessionToken(userID string) (string, error) {
	if rDB == nil {
		return "", fmt.Errorf("rDB uninitialized")
	}

	key := fmt.Sprintf("%s:googlePlacesSession", userID)
	sess, err := rDB.Get(ctx, key).Result()

	if err == nil {
		return sess, nil
	} else if err != redis.Nil {
		return "", err
	}

	sess = uuid.New().String()
	if err := rDB.Set(ctx, key, sess, 5*time.Minute).Err(); err != nil {
		return "", err
	}

	return sess, nil
}
