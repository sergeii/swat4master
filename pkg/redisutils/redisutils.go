package redisutils

func KeysToMembers(keys []string) []interface{} {
	members := make([]interface{}, len(keys))
	for i, v := range keys {
		members[i] = v
	}
	return members
}
