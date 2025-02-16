package instances_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/filterset"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/persistence/redis/repositories/instances"

	tu "github.com/sergeii/swat4master/internal/testutils"
	"github.com/sergeii/swat4master/internal/testutils/factories/instancefactory"
	"github.com/sergeii/swat4master/internal/testutils/testredis"
)

const (
	DEADBEEF = "\xde\xad\xbe\xef"
	FEEDFOOD = "\xfe\xed\xf0\x0d"
	CAFEBABE = "\xca\xfe\xba\xbe"
	BAADCODE = "\xba\xad\xc0\xde"
)

type b []byte

type storedItem struct {
	ID   []byte `json:"id"`
	IP   net.IP `json:"ip"`
	Port int    `json:"port"`
}

type updated struct {
	ID   string
	Time float64
}

type storageState struct {
	Updates []updated
	Items   map[string]instance.Instance
}

func collectStorageState(ctx context.Context, rdb *redis.Client) storageState {
	zUpdatedMembers := tu.Must(rdb.ZRangeWithScores(ctx, "instances:updated", 0, -1).Result())
	hItems := tu.Must(rdb.HGetAll(ctx, "instances:items").Result())

	updates := make([]updated, 0, len(zUpdatedMembers))
	for _, m := range zUpdatedMembers {
		id := string(tu.Must(hex.DecodeString(m.Member.(string)))) // nolint:forcetypeassert
		updates = append(updates, updated{ID: id, Time: m.Score})
	}

	items := make(map[string]instance.Instance)
	for k, v := range hItems {
		var item storedItem
		id4bytes := tu.Must(hex.DecodeString(k))
		tu.MustNoErr(json.Unmarshal([]byte(v), &item))
		items[string(id4bytes)] = instance.MustNew(instance.MustNewID(id4bytes), item.IP, item.Port)
	}

	return storageState{
		Updates: updates,
		Items:   items,
	}
}

func TestInstancesRedisRepo_Add_New(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	now := c.Now()

	// Given a repository with no instances added...
	repo := instances.New(rdb, c)

	// ...and a new instance to add
	ins1 := instancefactory.Build(
		instancefactory.WithStringID(DEADBEEF),
		instancefactory.WithRandomServerAddress(),
	)

	// When adding the instance to the repository
	err := repo.Add(ctx, ins1)
	require.NoError(t, err)

	// Then the instance is stored in redis
	state := collectStorageState(ctx, rdb)
	require.Len(t, state.Items, 1)
	require.Equal(t, ins1, state.Items[DEADBEEF])
	// And the update time is stored in the sorted set
	require.Len(t, state.Updates, 1)
	assert.Equal(t, DEADBEEF, state.Updates[0].ID)
	assert.Equal(t, float64(now.UnixNano()), state.Updates[0].Time)

	// When another instance is added at a later time
	c.Advance(time.Millisecond * 100)
	ins2 := instancefactory.Build(
		instancefactory.WithStringID(FEEDFOOD),
		instancefactory.WithRandomServerAddress(),
	)
	err = repo.Add(ctx, ins2)
	require.NoError(t, err)

	// Then the second instance is also stored in redis
	state = collectStorageState(ctx, rdb)
	require.Len(t, state.Items, 2)
	require.Equal(t, ins2, state.Items[FEEDFOOD])
	// And the added instances are sorted by the time of addition
	require.Len(t, state.Updates, 2)
	assert.Equal(t, FEEDFOOD, state.Updates[1].ID)
	assert.Equal(t, float64(c.Now().UnixNano()), state.Updates[1].Time)

	// When another instance is added at the same time as the last one
	ins3 := instancefactory.Build(
		instancefactory.WithStringID("baz3"),
		instancefactory.WithRandomServerAddress(),
	)
	err = repo.Add(ctx, ins3)
	require.NoError(t, err)

	// Then the third instance is also stored in redis
	state = collectStorageState(ctx, rdb)
	require.Len(t, state.Items, 3)
	require.Equal(t, ins3, state.Items["baz3"])
	// And the added instances are sorted by the time of addition
	require.Len(t, state.Updates, 3)
}

