package listservers_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/sergeii/swat4master/internal/core/entities/details"
	ds "github.com/sergeii/swat4master/internal/core/entities/discovery/status"
	"github.com/sergeii/swat4master/internal/core/entities/server"
	"github.com/sergeii/swat4master/internal/core/repositories"
	"github.com/sergeii/swat4master/internal/core/usecases/listservers"
	"github.com/sergeii/swat4master/internal/testutils/factories"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query"
	"github.com/sergeii/swat4master/pkg/gamespy/browsing/query/filter"
)

type MockServerRepository struct {
	mock.Mock
	repositories.ServerRepository
}

func (m *MockServerRepository) Filter(ctx context.Context, fs repositories.ServerFilterSet) ([]server.Server, error) {
	args := m.Called(ctx, fs)
	if err := args.Error(1); err != nil {
		return nil, err
	}
	return args.Get(0).([]server.Server), nil // nolint: forcetypeassert
}

func TestListServersUseCase_FilterParams(t *testing.T) {
	ctx := context.TODO()
	now := time.Now()

	repoServers := []server.Server{
		factories.BuildRandomServer(),
		factories.BuildRandomServer(),
	}

	tests := []struct {
		name       string
		recentness time.Duration
		status     ds.DiscoveryStatus
		wantCall   func(fs repositories.ServerFilterSet) bool
	}{
		{
			"servers active in the last hour with info status",
			time.Hour,
			ds.Info,
			func(fs repositories.ServerFilterSet) bool {
				_, activeBeforeIsSet := fs.GetActiveBefore()
				activeAfter, activeAfterIsSet := fs.GetActiveAfter()
				withStatus, withStatusIsSet := fs.GetWithStatus()

				expectActiveBefore := !activeBeforeIsSet
				expectActiveAfter := activeAfterIsSet && activeAfter.After(now.Add(-time.Hour))
				expectWithStatus := withStatusIsSet && withStatus == ds.Info

				return expectActiveBefore && expectActiveAfter && expectWithStatus
			},
		},
		{
			"servers active in the last 5 minutes with details and master status",
			5 * time.Minute,
			ds.Details | ds.Master,
			func(fs repositories.ServerFilterSet) bool {
				_, activeBeforeIsSet := fs.GetActiveBefore()
				activeAfter, activeAfterIsSet := fs.GetActiveAfter()
				withStatus, withStatusIsSet := fs.GetWithStatus()

				expectActiveBefore := !activeBeforeIsSet
				expectActiveAfter := activeAfterIsSet && activeAfter.After(now.Add(-5*time.Minute))
				expectWithStatus := withStatusIsSet && withStatus == ds.Details|ds.Master

				return expectActiveBefore && expectActiveAfter && expectWithStatus
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockServerRepository)
			mockRepo.On("Filter", ctx, mock.Anything).Return(repoServers, nil)

			uc := listservers.New(mockRepo)
			ucRequest := listservers.NewRequest(query.Blank, tt.recentness, tt.status)

			_, err := uc.Execute(ctx, ucRequest)
			assert.NoError(t, err)

			mockRepo.AssertCalled(t, "Filter", ctx, mock.MatchedBy(tt.wantCall))
		})
	}
}

