package index

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/shantanu-hashcash/go/historyarchive"
	"github.com/shantanu-hashcash/go/ingest"
	"github.com/shantanu-hashcash/go/ingest/ledgerbackend"
	"github.com/shantanu-hashcash/go/metaarchive"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/support/log"
	"github.com/shantanu-hashcash/go/support/storage"
	"github.com/shantanu-hashcash/go/xdr"
)

func BuildIndices(
	ctx context.Context,
	sourceUrl string, // where is raw txmeta coming from?
	targetUrl string, // where should the resulting indices go?
	networkPassphrase string,
	ledgerRange historyarchive.Range, // inclusive
	modules []string,
	workerCount int,
) (*IndexBuilder, error) {
	L := log.Ctx(ctx).WithField("service", "builder")

	indexStore, err := ConnectWithConfig(StoreConfig{
		URL:     targetUrl,
		Workers: uint32(workerCount),
		Log:     L.WithField("subservice", "index"),
	})
	if err != nil {
		return nil, err
	}

	// We use historyarchive as a backend here just to abstract away dealing
	// with the filesystem directly.
	source, err := historyarchive.ConnectBackend(
		sourceUrl,
		storage.ConnectOptions{
			Context:  ctx,
			S3Region: "us-east-1",
		},
	)
	if err != nil {
		return nil, err
	}

	metaArchive := metaarchive.NewMetaArchive(source)

	ledgerBackend := ledgerbackend.NewHistoryArchiveBackend(metaArchive)

	if ledgerRange.High == 0 {
		var backendErr error
		ledgerRange.High, backendErr = ledgerBackend.GetLatestLedgerSequence(ctx)
		if backendErr != nil {
			return nil, backendErr
		}
	}

	if ledgerRange.High < ledgerRange.Low {
		return nil, fmt.Errorf("invalid ledger range: %s", ledgerRange.String())
	}

	ledgerCount := 1 + (ledgerRange.High - ledgerRange.Low) // +1 bc inclusive
	parallel := int(max(1, uint32(workerCount)))

	startTime := time.Now()
	L.Infof("Creating indices for ledger range: [%d, %d] (%d ledgers)",
		ledgerRange.Low, ledgerRange.High, ledgerCount)
	L.Infof("Using %d workers", parallel)

	// Create a bunch of workers that process ledgers a checkpoint range at a
	// time (better than a ledger at a time to minimize flushes).
	wg, ctx := errgroup.WithContext(ctx)
	ch := make(chan historyarchive.Range, parallel)

	indexBuilder := NewIndexBuilder(indexStore, metaArchive, networkPassphrase)
	for _, part := range modules {
		switch part {
		case "transactions":
			indexBuilder.RegisterModule(ProcessTransaction)
		case "accounts":
			indexBuilder.RegisterModule(ProcessAccountsByCheckpoint)
		case "accounts_by_ledger":
			indexBuilder.RegisterModule(ProcessAccountsByLedger)
		case "accounts_unbacked":
			indexBuilder.RegisterModule(ProcessAccountsByCheckpointWithoutBackend)
			indexStore.ClearMemory(false)
		case "accounts_by_ledger_unbacked":
			indexBuilder.RegisterModule(ProcessAccountsByLedgerWithoutBackend)
			indexStore.ClearMemory(false)
		default:
			return indexBuilder, fmt.Errorf("unknown module '%s'", part)
		}
	}

	// Submit the work to the channels, breaking up the range into individual
	// checkpoint ranges.
	checkpoints := historyarchive.NewCheckpointManager(0)
	go func() {
		for ledger := range ledgerRange.GenerateCheckpoints(checkpoints) {
			chunk := checkpoints.GetCheckpointRange(ledger)
			chunk.High = min(chunk.High, ledgerRange.High) // don't exceed upper bound
			chunk.Low = max(chunk.Low, ledgerRange.Low)    // nor the lower bound

			ch <- chunk
		}

		close(ch)
	}()

	processed := uint64(0)
	for i := 0; i < parallel; i++ {
		wg.Go(func() error {
			for ledgerRange := range ch {
				count := (ledgerRange.High - ledgerRange.Low) + 1
				L.Debugf("Working on checkpoint range [%d, %d] (%d ledgers)",
					ledgerRange.Low, ledgerRange.High, count)

				if err := indexBuilder.Build(ctx, ledgerRange); err != nil {
					return errors.Wrapf(err,
						"building indices for ledger range [%d, %d] failed",
						ledgerRange.Low, ledgerRange.High)
				}

				nprocessed := atomic.AddUint64(&processed, uint64(count))
				if nprocessed%1234 == 0 {
					PrintProgress("Reading ledgers", nprocessed, uint64(ledgerCount), startTime)
				}

				// Upload indices once every 10 checkpoints to save memory
				if nprocessed%(10*uint64(checkpoints.GetCheckpointFrequency())) == 0 {
					if err := indexStore.Flush(); err != nil {
						return errors.Wrap(err, "flushing indices failed")
					}
				}
			}
			return nil
		})
	}

	if err := wg.Wait(); err != nil {
		return indexBuilder, errors.Wrap(err, "one or more workers failed")
	}

	PrintProgress("Reading ledgers", processed, uint64(ledgerCount), startTime)

	L.Infof("Processed %d ledgers via %d workers", processed, parallel)
	L.Infof("Uploading indices to %s", targetUrl)
	if err := indexStore.Flush(); err != nil {
		return indexBuilder, errors.Wrap(err, "flushing indices failed")
	}

	// Assertion for testing
	if processed != uint64(ledgerCount) {
		L.Warnf("processed %d but expected %d", processed, ledgerCount)
	}

	return indexBuilder, nil
}

