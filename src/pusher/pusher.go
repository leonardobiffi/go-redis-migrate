package pusher

import (
	"context"
	"log"
	"sync"

	"github.com/leonardobiffi/go-redis-migrate/src/reporter"
	"github.com/leonardobiffi/go-redis-migrate/src/scanner"
	"github.com/mediocregopher/radix/v4"
)

func NewRedisPusher(client radix.Client, dumpChannel <-chan scanner.KeyDump, reporter *reporter.Reporter) *RedisPusher {
	return &RedisPusher{
		client:      client,
		reporter:    reporter,
		dumpChannel: dumpChannel,
	}
}

type RedisPusher struct {
	client      radix.Client
	reporter    *reporter.Reporter
	dumpChannel <-chan scanner.KeyDump
}

func (p *RedisPusher) Start(wg *sync.WaitGroup, number int) {
	wg.Add(number)
	for i := 0; i < number; i++ {
		go p.pushRoutine(wg)
	}

}

func (p *RedisPusher) pushRoutine(wg *sync.WaitGroup) {
	ctx := context.Background()
	for dump := range p.dumpChannel {
		p.reporter.AddPushedCounter(1)
		err := p.client.Do(ctx, radix.FlatCmd(nil, "RESTORE", dump.Key, dump.Ttl, dump.Value, "REPLACE"))
		if err != nil {
			log.Fatal(err)
		}
	}

	wg.Done()
}
