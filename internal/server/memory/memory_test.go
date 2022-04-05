package memory_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sergeii/swat4master/internal/aggregate"
	"github.com/sergeii/swat4master/internal/server"
	"github.com/sergeii/swat4master/internal/server/memory"
)

func TestServerMemoryRepo_Report_NewInstance(t *testing.T) {
	repo := memory.New()

	instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	gameServer, _ := aggregate.NewGameServer(net.ParseIP("1.1.1.1"), 10480, 10481)
	err := repo.Report(gameServer, string(instanceID), map[string]string{"hostname": "Swat4 Server"})
	require.NoError(t, err)

	otherInstanceID := []byte{0xde, 0xad, 0xbe, 0xef}
	otherGameServer, _ := aggregate.NewGameServer(net.ParseIP("2.2.2.2"), 10480, 10481)
	otherErr := repo.Report(otherGameServer, string(otherInstanceID),
		map[string]string{"hostname": "Another Swat4 Server"},
	)
	require.NoError(t, otherErr)

	got, err := repo.GetByInstanceID(string(instanceID))
	require.NoError(t, err)
	assert.Equal(t, "Swat4 Server", got.GetReportedParams()["hostname"])

	got, err = repo.GetByInstanceID(string(otherInstanceID))
	require.NoError(t, err)
	assert.Equal(t, "Another Swat4 Server", got.GetReportedParams()["hostname"])
}

func TestServerMemoryRepo_Report_UpdateInstance(t *testing.T) {
	repo := memory.New()

	instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	gameServer, _ := aggregate.NewGameServer(net.ParseIP("1.1.1.1"), 10480, 10481)
	before := time.Now()
	err := repo.Report(gameServer, string(instanceID), map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10480",
	})
	require.NoError(t, err)
	got, err := repo.GetByInstanceID(string(instanceID))
	require.NoError(t, err)
	assert.Equal(t, "A-Bomb Nightclub", got.GetReportedParams()["mapname"])
	assert.Equal(t, "16", got.GetReportedParams()["numplayers"])
	reportedSinceBefore, _ := repo.GetReportedSince(before)
	assert.Len(t, reportedSinceBefore, 1)

	time.Sleep(time.Millisecond)
	after := time.Now()
	// server is now unlisted
	reportedSinceAfter, _ := repo.GetReportedSince(after)
	assert.Len(t, reportedSinceAfter, 0)

	err = repo.Report(gameServer, string(instanceID), map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
	})
	require.NoError(t, err)
	got, err = repo.GetByInstanceID(string(instanceID))
	require.NoError(t, err)
	assert.Equal(t, "Food Wall Restaurant", got.GetReportedParams()["mapname"])
	assert.Equal(t, "15", got.GetReportedParams()["numplayers"])

	// server is listed again
	reportedSinceAfter, _ = repo.GetReportedSince(after)
	assert.Len(t, reportedSinceAfter, 1)
}

func TestServerMemoryRepo_Report_NewInstanceOldServer(t *testing.T) {
	repo := memory.New()

	oldID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	gameServer, _ := aggregate.NewGameServer(net.ParseIP("1.1.1.1"), 10480, 10481)
	err := repo.Report(gameServer, string(oldID), map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "A-Bomb Nightclub",
		"numplayers": "16",
		"hostport":   "10480",
	})
	require.NoError(t, err)
	got, err := repo.GetByInstanceID(string(oldID))
	require.NoError(t, err)
	assert.Equal(t, "A-Bomb Nightclub", got.GetReportedParams()["mapname"])
	assert.Equal(t, "16", got.GetReportedParams()["numplayers"])

	time.Sleep(time.Millisecond)
	before := time.Now()
	reportedSinceBefore, _ := repo.GetReportedSince(before)
	assert.Len(t, reportedSinceBefore, 0)

	newID := []byte{0xde, 0xad, 0xbe, 0xef}
	err = repo.Report(gameServer, string(newID), map[string]string{
		"gametype":   "VIP Escort",
		"mapname":    "Food Wall Restaurant",
		"numplayers": "15",
		"hostport":   "10480",
	})
	require.NoError(t, err)

	got, err = repo.GetByInstanceID(string(newID))
	require.NoError(t, err)
	assert.Equal(t, "Food Wall Restaurant", got.GetReportedParams()["mapname"])
	assert.Equal(t, "15", got.GetReportedParams()["numplayers"])

	// server is relisted
	reportedSinceBefore, _ = repo.GetReportedSince(before)
	assert.Len(t, reportedSinceBefore, 1)

	// the server is no longer accessible with the old instance key
	_, err = repo.GetByInstanceID(string(oldID))
	require.ErrorIs(t, err, server.ErrServerNotFound)
}

