package lib

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/go-redis/redis/v8"
)

const currentHouseholdQuery = `
select current_household.id
from users u
inner join household_users owner_hu on
	u.id = owner_hu.user_id and owner_hu.household_user_type_id = 1
left join household_users invited_hu on
	u.id = invited_hu.user_id and invited_hu.household_user_type_id in (2, 4, 5)
inner join households current_household on
	coalesce(invited_hu.household_id, owner_hu.household_id) = current_household.id
where u.id = $1`

var (
	ctx                   = context.Background()
	currentHouseholdCache = map[string]int64{}
)

func GetCurrentHouseholdID(
	userID string,
	rDB *redis.Client,
	pgDB *sqlx.DB,
) int64 {
	hhID, ok := currentHouseholdCache[userID]
	if ok && hhID != 0 {
		return hhID
	}

	key := fmt.Sprintf("%s:currentHousehold", userID)
	hhID, err := rDB.Get(ctx, key).Int64()
	if err == nil {
		currentHouseholdCache[userID] = hhID
		return hhID
	} else if err != redis.Nil {
		fmt.Printf("error fetching %s for user(%s): %s\n", key, userID, err)
	}

	var result int64
	err = pgDB.Get(&result, currentHouseholdQuery, userID)
	if err != nil {
		panic(fmt.Errorf("unable to get household for user(%s): %s", userID, err))
	} else if result == 0 {
		panic(fmt.Errorf("invalid householdID for user(%s)", userID))
	}

	currentHouseholdCache[userID] = result

	twoWeeks := time.Hour * 24 * 14
	if err := rDB.Set(ctx, key, result, twoWeeks).Err(); err != nil {
		fmt.Printf("unable to set household for user(%s): %s\n", userID, err)
	}

	return result
}
