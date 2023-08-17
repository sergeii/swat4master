package instances_test

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/core/entities/addr"
	"github.com/sergeii/swat4master/internal/core/entities/instance"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/data/memory/instances"
)

func TestInstanceMemoryRepo_Add_NewInstance(t *testing.T) {
	repo := instances.New()
	ctx := context.TODO()

	ins1 := instance.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
	err := repo.Add(ctx, ins1)
	require.NoError(t, err)

	ins2 := instance.MustNew("bar", net.ParseIP("2.2.2.2"), 10480)
	err = repo.Add(ctx, ins2)
	require.NoError(t, err)

	got, err := repo.GetByAddr(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1:10480", got.GetAddr().String())

	got, err = repo.GetByID(ctx, "foo")
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1:10480", got.GetAddr().String())
	assert.Equal(t, "foo", got.GetID())

	got, err = repo.GetByAddr(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	require.NoError(t, err)
	assert.Equal(t, "2.2.2.2:10480", got.GetAddr().String())
	assert.Equal(t, "bar", got.GetID())

	got, err = repo.GetByID(ctx, "bar")
	require.NoError(t, err)
	assert.Equal(t, "2.2.2.2:10480", got.GetAddr().String())
}

func TestInstanceMemoryRepo_Add_OldInstance(t *testing.T) {
	repo := instances.New()
	ctx := context.TODO()

	former := instance.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
	err := repo.Add(ctx, former)
	require.NoError(t, err)

	got, err := repo.GetByAddr(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1:10480", got.GetAddr().String())
	assert.Equal(t, "foo", got.GetID())

	got, err = repo.GetByID(ctx, "foo")
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1:10480", got.GetAddr().String())
	assert.Equal(t, "foo", got.GetID())

	// add a new instance with different id but same address
	latter := instance.MustNew("bar", net.ParseIP("1.1.1.1"), 10480)
	err = repo.Add(ctx, latter)
	require.NoError(t, err)

	got, err = repo.GetByAddr(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1:10480", got.GetAddr().String())
	assert.Equal(t, "bar", got.GetID())

	got, err = repo.GetByID(ctx, "bar")
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1:10480", got.GetAddr().String())
	assert.Equal(t, "bar", got.GetID())

	// instance with the former id was removed
	_, err = repo.GetByID(ctx, "foo")
	assert.ErrorIs(t, err, repositories.ErrInstanceNotFound)
}

func TestInstanceMemoryRepo_GetByAddr(t *testing.T) {
	tests := []struct {
		name    string
		addr    addr.Addr
		wantID  string
		wantErr error
	}{
		{
			"positive case #1",
			addr.MustNewFromString("1.1.1.1", 10480),
			"foo",
			nil,
		},
		{
			"positive case #2",
			addr.MustNewFromString("1.1.1.1", 10580),
			"bar",
			nil,
		},
		{
			"empty address",
			addr.Blank,
			"",
			repositories.ErrInstanceNotFound,
		},
		{
			"unknown address",
			addr.MustNewFromString("1.1.1.1", 10680),
			"",
			repositories.ErrInstanceNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := instances.New()
			ctx := context.TODO()

			ins1 := instance.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
			_ = repo.Add(ctx, ins1)
			ins2 := instance.MustNew("bar", net.ParseIP("1.1.1.1"), 10580)
			_ = repo.Add(ctx, ins2)

			got, err := repo.GetByAddr(ctx, tt.addr)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantID, got.GetID())
				assert.Equal(t, tt.addr, got.GetAddr())
			}
		})
	}
}

func TestInstanceMemoryRepo_GetByID(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		wantAddr addr.Addr
		wantErr  error
	}{
		{
			"positive case #1",
			"foo",
			addr.MustNewFromString("1.1.1.1", 10480),
			nil,
		},
		{
			"positive case #2",
			"bar",
			addr.MustNewFromString("1.1.1.1", 10580),
			nil,
		},
		{
			"empty id",
			"",
			addr.Blank,
			repositories.ErrInstanceNotFound,
		},
		{
			"unknown id",
			"baz",
			addr.Blank,
			repositories.ErrInstanceNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := instances.New()
			ctx := context.TODO()

			ins1 := instance.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
			_ = repo.Add(ctx, ins1)
			ins2 := instance.MustNew("bar", net.ParseIP("1.1.1.1"), 10580)
			_ = repo.Add(ctx, ins2)

			got, err := repo.GetByID(ctx, tt.id)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantAddr, got.GetAddr())
				assert.Equal(t, tt.id, got.GetID())
			}
		})
	}
}

