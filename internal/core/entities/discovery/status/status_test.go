package status_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
)

func TestDiscoveryStatus_Bits(t *testing.T) {
	tests := []struct {
		name   string
		status ds.DiscoveryStatus
		want   []ds.DiscoveryStatus
	}{
		{
			name:   "No status",
			status: ds.NoStatus,
			want:   []ds.DiscoveryStatus{},
		},
		{
			name:   "New",
			status: ds.New,
			want:   []ds.DiscoveryStatus{ds.New},
		},
		{
			name:   "No Port",
			status: ds.NoPort,
			want:   []ds.DiscoveryStatus{ds.NoPort},
		},
		{
			name:   "Master | Info | Details | Port",
			status: ds.Master | ds.Info | ds.Details | ds.Port,
			want:   []ds.DiscoveryStatus{ds.Master, ds.Info, ds.Details, ds.Port},
		},
		{
			name:   "All statuses",
			status: ds.Master | ds.Info | ds.Details | ds.DetailsRetry | ds.NoDetails | ds.Port | ds.PortRetry | ds.NoPort,
			want: []ds.DiscoveryStatus{
				ds.Master,
				ds.Info,
				ds.Details,
				ds.DetailsRetry,
				ds.NoDetails,
				ds.Port,
				ds.PortRetry,
				ds.NoPort,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make([]ds.DiscoveryStatus, 0, len(tt.want))
			for status := range tt.status.Bits() {
				got = append(got, status)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
