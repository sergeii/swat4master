package commander

import (
	"fmt"
	"os"
	"time"

	"github.com/alecthomas/kong"

	"github.com/sergeii/swat4master/cmd/swat4master/build"
)

type Globals struct {
	LogLevel  string `default:"info"    enum:"debug,info,warn,error" help:"Sets the minimum severity level for log messages"` // nolint:lll
	LogOutput string `default:"console" enum:"console,stdout,json"   help:"Specifies the format for log output"`

	RedisURL string `default:"redis://localhost:6379" help:"Defines the Redis URL connection"`

	ExporterHTTPListenAddress   string        `default:":9000" help:"Sets the address where the Prometheus exporter server listens for requests"`            // nolint:lll
	ExporterHTTPReadTimeout     time.Duration `default:"5s"    help:"Sets the maximum duration to read the request body before timing out"`                  // nolint:lll
	ExporterHTTPWriteTimeout    time.Duration `default:"5s"    help:"Sets the maximum duration to write a response before timing out"`                       // nolint:lll
	ExporterHTTPShutdownTimeout time.Duration `default:"10s"   help:"The amount of time the server will wait gracefully closing connections before exiting"` // nolint:lll

	BrowsingServerLiveness time.Duration `default:"3m" help:"Determines the maximum time a game server can remain unseen before being considered offline"` // nolint:lll

	DiscoveryRefreshInterval time.Duration `default:"5s" help:"Sets how frequently game server details are refreshed"`
	DiscoveryRefreshRetries  int           `default:"4"  help:"Specifies how many times a failed server details refresh should be retried"` // nolint:lll

	DiscoveryRevivalInterval  time.Duration `default:"10m"     help:"Sets how often delisted servers are checked for possible revival"`                            // nolint:lll
	DiscoveryRevivalScope     time.Duration `default:"1h"      help:"Limits how long a delisted server remains eligible for revival after it was last seen"`       // nolint:lll
	DiscoveryRevivalCountdown time.Duration `default:"5m"      help:"Sets the maximum random delay to stagger revival probes"`                                     // nolint:lll
	DiscoveryRevivalPorts     []int         `default:"1,2,3,4" help:"Defines port offsets to probe when searching for a server's query port (e.g., +1, +2, etc.)"` // nolint:lll
	DiscoveryRevivalRetries   int           `default:"2"       help:"Sets how many times a failed revival probe should be retried"`                                // nolint:lll

	ProbePollSchedule time.Duration `default:"250ms" help:"Determines how often the system checks for pending discovery probes"`          // nolint:lll
	ProbeTimeout      time.Duration `default:"1s"    help:"Sets the maximum time to wait for a response from a discovery probe"`          // nolint:lll
	ProbeConcurrency  int           `default:"25"    help:"Specifies the maximum number of discovery probes that can run simultaneously"` // nolint:lll
}

type VersionCmd struct{}

func (v *VersionCmd) Run() error {
	version := fmt.Sprintf("Version: %s (%s) built at %s", build.Version, build.Commit, build.Time)
	fmt.Println(version) // nolint: forbidigo
	os.Exit(0)
	return nil
}

type RunCmd struct {
	kong.Plugins
}

type CLI struct {
	Globals

	Version VersionCmd `cmd:"" help:"Display the app version and exit"`
	Run     RunCmd     `cmd:""`
}