func TestInstanceMemoryRepo_RemoveByID(t *testing.T) {
	repo := instances.New()
	ctx := context.TODO()

	ins1 := instance.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
	_ = repo.Add(ctx, ins1)
	ins2 := instance.MustNew("bar", net.ParseIP("2.2.2.2"), 10480)
	_ = repo.Add(ctx, ins2)

	err := repo.RemoveByID(ctx, "foo")
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, "foo")
	assert.ErrorIs(t, err, repositories.ErrInstanceNotFound)

	got, err := repo.GetByID(ctx, "bar")
	require.NoError(t, err)
	assert.Equal(t, "2.2.2.2:10480", got.GetAddr().String())

	// now remove ins2 too
	err = repo.RemoveByID(ctx, "bar")
	require.NoError(t, err)

	_, err = repo.GetByID(ctx, "bar")
	assert.ErrorIs(t, err, repositories.ErrInstanceNotFound)

	// subsequent remove calls do not trigger an error
	for _, id := range []string{"foo", "bar"} {
		err := repo.RemoveByID(ctx, id)
		require.NoError(t, err)
	}
}

func TestInstanceMemoryRepo_RemoveByAddr(t *testing.T) {
	repo := instances.New()
	ctx := context.TODO()

	ins1 := instance.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
	_ = repo.Add(ctx, ins1)
	ins2 := instance.MustNew("bar", net.ParseIP("2.2.2.2"), 10480)
	_ = repo.Add(ctx, ins2)

	err := repo.RemoveByAddr(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	require.NoError(t, err)

	_, err = repo.GetByAddr(ctx, addr.MustNewFromString("1.1.1.1", 10480))
	assert.ErrorIs(t, err, repositories.ErrInstanceNotFound)
	_, err = repo.GetByID(ctx, "foo")
	assert.ErrorIs(t, err, repositories.ErrInstanceNotFound)

	got, err := repo.GetByAddr(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	require.NoError(t, err)
	assert.Equal(t, "bar", got.GetID())

	// now remove ins2 too
	err = repo.RemoveByAddr(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	require.NoError(t, err)

	_, err = repo.GetByAddr(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	assert.ErrorIs(t, err, repositories.ErrInstanceNotFound)
	_, err = repo.GetByID(ctx, "bar")
	assert.ErrorIs(t, err, repositories.ErrInstanceNotFound)

	// subsequent remove calls do not trigger an error
	for _, insAddr := range []addr.Addr{
		addr.MustNewFromString("1.1.1.1", 10480),
		addr.MustNewFromString("2.2.2.2", 10480),
	} {
		err := repo.RemoveByAddr(ctx, insAddr)
		require.NoError(t, err)
	}
}

func TestInstanceMemoryRepo_Count(t *testing.T) {
	repo := instances.New()
	ctx := context.TODO()

	assertCount := func(expected int) {
		cnt, err := repo.Count(ctx)
		assert.NoError(t, err)
		assert.Equal(t, expected, cnt)
	}

	assertCount(0)

	ins1 := instance.MustNew("foo", net.ParseIP("1.1.1.1"), 10480)
	_ = repo.Add(ctx, ins1)
	assertCount(1)

	ins2 := instance.MustNew("bar", net.ParseIP("2.2.2.2"), 10480)
	_ = repo.Add(ctx, ins2)
	assertCount(2)

	_ = repo.RemoveByID(ctx, "foo")
	assertCount(1)

	_ = repo.RemoveByAddr(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	assertCount(0)

	// double remove
	_ = repo.RemoveByID(ctx, "foo")
	assertCount(0)

	_ = repo.RemoveByAddr(ctx, addr.MustNewFromString("2.2.2.2", 10480))
	assertCount(0)
}
