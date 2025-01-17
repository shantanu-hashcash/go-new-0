package processors

import (
	"context"
	"io"

	"github.com/guregu/null"

	"github.com/shantanu-hashcash/go/ingest"
	"github.com/shantanu-hashcash/go/support/db"
	"github.com/shantanu-hashcash/go/support/errors"
	logpkg "github.com/shantanu-hashcash/go/support/log"
	"github.com/shantanu-hashcash/go/xdr"
)

var log = logpkg.DefaultLogger.WithField("service", "ingest")

const maxBatchSize = 100000

type ChangeProcessor interface {
	ProcessChange(ctx context.Context, change ingest.Change) error
}

type LedgerTransactionProcessor interface {
	ProcessTransaction(lcm xdr.LedgerCloseMeta, transaction ingest.LedgerTransaction) error
	Flush(ctx context.Context, session db.SessionInterface) error
}

type LedgerTransactionFilterer interface {
	Name() string
	FilterTransaction(ctx context.Context, transaction ingest.LedgerTransaction) (bool, error)
}

func StreamLedgerTransactions(
	ctx context.Context,
	txFilterer LedgerTransactionFilterer,
	filteredTxProcessor LedgerTransactionProcessor,
	txProcessor LedgerTransactionProcessor,
	reader *ingest.LedgerTransactionReader,
	ledger xdr.LedgerCloseMeta,
) error {
	for {
		tx, err := reader.Read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return errors.Wrap(err, "could not read transaction")
		}
		include, err := txFilterer.FilterTransaction(ctx, tx)
		if err != nil {
			return errors.Wrapf(
				err,
				"could not filter transaction %v",
				tx.Index,
			)
		}
		if !include {
			if err = filteredTxProcessor.ProcessTransaction(ledger, tx); err != nil {
				return errors.Wrapf(
					err,
					"could not process transaction %v",
					tx.Index,
				)
			}
			log.Debugf("Filters did not find match on transaction, dropping this tx with hash %v", tx.Result.TransactionHash.HexString())
			continue
		}

		if err = txProcessor.ProcessTransaction(ledger, tx); err != nil {
			return errors.Wrapf(
				err,
				"could not process transaction %v",
				tx.Index,
			)
		}
	}
}

func ledgerEntrySponsorToNullString(entry xdr.LedgerEntry) null.String {
	sponsoringID := entry.SponsoringID()

	var sponsor null.String
	if sponsoringID != nil {
		sponsor.SetValid((*sponsoringID).Address())
	}

	return sponsor
}