// Module is a way to process ingested data and shove it into an index store.
type Module func(
	indexStore Store,
	ledger xdr.LedgerCloseMeta,
	transaction ingest.LedgerTransaction,
) error

// IndexBuilder contains everything needed to build indices from ledger ranges.
type IndexBuilder struct {
	store             Store
	metaArchive       metaarchive.MetaArchive
	networkPassphrase string

	lastBuiltLedgerWriteLock sync.Mutex
	lastBuiltLedger          uint32

	modules []Module
}

func NewIndexBuilder(
	indexStore Store,
	metaArchive metaarchive.MetaArchive,
	networkPassphrase string,
) *IndexBuilder {
	return &IndexBuilder{
		store:             indexStore,
		metaArchive:       metaArchive,
		networkPassphrase: networkPassphrase,
	}
}

// RegisterModule adds a module to process every given ledger. It is not
// threadsafe and all calls should be made *before* any calls to `Build`.
func (builder *IndexBuilder) RegisterModule(module Module) {
	builder.modules = append(builder.modules, module)
}

// RunModules executes all of the registered modules on the given ledger.
func (builder *IndexBuilder) RunModules(
	ledger xdr.LedgerCloseMeta,
	tx ingest.LedgerTransaction,
) error {
	for _, module := range builder.modules {
		if err := module(builder.store, ledger, tx); err != nil {
			return err
		}
	}

	return nil
}

// Build sequentially creates indices for each ledger in the given range based
// on the registered modules.
//
// TODO: We can probably optimize this by doing GetLedger in parallel with the
// ingestion & index building, since the network will be idle during the latter
// portion.
func (builder *IndexBuilder) Build(ctx context.Context, ledgerRange historyarchive.Range) error {
	for ledgerSeq := ledgerRange.Low; ledgerSeq <= ledgerRange.High; ledgerSeq++ {
		ledger, err := builder.metaArchive.GetLedger(ctx, ledgerSeq)
		if err != nil {
			if !os.IsNotExist(err) {
				log.Errorf("error getting ledger %d: %v", ledgerSeq, err)
			}
			return err
		}

		reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(
			builder.networkPassphrase, *ledger.V0)
		if err != nil {
			return err
		}

		for {
			tx, err := reader.Read()
			if err == io.EOF {
				break
			} else if err != nil {
				return err
			}

			if err := builder.RunModules(*ledger.V0, tx); err != nil {
				return err
			}
		}
	}

	builder.lastBuiltLedgerWriteLock.Lock()
	defer builder.lastBuiltLedgerWriteLock.Unlock()
	builder.lastBuiltLedger = max(builder.lastBuiltLedger, ledgerRange.High)

	return nil
}

func (builder *IndexBuilder) Watch(ctx context.Context) error {
	latestLedger, err := builder.metaArchive.GetLatestLedgerSequence(ctx)
	if err != nil {
		log.Errorf("Failed to retrieve latest ledger: %v", err)
		return err
	}
	nextLedger := builder.lastBuiltLedger + 1

	log.Infof("Catching up to latest ledger: (%d, %d]", nextLedger, latestLedger)
	if err = builder.Build(ctx, historyarchive.Range{
		Low:  nextLedger,
		High: latestLedger,
	}); err != nil {
		log.Errorf("Initial catchup failed: %v", err)
	}

	for {
		nextLedger = builder.lastBuiltLedger + 1
		log.Infof("Awaiting next ledger (%d)", nextLedger)

		// To keep the MVP simple, let's just naively poll the backend until the
		// ledger we want becomes available.
		//
		//  Refer to this thread [1] for a deeper brain dump on why we're
		//  preferring this over doing proper filesystem monitoring (e.g.
		//  fsnotify for on-disk). Essentially, supporting this for every
		//  possible index backend is a non-trivial amount of work with an
		//  uncertain payoff.
		//
		// [1]: https://hcnetfoundation.slack.com/archives/C02B04RMK/p1654903342555669

		// We sleep with linear backoff starting with 6s. Ledgers get posted
		// every 5-7s on average, but to be extra careful, let's give it a full
		// minute before we give up entirely.
		timedCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		sleepTime := (6 * time.Second)
	outer:
		for {
			time.Sleep(sleepTime)
			select {
			case <-timedCtx.Done():
				return errors.Wrap(timedCtx.Err(), "awaiting next ledger failed")

			default:
				buildErr := builder.Build(timedCtx, historyarchive.Range{
					Low:  nextLedger,
					High: nextLedger,
				})
				if buildErr == nil {
					break outer
				}

				if os.IsNotExist(buildErr) {
					sleepTime += (time.Second * 2)
					continue
				}

				return errors.Wrap(buildErr, "awaiting next ledger failed")
			}
		}
	}
}

func PrintProgress(prefix string, done, total uint64, startTime time.Time) {
	progress := float64(done) / float64(total)
	elapsed := time.Since(startTime)

	// Approximate based on how many stuff is left to do and how long this much
	// progress took, e.g. if 4/10 took 2s then 6/10 will "take" 3s (though this
	// assumes consistent load).
	remaining := (float64(elapsed) / float64(done)) * float64(total-done)

	var remainingStr string
	if math.IsInf(remaining, 0) || math.IsNaN(remaining) {
		remainingStr = "unknown"
	} else {
		remainingStr = time.Duration(remaining).Round(time.Millisecond).String()
	}

	log.Infof("%s - %.1f%% (%d/%d) - elapsed: %s, remaining: ~%s", prefix,
		100*progress, done, total,
		elapsed.Round(time.Millisecond),
		remainingStr,
	)
}

func min(a, b uint32) uint32 {
	if a < b {
		return a
	}
	return b
}

func max(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}
