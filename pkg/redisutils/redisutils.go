package redisutils

func KeysToMembers(keys []string) []any {
	members := make([]any, len(keys))
	for i, v := range keys {
		members[i] = v
	}
	return members
}
