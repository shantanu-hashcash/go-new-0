package txsub

import (
	"context"
	"github.com/shantanu-hashcash/go/services/aurora/internal/db2/history"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/xdr"
)

func txResultByHash(ctx context.Context, db AuroraDB, hash string) (history.Transaction, error) {
	// query history database
	var hr history.Transaction
	err := db.PreFilteredTransactionByHash(ctx, &hr, hash)
	if err == nil {
		return txResultFromHistory(hr)
	}

	if !db.NoRows(err) {
		return hr, errors.Wrap(err, "server error, could not query prefiltered transaction by hash")
	}

	err = db.TransactionByHash(ctx, &hr, hash)
	if err == nil {
		return txResultFromHistory(hr)
	}

	if !db.NoRows(err) {
		return hr, errors.Wrap(err, "server error, could not query history transaction by hash")
	}

	// if no result was found in either db, return ErrNoResults
	return hr, ErrNoResults
}

func txResultFromHistory(tx history.Transaction) (history.Transaction, error) {
	var txResult xdr.TransactionResult
	err := xdr.SafeUnmarshalBase64(tx.TxResult, &txResult)
	if err == nil {
		if !txResult.Successful() {
			err = &FailedTransactionError{
				ResultXDR: tx.TxResult,
			}
		}
	} else {
		err = errors.Wrap(err, "could not unmarshall transaction result")
	}

	return tx, err
}
