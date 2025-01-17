package actions

import (
	"context"
	"embed"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/shantanu-hashcash/go/support/log"
	"github.com/shantanu-hashcash/go/support/render/hal"
	supportProblem "github.com/shantanu-hashcash/go/support/render/problem"
)

var (
	//go:embed static
	staticFiles embed.FS
	//lint:ignore U1000 temporary
	requestCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "aurora_lite_request_count",
		Help: "How many requests have occurred?",
	})
	//lint:ignore U1000 temporary
	requestTime = promauto.NewHistogram(prometheus.HistogramOpts{
		Name: "aurora_lite_request_duration",
		Help: "How long do requests take?",
		Buckets: append(
			prometheus.LinearBuckets(0, 50, 20),
			prometheus.LinearBuckets(1000, 1000, 8)...,
		),
	})
)

type order string

const (
	orderAsc  order = "asc"
	orderDesc order = "desc"
)

type pagination struct {
	Limit  uint64
	Cursor int64
	Order  order
}

func sendPageResponse(ctx context.Context, w http.ResponseWriter, page hal.Page) {
	w.Header().Set("Content-Type", "application/hal+json; charset=utf-8")
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(page)
	if err != nil {
		log.Error(err)
		sendErrorResponse(ctx, w, supportProblem.ServerError)
	}
}

func sendErrorResponse(ctx context.Context, w http.ResponseWriter, problem supportProblem.P) {
	supportProblem.Render(ctx, w, problem)
}

func requestUnaryParam(r *http.Request, paramName string) (string, error) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return "", err
	}
	return query.Get(paramName), nil
}

func paging(r *http.Request) (pagination, error) {
	paginate := pagination{
		Order: orderAsc,
	}

	if cursorRequested, err := requestUnaryParam(r, "cursor"); err != nil {
		return pagination{}, err
	} else if cursorRequested != "" {
		paginate.Cursor, err = strconv.ParseInt(cursorRequested, 10, 64)
		if err != nil {
			return pagination{}, err
		}
	}

	if limitRequested, err := requestUnaryParam(r, "limit"); err != nil {
		return pagination{}, err
	} else if limitRequested != "" {
		paginate.Limit, err = strconv.ParseUint(limitRequested, 10, 64)
		if err != nil {
			return pagination{}, err
		}
	}

	if orderRequested, err := requestUnaryParam(r, "order"); err != nil {
		return pagination{}, err
	} else if orderRequested != "" && orderRequested == string(orderDesc) {
		paginate.Order = orderDesc
	}

	return paginate, nil
}

func getURLParam(r *http.Request, key string) (string, bool) {
	rctx := chi.RouteContext(r.Context())

	if rctx == nil {
		return "", false
	}

	if len(rctx.URLParams.Keys) != len(rctx.URLParams.Values) {
		return "", false
	}

	for k := len(rctx.URLParams.Keys) - 1; k >= 0; k-- {
		if rctx.URLParams.Keys[k] == key {
			return rctx.URLParams.Values[k], true
		}
	}

	return "", false
}