func TestServerMemoryRepo_Renew(t *testing.T) {
	tests := []struct {
		name       string
		instanceID []byte
		ipaddr     string
		wantErr    error
	}{
		{
			name:       "positive case",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			ipaddr:     "1.1.1.1",
			wantErr:    nil,
		},
		{
			name:       "unknown instance id",
			instanceID: []byte{0xde, 0xad, 0xbe, 0xef},
			ipaddr:     "1.1.1.1",
			wantErr:    server.ErrServerNotFound,
		},
		{
			name:       "ip address does not match",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			ipaddr:     "192.168.0.1",
			wantErr:    server.ErrServerNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := memory.New()
			instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
			gameServer, _ := aggregate.NewGameServer(net.ParseIP("1.1.1.1"), 10480, 10481)
			repo.Report(gameServer, string(instanceID), map[string]string{"hostname": "Swat4 Server"}) // nolint: errcheck

			time.Sleep(time.Millisecond)
			beforeRenew := time.Now()
			reportedSinceBeforeRenew, _ := repo.GetReportedSince(beforeRenew)
			assert.Len(t, reportedSinceBeforeRenew, 0)

			afterRenew := time.Now()
			err := repo.Renew(tt.ipaddr, string(tt.instanceID))
			reportedSinceAfterRenew, _ := repo.GetReportedSince(afterRenew)

			if tt.wantErr != nil {
				assert.ErrorIs(t, tt.wantErr, err)
				assert.Len(t, reportedSinceAfterRenew, 0)
			} else {
				assert.NoError(t, err)
				assert.Len(t, reportedSinceAfterRenew, 1)
			}
		})
	}
}

func TestServerMemoryRepo_Remove(t *testing.T) {
	tests := []struct {
		name       string
		instanceID []byte
		ipaddr     string
		wantErr    error
	}{
		{
			name:       "positive case",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			ipaddr:     "1.1.1.1",
			wantErr:    nil,
		},
		{
			name:       "unknown instance id",
			instanceID: []byte{0xde, 0xad, 0xbe, 0xef},
			ipaddr:     "1.1.1.1",
			wantErr:    server.ErrServerNotFound,
		},
		{
			name:       "ip address does not match",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			ipaddr:     "2.2.2.2",
			wantErr:    server.ErrServerNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := time.Now()
			repo := memory.New()
			instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
			gameServer, _ := aggregate.NewGameServer(net.ParseIP("1.1.1.1"), 10480, 10481)
			repo.Report(gameServer, string(instanceID), map[string]string{"hostname": "Swat4 Server"}) // nolint: errcheck

			reportedSinceBefore, _ := repo.GetReportedSince(before)
			assert.Len(t, reportedSinceBefore, 1)

			err := repo.Remove(tt.ipaddr, string(tt.instanceID))
			getInst, getErr := repo.GetByInstanceID(string(instanceID))
			reportedAfterRemove, _ := repo.GetReportedSince(before)

			if tt.wantErr != nil {
				assert.ErrorIs(t, tt.wantErr, err)
				assert.NoError(t, getErr)
				assert.NotNil(t, getInst)
				assert.Len(t, reportedAfterRemove, 1)
			} else {
				assert.NoError(t, err)
				assert.Nil(t, getInst)
				assert.ErrorIs(t, getErr, server.ErrServerNotFound)
				assert.Len(t, reportedAfterRemove, 0)
			}
		})
	}
}

func TestServerMemoryRepo_GetByInstanceID(t *testing.T) {
	tests := []struct {
		name       string
		instanceID []byte
		wantErr    error
	}{
		{
			name:       "positive case",
			instanceID: []byte{0xfe, 0xed, 0xf0, 0x0d},
			wantErr:    nil,
		},
		{
			name:       "unknown instance id",
			instanceID: []byte{0xde, 0xad, 0xbe, 0xef},
			wantErr:    server.ErrServerNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := memory.New()
			instanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
			gameServer, _ := aggregate.NewGameServer(net.ParseIP("1.1.1.1"), 10480, 10481)
			repo.Report(gameServer, string(instanceID), map[string]string{"hostname": "Swat4 Server"}) // nolint: errcheck

			got, err := repo.GetByInstanceID(string(tt.instanceID))
			if tt.wantErr != nil {
				assert.ErrorIs(t, tt.wantErr, err)
			} else {
				assert.Equal(t, "Swat4 Server", got.GetReportedParams()["hostname"])
			}
		})
	}
}

func TestServerMemoryRepo_WithCleaner_Run(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	repo := memory.New(memory.WithCleaner(ctx, time.Millisecond*5, time.Millisecond*50))

	oldInstanceID := []byte{0xfe, 0xed, 0xf0, 0x0d}
	oldServer, _ := aggregate.NewGameServer(net.ParseIP("1.1.1.1"), 10480, 10481)
	repo.Report(oldServer, string(oldInstanceID), map[string]string{"hostname": "Old Swat4 Server"}) // nolint: errcheck
	time.Sleep(time.Millisecond * 51)

	newnstanceID := []byte{0xde, 0xad, 0xbe, 0xef}
	newServer, _ := aggregate.NewGameServer(net.ParseIP("2.2.2.2"), 10480, 10481)
	repo.Report(newServer, string(newnstanceID), map[string]string{"hostname": "New Swat4 Server"}) // nolint: errcheck

	time.Sleep(time.Millisecond * 6)
	// the older server should have been sweeped by this time
	got, err := repo.GetByInstanceID(string(oldInstanceID))
	assert.ErrorIs(t, err, server.ErrServerNotFound)
	assert.Nil(t, got)

	got, err = repo.GetByInstanceID(string(newnstanceID))
	require.NoError(t, err)
	assert.Equal(t, "New Swat4 Server", got.GetReportedParams()["hostname"])
}

func TestServerMemoryRepo_WithCleaner_EmptyStorageNoError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	memory.New(memory.WithCleaner(ctx, time.Millisecond*5, time.Second))
	time.Sleep(time.Millisecond * 20)
}
