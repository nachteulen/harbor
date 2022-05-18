package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	ctx        = context.Background()
	localCache = map[int64][]int64{}
)

const query = `
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

func getOwnerships(userID int64) []int64 {
	locallyCached, ok := localCache[userID]
	if ok && len(locallyCached) != 0 {
		return locallyCached
	}

	key := fmt.Sprintf("allUserOwnerships:%d", userID)

	v, err := rDB.Get(ctx, key).Result()
	if err != nil {
		fmt.Printf("unable to get ownerships for user(%d): %s\n", userID, err)
	} else if len(v) != 0 {
		var ownerships []int64
		if err := json.Unmarshal([]byte(v), &ownerships); err != nil {
			fmt.Printf("unable to parse cached ownerships for user(%d): %s\n", userID, err)
		} else {
			return ownerships
		}
	}

	db, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		msg := fmt.Sprintf("JWTAuth unable to establish PG connection: %s", err)
		maybeAlertTom("pgConnError", msg)
		panic(msg)
	}

	var idsJSON string
	err = db.Get(&idsJSON, query, userID)
	if err != nil {
		msg := fmt.Sprintf("JWTAuth unable to select ownerships  user(%d): %s", userID, err)
		maybeAlertTom("pgSelectError", msg)
		panic(msg)
	}

	var ownerships []int64
	if err := json.Unmarshal([]byte(idsJSON), &ownerships); err != nil {
		panic(fmt.Sprintf("unable to parse fetched ownerships(%s) for user(%d): %s", idsJSON, userID, err))
	} else if len(ownerships) == 0 {
		panic(fmt.Errorf("no ownerships found for user: %d", userID))
	}

	twoWeeks := time.Hour * 24 * 14
	if err := rDB.Set(ctx, key, idsJSON, twoWeeks).Err(); err != nil {
		fmt.Printf("unable to set ownerships for user(%d): %s\n", userID, err)
	}

	localCache[userID] = ownerships
	return ownerships
}