func TestInstancesRedisRepo_Add_Existing(t *testing.T) {
	ctx := context.TODO()
	rdb := testredis.MakeClient(t)
	c := clockwork.NewFakeClock()
	then := c.Now()

	// Given a repository...
	repo := instances.New(rdb, c)
	// ...with an instance previously added
	ins := instancefactory.Build(
		instancefactory.WithStringID(DEADBEEF),
		instancefactory.WithServerAddress("1.1.1.1", 10480),
	)
	err := repo.Add(ctx, ins)
	require.NoError(t, err)
	// And the instance is stored in the storage
	state := collectStorageState(ctx, rdb)
	require.Len(t, state.Items, 1)
	require.Equal(t, ins, state.Items[DEADBEEF])
	require.Len(t, state.Updates, 1)
	assert.Equal(t, float64(then.UnixNano()), state.Updates[0].Time)

	// When adding another instance with the same ID at a later time
	c.Advance(time.Millisecond * 100)
	other := instancefactory.Build(
		instancefactory.WithStringID(DEADBEEF),
		instancefactory.WithServerAddress(
			"2.2.2.2",
			10580,
		),
	)
	err = repo.Add(ctx, other)
	require.NoError(t, err)

	// Then the instance is replaced in the storage
	state = collectStorageState(ctx, rdb)
	require.Len(t, state.Items, 1)
	assert.Equal(t, other, state.Items[DEADBEEF])
	assert.Len(t, state.Updates, 1)
	assert.Equal(t, float64(then.Add(time.Millisecond*100).UnixNano()), state.Updates[0].Time)
}

func TestInstancesRedisRepo_Get_OK(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	// Given a repository with 2 instances added at the same time...
	repo := instances.New(rdb, c)

	ins1 := instancefactory.Build(
		instancefactory.WithStringID(DEADBEEF),
		instancefactory.WithRandomServerAddress(),
	)
	ins2 := instancefactory.Build(
		instancefactory.WithStringID(FEEDFOOD),
		instancefactory.WithRandomServerAddress(),
	)
	ins3 := instancefactory.Build(
		instancefactory.WithStringID(CAFEBABE),
		instancefactory.WithRandomServerAddress(),
	)

	tu.MustNoErr(repo.Add(ctx, ins1))
	tu.MustNoErr(repo.Add(ctx, ins2))

	// ...and another one added later
	c.Advance(time.Millisecond * 100)
	tu.MustNoErr(repo.Add(ctx, ins3))

	// When retrieving an instance by ID
	for _, pair := range []struct {
		id   []byte
		want instance.Instance
	}{
		{b(DEADBEEF), ins1},
		{b(FEEDFOOD), ins2},
		{b(CAFEBABE), ins3},
	} {
		got, err := repo.Get(ctx, instance.MustNewID(pair.id))
		// Then the instance should be retrieved successfully
		require.NoError(t, err)
		assert.Equal(t, pair.want, got)
	}

	// When retrieving a non-existent instance
	_, err := repo.Get(ctx, instance.MustNewID(b(BAADCODE)))
	// Then the operation should fail with an error
	assert.ErrorIs(t, err, repositories.ErrInstanceNotFound)
}

