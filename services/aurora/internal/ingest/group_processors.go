package ingest

import (
	"context"
	"time"

	"github.com/shantanu-hashcash/go/ingest"
	"github.com/shantanu-hashcash/go/services/aurora/internal/db2/history"
	"github.com/shantanu-hashcash/go/services/aurora/internal/ingest/processors"
	"github.com/shantanu-hashcash/go/support/db"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/xdr"
)

type runDurations map[string]time.Duration

func (d runDurations) AddRunDuration(name string, startTime time.Time) {
	d[name] += time.Since(startTime)
}

type groupChangeProcessors struct {
	processors             []auroraChangeProcessor
	processorsRunDurations runDurations
}

func newGroupChangeProcessors(processors []auroraChangeProcessor) *groupChangeProcessors {
	return &groupChangeProcessors{
		processors:             processors,
		processorsRunDurations: make(map[string]time.Duration),
	}
}

func (g groupChangeProcessors) Name() string {
	return "groupChangeProcessors"
}

func (g groupChangeProcessors) ProcessChange(ctx context.Context, change ingest.Change) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.ProcessChange(ctx, change); err != nil {
			return errors.Wrapf(err, "error in %T.ProcessChange", p)
		}
		g.processorsRunDurations.AddRunDuration(p.Name(), startTime)
	}
	return nil
}

func (g groupChangeProcessors) Commit(ctx context.Context) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.Commit(ctx); err != nil {
			return errors.Wrapf(err, "error in %T.Commit", p)
		}
		g.processorsRunDurations.AddRunDuration(p.Name(), startTime)
	}
	return nil
}

type groupTransactionProcessors struct {
	processors                []auroraTransactionProcessor
	lazyLoaders               []auroraLazyLoader
	processorsRunDurations    runDurations
	loaderRunDurations        runDurations
	loaderStats               map[string]history.LoaderStats
	transactionStatsProcessor *processors.StatsLedgerTransactionProcessor
	tradeProcessor            *processors.TradeProcessor
}

// build the group processor for all tx processors
// processors - list of processors this should include StatsLedgerTransactionProcessor and TradeProcessor
// transactionStatsProcessor - provide a direct reference to the stats processor that is in processors or nil,
//
//	group processing will reset stats as needed
//
// tradeProcessor - provide a direct reference to the trades processor in processors or nil,
//
//	so group processing will reset stats as needed
func newGroupTransactionProcessors(processors []auroraTransactionProcessor,
	lazyLoaders []auroraLazyLoader,
	transactionStatsProcessor *processors.StatsLedgerTransactionProcessor,
	tradeProcessor *processors.TradeProcessor,
) *groupTransactionProcessors {

	return &groupTransactionProcessors{
		processors:                processors,
		processorsRunDurations:    make(map[string]time.Duration),
		loaderRunDurations:        make(map[string]time.Duration),
		loaderStats:               make(map[string]history.LoaderStats),
		lazyLoaders:               lazyLoaders,
		transactionStatsProcessor: transactionStatsProcessor,
		tradeProcessor:            tradeProcessor,
	}
}

func (g groupTransactionProcessors) ProcessTransaction(lcm xdr.LedgerCloseMeta, tx ingest.LedgerTransaction) error {
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.ProcessTransaction(lcm, tx); err != nil {
			return errors.Wrapf(err, "error in %T.ProcessTransaction", p)
		}
		g.processorsRunDurations.AddRunDuration(p.Name(), startTime)
	}
	return nil
}

func (g groupTransactionProcessors) Flush(ctx context.Context, session db.SessionInterface) error {
	// need to trigger all lazy loaders to now resolve their future placeholders
	// with real db values first
	for _, loader := range g.lazyLoaders {
		startTime := time.Now()
		if err := loader.Exec(ctx, session); err != nil {
			return errors.Wrapf(err, "error during lazy loader resolution, %T.Exec", loader)
		}
		name := loader.Name()
		g.loaderRunDurations.AddRunDuration(name, startTime)
		g.loaderStats[name] = loader.Stats()
	}

	// now flush each processor which may call loader.GetNow(), which
	// required the prior loader.Exec() to have been called.
	for _, p := range g.processors {
		startTime := time.Now()
		if err := p.Flush(ctx, session); err != nil {
			return errors.Wrapf(err, "error in %T.Flush", p)
		}
		g.processorsRunDurations.AddRunDuration(p.Name(), startTime)
	}
	return nil
}

func (g *groupTransactionProcessors) ResetStats() {
	g.processorsRunDurations = make(map[string]time.Duration)
	g.loaderRunDurations = make(map[string]time.Duration)
	g.loaderStats = make(map[string]history.LoaderStats)
	if g.tradeProcessor != nil {
		g.tradeProcessor.ResetStats()
	}
	if g.transactionStatsProcessor != nil {
		g.transactionStatsProcessor.ResetStats()
	}
}

type groupTransactionFilterers struct {
	filterers []processors.LedgerTransactionFilterer
	runDurations
	droppedTransactions int64
}

func newGroupTransactionFilterers(filterers []processors.LedgerTransactionFilterer) *groupTransactionFilterers {
	return &groupTransactionFilterers{
		filterers:    filterers,
		runDurations: make(map[string]time.Duration),
	}
}

func (g *groupTransactionFilterers) Name() string {
	return "groupTransactionFilterers"
}

func (g *groupTransactionFilterers) FilterTransaction(ctx context.Context, tx ingest.LedgerTransaction) (bool, error) {
	for _, f := range g.filterers {
		startTime := time.Now()
		include, err := f.FilterTransaction(ctx, tx)
		if err != nil {
			return false, errors.Wrapf(err, "error in %T.FilterTransaction", f)
		}
		g.AddRunDuration(f.Name(), startTime)
		if !include {
			// filter out, we can return early
			g.droppedTransactions++
			return false, nil
		}
	}
	return true, nil
}

func (g *groupTransactionFilterers) ResetStats() {
	g.droppedTransactions = 0
	g.runDurations = make(map[string]time.Duration)
}
