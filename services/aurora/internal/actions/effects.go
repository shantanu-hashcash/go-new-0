package actions

import (
	"context"
	"net/http"

	auroraContext "github.com/shantanu-hashcash/go/services/aurora/internal/context"
	"github.com/shantanu-hashcash/go/services/aurora/internal/db2"
	"github.com/shantanu-hashcash/go/services/aurora/internal/db2/history"
	"github.com/shantanu-hashcash/go/services/aurora/internal/ledger"
	"github.com/shantanu-hashcash/go/services/aurora/internal/resourceadapter"
	"github.com/shantanu-hashcash/go/support/errors"
	"github.com/shantanu-hashcash/go/support/render/hal"
	"github.com/shantanu-hashcash/go/support/render/problem"
)

// EffectsQuery query struct for effects end-points
type EffectsQuery struct {
	AccountID       string `schema:"account_id" valid:"accountID,optional"`
	OperationID     uint64 `schema:"op_id" valid:"-"`
	LiquidityPoolID string `schema:"liquidity_pool_id" valid:"sha256,optional"`
	TxHash          string `schema:"tx_id" valid:"transactionHash,optional"`
	LedgerID        uint32 `schema:"ledger_id" valid:"-"`
}

// Validate runs extra validations on query parameters
func (qp EffectsQuery) Validate() error {
	count, err := countNonEmpty(
		qp.AccountID,
		qp.OperationID,
		qp.LiquidityPoolID,
		qp.TxHash,
		qp.LedgerID,
	)

	if err != nil {
		return problem.BadRequest
	}

	if count > 1 {
		return problem.MakeInvalidFieldProblem(
			"filters",
			errors.New("Use a single filter for effects, you can only use one of account_id, op_id, tx_id or ledger_id"),
		)
	}
	return nil
}

type GetEffectsHandler struct {
	LedgerState *ledger.State
}

func (handler GetEffectsHandler) GetResourcePage(w HeaderWriter, r *http.Request) ([]hal.Pageable, error) {
	pq, err := GetPageQuery(handler.LedgerState, r)
	if err != nil {
		return nil, err
	}

	err = validateCursorWithinHistory(handler.LedgerState, pq)
	if err != nil {
		return nil, err
	}

	qp := EffectsQuery{}
	err = getParams(&qp, r)
	if err != nil {
		return nil, err
	}

	historyQ, err := auroraContext.HistoryQFromRequest(r)
	if err != nil {
		return nil, err
	}

	records, err := loadEffectRecords(r.Context(), historyQ, qp, pq)
	if err != nil {
		return nil, errors.Wrap(err, "loading transaction records")
	}

	ledgers, err := loadEffectLedgers(r.Context(), historyQ, records)
	if err != nil {
		return nil, errors.Wrap(err, "loading ledgers")
	}

	var result []hal.Pageable
	for _, record := range records {
		effect, err := resourceadapter.NewEffect(r.Context(), record, ledgers[record.LedgerSequence()])
		if err != nil {
			return nil, errors.Wrap(err, "could not create effect")
		}
		result = append(result, effect)
	}

	return result, nil
}

func loadEffectRecords(ctx context.Context, hq *history.Q, qp EffectsQuery, pq db2.PageQuery) ([]history.Effect, error) {
	switch {
	case qp.AccountID != "":
		return hq.EffectsForAccount(ctx, qp.AccountID, pq)
	case qp.LiquidityPoolID != "":
		return hq.EffectsForLiquidityPool(ctx, qp.LiquidityPoolID, pq)
	case qp.OperationID > 0:
		return hq.EffectsForOperation(ctx, int64(qp.OperationID), pq)
	case qp.LedgerID > 0:
		return hq.EffectsForLedger(ctx, int32(qp.LedgerID), pq)
	case qp.TxHash != "":
		return hq.EffectsForTransaction(ctx, qp.TxHash, pq)
	default:
		return hq.Effects(ctx, pq)
	}
}

func loadEffectLedgers(ctx context.Context, hq *history.Q, effects []history.Effect) (map[int32]history.Ledger, error) {
	ledgers := &history.LedgerCache{}

	for _, e := range effects {
		ledgers.Queue(e.LedgerSequence())
	}

	if err := ledgers.Load(ctx, hq); err != nil {
		return nil, err
	}
	return ledgers.Records, nil
}
