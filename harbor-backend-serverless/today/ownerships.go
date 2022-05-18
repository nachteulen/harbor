package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/go-redis/redis/v8"
	_ "github.com/lib/pq"
)

var (
	ctx        = context.Background()
	redisConn  *redis.Client
	localCache = map[string]string{}
)

const ownershipsQuery = `
select json_agg(distinct o.id)
from household_users
inner join households h on household_users.household_id = h.id
inner join household_users ownerAndJoinedHouseholdUsers
	on ownerAndJoinedHouseholdUsers.household_id = h.id
inner join household_users other_hh
	on ownerAndJoinedHouseholdUsers.user_id = other_hh.user_id
inner join ownerships o
	on o.household_user_id = household_users.id
    or (other_hh.id = o.household_user_id and o.ownership_type_id = 2)
where household_users.user_id = $1`

func getOwnershipsStr(userID string) string {
	locallyCached, ok := localCache[userID]
	if ok && len(locallyCached) != 0 {
		return locallyCached
	}

	if redisConn == nil {
		opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
		if err != nil {
			fmt.Printf("unable to connect to redis for user(%s): %s\n", userID, err)
		} else {
			redisConn = redis.NewClient(opt)
		}
	}

	key := fmt.Sprintf("allUserOwnerships:%s", userID)
	if redisConn != nil {
		v, err := redisConn.Get(ctx, key).Result()
		if err != nil {
			fmt.Printf("unable to get ownerships for user(%s): %s\n", userID, err)
		} else if len(v) != 0 {
			return v
		}
	}

	var idsJSON string
	err := pgDB.Get(&idsJSON, ownershipsQuery, userID)
	if err != nil {
		panic(fmt.Sprintf("unable to select ownerships for user(%s): %s", userID, err))
	}

	if redisConn != nil {
		twoWeeks := time.Hour * 24 * 14
		if err := redisConn.Set(ctx, key, idsJSON, twoWeeks).Err(); err != nil {
			fmt.Printf("unable to set ownerships for user(%s): %s\n", userID, err)
		}
	}

	localCache[userID] = idsJSON
	return idsJSON
}