func TestListServersUseCase_FilterByQuery(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name      string
		query     query.Query
		wantNames []string
	}{
		{
			"no filters applied",
			query.Blank,
			[]string{
				"Private Swat4 Server", "TSS COOP Swat4 Server",
				"S&G Swat4 Server", "COOP Swat4 Server", "BS Swat4 Server",
				"VIP 1.0 Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"hide passworded",
			query.MustNew([]filter.Filter{
				filter.MustNew("password", "!=", 1),
			}),
			[]string{
				"TSS COOP Swat4 Server",
				"S&G Swat4 Server", "COOP Swat4 Server", "BS Swat4 Server",
				"VIP 1.0 Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"hide full",
			query.MustNew([]filter.Filter{
				filter.MustNew("numplayers", "!=", filter.NewFieldValue("maxplayers")),
			}),
			[]string{
				"Private Swat4 Server", "TSS COOP Swat4 Server",
				"S&G Swat4 Server", "COOP Swat4 Server", "BS Swat4 Server",
				"VIP 1.0 Swat4 Server",
			},
		},
		{
			"hide empty",
			query.MustNew([]filter.Filter{
				filter.MustNew("numplayers", ">", 0),
			}),
			[]string{
				"TSS COOP Swat4 Server", "VIP 1.0 Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"hide full and hide empty",
			query.MustNew([]filter.Filter{
				filter.MustNew("numplayers", "!=", filter.NewFieldValue("maxplayers")),
				filter.MustNew("numplayers", ">", 0),
			}),
			[]string{
				"TSS COOP Swat4 Server", "VIP 1.0 Swat4 Server",
			},
		},
		{
			"coop",
			query.MustNew([]filter.Filter{
				filter.MustNew("gametype", "=", "CO-OP"),
			}),
			[]string{
				"TSS COOP Swat4 Server", "COOP Swat4 Server",
			},
		},
		{
			"coop not empty",
			query.MustNew([]filter.Filter{
				filter.MustNew("gametype", "=", "CO-OP"),
				filter.MustNew("numplayers", ">", 0),
			}),
			[]string{
				"TSS COOP Swat4 Server",
			},
		},
		{
			"coop not empty and not full",
			query.MustNew([]filter.Filter{
				filter.MustNew("gametype", "=", "CO-OP"),
				filter.MustNew("numplayers", ">", 0),
				filter.MustNew("numplayers", "!=", filter.NewFieldValue("maxplayers")),
			}),
			[]string{
				"TSS COOP Swat4 Server",
			},
		},
		{
			"coop 1.0",
			query.MustNew([]filter.Filter{
				filter.MustNew("gametype", "=", "CO-OP"),
				filter.MustNew("gamever", "=", "1.0"),
				filter.MustNew("gamevariant", "=", "SWAT 4"),
			}),
			[]string{},
		},
		{
			"coop 1.1",
			query.MustNew([]filter.Filter{
				filter.MustNew("gametype", "=", "CO-OP"),
				filter.MustNew("gamever", "=", "1.1"),
				filter.MustNew("gamevariant", "=", "SWAT 4"),
			}),
			[]string{"COOP Swat4 Server"},
		},
		{
			"coop tss",
			query.MustNew([]filter.Filter{
				filter.MustNew("gametype", "=", "CO-OP"),
				filter.MustNew("gamever", "=", "1.0"),
				filter.MustNew("gamevariant", "=", "SWAT 4X"),
			}),
			[]string{"TSS COOP Swat4 Server"},
		},
		{
			"tss",
			query.MustNew([]filter.Filter{
				filter.MustNew("gamevariant", "=", "SWAT 4X"),
			}),
			[]string{"TSS COOP Swat4 Server", "S&G Swat4 Server"},
		},
		{
			"vip tss",
			query.MustNew([]filter.Filter{
				filter.MustNew("gametype", "=", "VIP Escort"),
				filter.MustNew("gamevariant", "=", "SWAT 4X"),
			}),
			[]string{},
		},
		{
			"tss hide empty",
			query.MustNew([]filter.Filter{
				filter.MustNew("numplayers", ">", 0),
				filter.MustNew("gamevariant", "=", "SWAT 4X"),
			}),
			[]string{"TSS COOP Swat4 Server"},
		},
		{
			"tss hide full",
			query.MustNew([]filter.Filter{
				filter.MustNew("numplayers", "!=", filter.NewFieldValue("maxplayers")),
				filter.MustNew("gamevariant", "=", "SWAT 4X"),
			}),
			[]string{"TSS COOP Swat4 Server", "S&G Swat4 Server"},
		},
		{
			"1.1",
			query.MustNew([]filter.Filter{
				filter.MustNew("gamever", "=", "1.1"),
			}),
			[]string{
				"Private Swat4 Server", "COOP Swat4 Server",
				"BS Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"1.0",
			query.MustNew([]filter.Filter{
				filter.MustNew("gamever", "=", "1.0"),
			}),
			[]string{
				"TSS COOP Swat4 Server",
				"S&G Swat4 Server",
				"VIP 1.0 Swat4 Server",
			},
		},
		{
			"1.1 vanilla",
			query.MustNew([]filter.Filter{
				filter.MustNew("gamever", "=", "1.1"),
				filter.MustNew("gamevariant", "=", "SWAT 4"),
			}),
			[]string{
				"Private Swat4 Server", "COOP Swat4 Server",
				"BS Swat4 Server", "VIP Escort Swat4 Server",
			},
		},
		{
			"1.0 vanilla",
			query.MustNew([]filter.Filter{
				filter.MustNew("gamever", "=", "1.0"),
				filter.MustNew("gamevariant", "=", "SWAT 4"),
			}),
			[]string{"VIP 1.0 Swat4 Server"},
		},
		{
			"1.0 tss",
			query.MustNew([]filter.Filter{
				filter.MustNew("gamever", "=", "1.0"),
				filter.MustNew("gamevariant", "=", "SWAT 4X"),
			}),
			[]string{"TSS COOP Swat4 Server", "S&G Swat4 Server"},
		},
		{
			"1.1 tss",
			// url.Values{"gamever": []string{"1.1"}, "gamevariant": []string{"SWAT 4X"}},
			query.MustNew([]filter.Filter{
				filter.MustNew("gamever", "=", "1.1"),
				filter.MustNew("gamevariant", "=", "SWAT 4X"),
			}),
			[]string{},
		},
		{
			"1.1 vanilla sg",
			query.MustNew([]filter.Filter{
				filter.MustNew("gamever", "=", "1.1"),
				filter.MustNew("gamevariant", "=", "SWAT 4"),
				filter.MustNew("gametype", "=", "Smash And Grab"),
			}),
			[]string{},
		},
		{
			"unknown gamevariant",
			query.MustNew([]filter.Filter{
				filter.MustNew("gamevariant", "=", "Invalid"),
			}),
			[]string{},
		},
		{
			"unknown gametype",
			query.MustNew([]filter.Filter{
				filter.MustNew("gametype", "=", "Unknown"),
			}),
			[]string{},
		},
	}

	vip := factories.BuildServerWithDetails(
		"1.1.1.1", 10580, 10581,
		details.MustNewDetailsFromParams(map[string]string{
			"hostname":    "VIP Escort Swat4 Server",
			"hostport":    "10480",
			"gametype":    "VIP Escort",
			"gamevariant": "SWAT 4",
			"mapname":     "A-Bomb Nightclub",
			"gamever":     "1.1",
			"password":    "0",
			"numplayers":  "16",
			"maxplayers":  "16",
		}, nil, nil),
		ds.Master|ds.Info,
	)

	vip10 := factories.BuildServerWithDetails(
		"2.2.2.2", 10580, 10581,
		details.MustNewDetailsFromParams(map[string]string{
			"hostname":    "VIP 1.0 Swat4 Server",
			"hostport":    "10480",
			"gametype":    "VIP Escort",
			"mapname":     "The Wolcott Projects",
			"gamevariant": "SWAT 4",
			"gamever":     "1.0",
			"password":    "0",
			"numplayers":  "16",
			"maxplayers":  "18",
		}, nil, nil),
		ds.Master|ds.Info,
	)

	bs := factories.BuildServerWithDetails(
		"3.3.3.3", 10480, 10481,
		details.MustNewDetailsFromParams(map[string]string{
			"hostname":    "BS Swat4 Server",
			"hostport":    "10480",
			"gametype":    "Barricaded Suspects",
			"mapname":     "Food Wall Restaurant",
			"gamevariant": "SWAT 4",
			"gamever":     "1.1",
			"password":    "0",
			"numplayers":  "0",
			"maxplayers":  "16",
		}, nil, nil),
		ds.Master|ds.Info|ds.Details,
	)

	coop := factories.BuildServerWithDetails(
		"4.4.4.4", 10480, 10481,
		details.MustNewDetailsFromParams(map[string]string{
			"hostname":    "COOP Swat4 Server",
			"hostport":    "10480",
			"gametype":    "CO-OP",
			"mapname":     "Food Wall Restaurant",
			"gamevariant": "SWAT 4",
			"gamever":     "1.1",
			"password":    "0",
			"numplayers":  "0",
			"maxplayers":  "5",
		}, nil, nil),
		ds.Info|ds.Details,
	)

	sg := factories.BuildServerWithDetails(
		"5.5.5.5", 10480, 10481,
		details.MustNewDetailsFromParams(map[string]string{
			"hostname":    "S&G Swat4 Server",
			"hostport":    "10480",
			"gametype":    "Smash And Grab",
			"gamevariant": "SWAT 4X",
			"mapname":     "-EXP- FunTime Amusements",
			"gamever":     "1.0",
			"password":    "0",
			"numplayers":  "0",
			"maxplayers":  "16",
		}, nil, nil),
		ds.Master|ds.Info|ds.NoDetails,
	)

	coopx := factories.BuildServerWithDetails(
		"6.6.6.6", 10480, 10481,
		details.MustNewDetailsFromParams(map[string]string{
			"hostname":    "TSS COOP Swat4 Server",
			"hostport":    "10480",
			"gametype":    "CO-OP",
			"gamevariant": "SWAT 4X",
			"mapname":     "-EXP- FunTime Amusements",
			"gamever":     "1.0",
			"password":    "0",
			"numplayers":  "1",
			"maxplayers":  "10",
		}, nil, nil),
		ds.Master|ds.Info,
	)

	passworded := factories.BuildServerWithDetails(
		"7.7.7.7", 10480, 10481,
		details.MustNewDetailsFromParams(map[string]string{
			"hostname":    "Private Swat4 Server",
			"hostport":    "10480",
			"gametype":    "VIP Escort",
			"gamevariant": "SWAT 4",
			"mapname":     "A-Bomb Nightclub",
			"gamever":     "1.1",
			"password":    "1",
			"numplayers":  "0",
			"maxplayers":  "16",
		}, nil, nil),
		ds.Details|ds.Info,
	)

	repoServers := []server.Server{
		passworded,
		coopx,
		sg,
		coop,
		bs,
		vip10,
		vip,
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockServerRepository)
			mockRepo.On("Filter", ctx, mock.Anything).Return(repoServers, nil)

			uc := listservers.New(mockRepo)
			ucRequest := listservers.NewRequest(tt.query, time.Hour, ds.Info)

			result, err := uc.Execute(ctx, ucRequest)
			assert.NoError(t, err)

			actualNames := make([]string, 0, len(result))
			for _, svr := range result {
				actualNames = append(actualNames, svr.Info.Hostname)
			}

			assert.Equal(t, tt.wantNames, actualNames)

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestListServersUseCase_Errors(t *testing.T) {
	ctx := context.TODO()

	tests := []struct {
		name    string
		repoErr error
		wantErr error
	}{
		{
			"unable to obtain servers",
			errors.New("some error"),
			listservers.ErrUnableToObtainServers,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockServerRepository)
			mockRepo.On("Filter", ctx, mock.Anything).Return(nil, tt.repoErr)

			uc := listservers.New(mockRepo)
			ucRequest := listservers.NewRequest(query.Blank, time.Hour, ds.Info)

			_, err := uc.Execute(ctx, ucRequest)
			assert.ErrorIs(t, err, tt.wantErr)

			mockRepo.AssertExpectations(t)
		})
	}
}
