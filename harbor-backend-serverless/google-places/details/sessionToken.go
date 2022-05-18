package main

import (
	"context"
	"fmt"
)

var (
	ctx = context.Background()
)

func getUserSessionToken(userID string) (string, error) {
	if rDB == nil {
		return "", fmt.Errorf("rDB uninitialized")
	}

	key := fmt.Sprintf("%s:googlePlacesSession", userID)
	sess, err := rDB.Get(ctx, key).Result()
	if err != nil {
		return "", err
	}

	if err := rDB.Del(ctx, key).Err(); err != nil {
		tmplt := "error deleting key(%s) for user(%s): %s\n"
		fmt.Printf(tmplt, key, userID, err)
	}

	return sess, nil
}