func TestInstancesRedisRepo_Remove_OK(t *testing.T) {
	tests := []struct {
		name             string
		id               []byte
		wantRemainingIDs []string
	}{
		{
			name:             "remove DEADBEEF",
			id:               b(DEADBEEF),
			wantRemainingIDs: []string{FEEDFOOD, CAFEBABE},
		},
		{
			name:             "remove FEEDFOOD",
			id:               b(FEEDFOOD),
			wantRemainingIDs: []string{DEADBEEF, CAFEBABE},
		},
		{
			name:             "remove non-existent",
			id:               b(BAADCODE),
			wantRemainingIDs: []string{DEADBEEF, FEEDFOOD, CAFEBABE},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			c := clockwork.NewFakeClock()
			rdb := testredis.MakeClient(t)

			// Given a repository with 2 instances added one after another
			repo := instances.New(rdb, c)

			for _, ins := range []instance.Instance{
				instancefactory.Build(
					instancefactory.WithStringID(DEADBEEF),
					instancefactory.WithRandomServerAddress(),
				),
				instancefactory.Build(
					instancefactory.WithStringID(FEEDFOOD),
					instancefactory.WithRandomServerAddress(),
				),
			} {
				c.Advance(time.Millisecond * 100)
				err := repo.Add(ctx, ins)
				require.NoError(t, err)
			}
			// And another instance added at the same time as the last one
			ins := instancefactory.Build(
				instancefactory.WithStringID(CAFEBABE),
				instancefactory.WithRandomServerAddress(),
			)
			err := repo.Add(ctx, ins)
			require.NoError(t, err)

			// When removing an instance by ID
			err = repo.Remove(ctx, instance.MustNewID(tt.id))
			require.NoError(t, err)

			// Then the instance should be removed from the storage
			state := collectStorageState(ctx, rdb)
			require.Len(t, state.Items, len(tt.wantRemainingIDs))
			require.Len(t, state.Updates, len(tt.wantRemainingIDs))
			remainingIDs := make([]string, 0, len(state.Items))
			for id := range state.Items {
				remainingIDs = append(remainingIDs, id)
			}
			assert.ElementsMatch(t, tt.wantRemainingIDs, remainingIDs)
		})
	}
}

func TestInstancesRedisRepo_Clear_OK(t *testing.T) {
	tests := []struct {
		name             string
		factory          func(filterset.InstanceFilterSet, time.Time) filterset.InstanceFilterSet
		wantAffected     int
		wantRemainingIDs []string
	}{
		{
			name: "no filters",
			factory: func(fs filterset.InstanceFilterSet, _ time.Time) filterset.InstanceFilterSet {
				return fs
			},
			wantAffected:     4,
			wantRemainingIDs: []string{},
		},
		{
			name: "filter by time in the past",
			factory: func(fs filterset.InstanceFilterSet, now time.Time) filterset.InstanceFilterSet {
				return fs.UpdatedBefore(now)
			},
			wantAffected:     0,
			wantRemainingIDs: []string{DEADBEEF, FEEDFOOD, CAFEBABE, BAADCODE},
		},
		{
			name: "filter by time in the future",
			factory: func(fs filterset.InstanceFilterSet, now time.Time) filterset.InstanceFilterSet {
				return fs.UpdatedBefore(now.Add(time.Millisecond * 300))
			},
			wantAffected:     4,
			wantRemainingIDs: []string{},
		},
		{
			name: "filter by time before CAFEBABE and BAADCODE",
			factory: func(fs filterset.InstanceFilterSet, now time.Time) filterset.InstanceFilterSet {
				return fs.UpdatedBefore(now.Add(time.Millisecond * 200))
			},
			wantAffected:     2,
			wantRemainingIDs: []string{CAFEBABE, BAADCODE},
		},
		{
			name: "filter by time before FEEDFOOD",
			factory: func(fs filterset.InstanceFilterSet, now time.Time) filterset.InstanceFilterSet {
				return fs.UpdatedBefore(now.Add(time.Millisecond * 100))
			},
			wantAffected:     1,
			wantRemainingIDs: []string{FEEDFOOD, CAFEBABE, BAADCODE},
		},
		{
			name: "filter by time before DEADBEEF",
			factory: func(fs filterset.InstanceFilterSet, now time.Time) filterset.InstanceFilterSet {
				return fs.UpdatedBefore(now.Add(time.Millisecond * 99))
			},
			wantAffected:     0,
			wantRemainingIDs: []string{DEADBEEF, FEEDFOOD, CAFEBABE, BAADCODE},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			c := clockwork.NewFakeClock()
			rdb := testredis.MakeClient(t)

			// Given a repository with 3 instances added one after another
			repo := instances.New(rdb, c)
			before := c.Now()

			for _, ins := range []instance.Instance{
				instancefactory.Build(
					instancefactory.WithStringID(DEADBEEF),
					instancefactory.WithRandomServerAddress(),
				),
				instancefactory.Build(
					instancefactory.WithStringID(FEEDFOOD),
					instancefactory.WithRandomServerAddress(),
				),
				instancefactory.Build(
					instancefactory.WithStringID(CAFEBABE),
					instancefactory.WithRandomServerAddress(),
				),
			} {
				c.Advance(time.Millisecond * 100)
				err := repo.Add(ctx, ins)
				require.NoError(t, err)
			}
			// and another instance added at the same time as the last one
			ins := instancefactory.Build(
				instancefactory.WithStringID(BAADCODE),
				instancefactory.WithRandomServerAddress(),
			)
			err := repo.Add(ctx, ins)
			require.NoError(t, err)

			// When clearing the repository with various filters
			affected, err := repo.Clear(ctx, tt.factory(filterset.NewInstanceFilterSet(), before))
			require.NoError(t, err)

			// Then the operation should succeed and the expected number of instances should be affected
			assert.Equal(t, tt.wantAffected, affected)

			// And the affected instances should be removed from the repository
			state := collectStorageState(ctx, rdb)
			require.Len(t, state.Items, len(tt.wantRemainingIDs))
			require.Len(t, state.Updates, len(tt.wantRemainingIDs))
			remainingIDs := make([]string, 0, len(state.Items))
			for id := range state.Items {
				remainingIDs = append(remainingIDs, id)
			}
			assert.ElementsMatch(t, tt.wantRemainingIDs, remainingIDs)
		})
	}
}

