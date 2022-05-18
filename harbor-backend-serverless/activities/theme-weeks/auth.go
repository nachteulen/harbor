package main

import "fmt"

const authQuery = `select exists (
	select * from users where id = $1 and role = 'admin'
)`

var isAdminCache = map[string]bool{}

func isAdmin(userID string) bool {
	cached, ok := isAdminCache[userID]
	if ok {
		return cached
	}

	var isAdmin bool
	if err := pgDB.Get(&isAdmin, authQuery, userID); err != nil {
		fmt.Printf("error checking admin status for user(%s): %s\n", userID, err)
		return false
	}

	isAdminCache[userID] = isAdmin
	return isAdmin
}
