package scanner

import (
	"context"
	"log"
	"sync"

	"github.com/leonardobiffi/go-redis-migrate/src/reporter"
	"github.com/mediocregopher/radix/v4"
)

type KeyDump struct {
	Key   string
	Value string
	Ttl   int
}

type RedisScannerOpts struct {
	Pattern          string
	ScanCount        int
	PullRoutineCount int
}

type RedisScanner struct {
	client      radix.Client
	options     RedisScannerOpts
	reporter    *reporter.Reporter
	keyChannel  chan string
	dumpChannel chan KeyDump
}

func NewScanner(client radix.Client, options RedisScannerOpts, reporter *reporter.Reporter) *RedisScanner {
	return &RedisScanner{
		client:      client,
		options:     options,
		reporter:    reporter,
		dumpChannel: make(chan KeyDump),
		keyChannel:  make(chan string),
	}
}

func (s *RedisScanner) Start() {
	wgPull := new(sync.WaitGroup)
	wgPull.Add(s.options.PullRoutineCount)

	go s.scanRoutine()
	for i := 0; i < s.options.PullRoutineCount; i++ {
		go s.exportRoutine(wgPull)
	}

	wgPull.Wait()
	close(s.dumpChannel)
}

func (s *RedisScanner) GetDumpChannel() <-chan KeyDump {
	return s.dumpChannel
}

func (s *RedisScanner) scanRoutine() {
	var key string
	ctx := context.Background()

	radixScanner := (radix.ScannerConfig{
		Command: "SCAN", Count: s.options.ScanCount, Pattern: s.options.Pattern,
	}).New(s.client)

	for radixScanner.Next(ctx, &key) {
		s.reporter.AddScannedCounter(1)
		s.keyChannel <- key
	}

	close(s.keyChannel)
}

func (s *RedisScanner) exportRoutine(wg *sync.WaitGroup) {
	ctx := context.Background()
	for key := range s.keyChannel {
		var value string
		var ttl int

		p := radix.NewPipeline()
		p.Append(radix.Cmd(&ttl, "PTTL", key))
		p.Append(radix.Cmd(&value, "DUMP", key))

		if err := s.client.Do(ctx, p); err != nil {
			log.Fatal(err)
		}

		if ttl < 0 {
			ttl = 0
		}

		s.reporter.AddExportedCounter(1)
		s.dumpChannel <- KeyDump{
			Key:   key,
			Ttl:   ttl,
			Value: value,
		}
	}

	wg.Done()
}