func TestInstancesRedisRepo_Clear_Empty(t *testing.T) {
	tests := []struct {
		name    string
		factory func(filterset.InstanceFilterSet, time.Time) filterset.InstanceFilterSet
	}{
		{
			name: "no filters",
			factory: func(fs filterset.InstanceFilterSet, _ time.Time) filterset.InstanceFilterSet {
				return fs
			},
		},
		{
			name: "filter by time",
			factory: func(fs filterset.InstanceFilterSet, now time.Time) filterset.InstanceFilterSet {
				return fs.UpdatedBefore(now)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			c := clockwork.NewFakeClock()
			rdb := testredis.MakeClient(t)

			// Given a repository with no instances
			repo := instances.New(rdb, c)
			now := c.Now()

			// When attempting to clear the repository
			affected, err := repo.Clear(ctx, tt.factory(filterset.NewInstanceFilterSet(), now))

			// Then the operation should succeed and no instances should be affected
			require.NoError(t, err)
			assert.Equal(t, 0, affected)
		})
	}
}

func TestInstancesRedisRepo_Count_OK(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	repo := instances.New(rdb, c)

	// Given a repository with 3 instances
	ins1 := instancefactory.Build(
		instancefactory.WithStringID(DEADBEEF),
		instancefactory.WithRandomServerAddress(),
	)
	ins2 := instancefactory.Build(
		instancefactory.WithStringID(FEEDFOOD),
		instancefactory.WithRandomServerAddress(),
	)
	ins3 := instancefactory.Build(
		instancefactory.WithStringID(CAFEBABE),
		instancefactory.WithRandomServerAddress(),
	)

	for _, ins := range []instance.Instance{ins1, ins2, ins3} {
		err := repo.Add(ctx, ins)
		require.NoError(t, err)
	}

	// When counting the objects in the repository
	count, err := repo.Count(ctx)
	require.NoError(t, err)

	// Then the count is expected to be the number of added instances
	assert.Equal(t, 3, count)
}

func TestInstancesRedisRepo_Count_Empty(t *testing.T) {
	ctx := context.TODO()
	c := clockwork.NewFakeClock()
	rdb := testredis.MakeClient(t)

	// Given a repository with empty underlying storage
	repo := instances.New(rdb, c)

	// When counting the objects in the repository
	count, err := repo.Count(ctx)
	require.NoError(t, err)

	// Then the count is expected to be 0
	assert.Equal(t, 0, count)
}
