package cmd

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/leonardobiffi/go-redis-migrate/src/pusher"
	"github.com/leonardobiffi/go-redis-migrate/src/reporter"
	"github.com/leonardobiffi/go-redis-migrate/src/scanner"
	"github.com/mediocregopher/radix/v4"
	"github.com/spf13/cobra"
)

var pattern string
var scanCount, report, exportRoutines, pushRoutines int

var copyCmd = &cobra.Command{
	Use:   "copy <source> <destination>",
	Short: "Copy keys from source redis instance to destination by given pattern",
	Long:  "Copy keys from source redis instance to destination by given pattern <source> and <destination> can be provided as just `<host>:<port>` or in Redis URL format: `redis://[:<password>@]<host>:<port>[/<dbIndex>]",
	Args:  cobra.MinimumNArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Start copying")

		clientSource, err := (radix.PoolConfig{}).New(context.Background(), "tcp", args[0])
		if err != nil {
			log.Fatal(err)
		}

		clientTarget, err := (radix.PoolConfig{}).New(context.Background(), "tcp", args[1])
		if err != nil {
			log.Fatal(err)
		}

		statusReporter := reporter.NewReporter()

		redisScanner := scanner.NewScanner(
			clientSource,
			scanner.RedisScannerOpts{
				Pattern:          pattern,
				ScanCount:        scanCount,
				PullRoutineCount: exportRoutines,
			},
			statusReporter,
		)

		redisPusher := pusher.NewRedisPusher(clientTarget, redisScanner.GetDumpChannel(), statusReporter)

		waitingGroup := new(sync.WaitGroup)

		statusReporter.Start(time.Second * time.Duration(report))
		redisPusher.Start(waitingGroup, pushRoutines)
		redisScanner.Start()

		waitingGroup.Wait()
		statusReporter.Stop()
		statusReporter.Report()

		fmt.Println("Finish copying")
	},
}

func init() {
	rootCmd.AddCommand(copyCmd)

	copyCmd.Flags().StringVar(&pattern, "pattern", "*", "Match pattern for keys")
	copyCmd.Flags().IntVar(&scanCount, "scanCount", 100, "COUNT parameter for redis SCAN command")
	copyCmd.Flags().IntVar(&report, "report", 1, "Report current status every N seconds")
	copyCmd.Flags().IntVar(&exportRoutines, "exportRoutines", 30, "Number of parallel export goroutines")
	copyCmd.Flags().IntVar(&pushRoutines, "pushRoutines", 30, "Number of parallel push goroutines")
}
