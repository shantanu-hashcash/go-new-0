package ingester

import (
	"context"

	"github.com/hcnet/go/ingest"
	"github.com/hcnet/go/metaarchive"

	"github.com/hcnet/go/historyarchive"
	"github.com/hcnet/go/xdr"
)

type IngesterConfig struct {
	SourceUrl         string
	NetworkPassphrase string

	CacheDir  string
	CacheSize int

	ParallelDownloads uint
}

type liteIngester struct {
	metaarchive.MetaArchive
	networkPassphrase string
}

func (i *liteIngester) PrepareRange(ctx context.Context, r historyarchive.Range) error {
	return nil
}

func (i *liteIngester) NewLedgerTransactionReader(
	ledgerCloseMeta xdr.SerializedLedgerCloseMeta,
) (LedgerTransactionReader, error) {
	reader, err := ingest.NewLedgerTransactionReaderFromLedgerCloseMeta(
		i.networkPassphrase,
		ledgerCloseMeta.MustV0())

	return &liteLedgerTransactionReader{reader}, err
}

type liteLedgerTransactionReader struct {
	*ingest.LedgerTransactionReader
}

func (reader *liteLedgerTransactionReader) Read() (LedgerTransaction, error) {
	ingestedTx, err := reader.LedgerTransactionReader.Read()
	if err != nil {
		return LedgerTransaction{}, err
	}
	return LedgerTransaction{LedgerTransaction: &ingestedTx}, nil
}

var _ Ingester = (*liteIngester)(nil) // ensure conformity to the interface
var _ LedgerTransactionReader = (*liteLedgerTransactionReader)(nil)
