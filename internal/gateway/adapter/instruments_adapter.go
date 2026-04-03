package adapter

import (
	"context"

	pb "github.com/russianinvestments/invest-api-go-sdk/proto"

	"github.com/24alert/trading-bot/internal/gateway/handlers"
	"github.com/24alert/trading-bot/internal/marketdata"
	"github.com/24alert/trading-bot/pkg/tinvest"
)

type InstrumentsAdapter struct {
	client *tinvest.Client
	mdSvc  *marketdata.Service
}

func NewInstrumentsAdapter(client *tinvest.Client, mdSvc *marketdata.Service) *InstrumentsAdapter {
	return &InstrumentsAdapter{client: client, mdSvc: mdSvc}
}

func (a *InstrumentsAdapter) GetShares(ctx context.Context) ([]handlers.InstrumentShort, error) {
	resp, err := a.client.InstrumentsServiceClient().Shares(1) // INSTRUMENT_STATUS_BASE = 1
	if err != nil {
		return nil, err
	}

	out := make([]handlers.InstrumentShort, 0, len(resp.GetInstruments()))
	for _, s := range resp.GetInstruments() {
		if s.GetExchange() == "" || !s.GetApiTradeAvailableFlag() {
			continue
		}
		out = append(out, handlers.InstrumentShort{
			UID:            s.GetUid(),
			FIGI:           s.GetFigi(),
			Ticker:         s.GetTicker(),
			ClassCode:      s.GetClassCode(),
			Name:           s.GetName(),
			Currency:       s.GetCurrency(),
			Exchange:       s.GetExchange(),
			Lot:            s.GetLot(),
			InstrumentType: "share",
			Sector:         s.GetSector(),
			MinPriceIncr:   quotationToFloat(s.GetMinPriceIncrement()),
		})
	}
	return out, nil
}

func (a *InstrumentsAdapter) GetFutures(ctx context.Context) ([]handlers.InstrumentShort, error) {
	resp, err := a.client.InstrumentsServiceClient().Futures(1) // INSTRUMENT_STATUS_BASE
	if err != nil {
		return nil, err
	}

	out := make([]handlers.InstrumentShort, 0, len(resp.GetInstruments()))
	for _, f := range resp.GetInstruments() {
		if f.GetExchange() == "" || !f.GetApiTradeAvailableFlag() {
			continue
		}
		out = append(out, handlers.InstrumentShort{
			UID:            f.GetUid(),
			FIGI:           f.GetFigi(),
			Ticker:         f.GetTicker(),
			ClassCode:      f.GetClassCode(),
			Name:           f.GetName(),
			Currency:       f.GetCurrency(),
			Exchange:       f.GetExchange(),
			Lot:            f.GetLot(),
			InstrumentType: "future",
			Sector:         f.GetBasicAsset(),
			MinPriceIncr:   quotationToFloat(f.GetMinPriceIncrement()),
		})
	}
	return out, nil
}

func (a *InstrumentsAdapter) GetClosePrices(ctx context.Context, instrumentUIDs []string) ([]handlers.ClosePrice, error) {
	closePrices, err := a.mdSvc.GetClosePrices(ctx, instrumentUIDs)
	if err != nil {
		return nil, err
	}

	out := make([]handlers.ClosePrice, 0, len(closePrices))
	for _, cp := range closePrices {
		out = append(out, handlers.ClosePrice{
			InstrumentUID: cp.InstrumentUID,
			Price:         cp.Price,
		})
	}
	return out, nil
}

func (a *InstrumentsAdapter) GetLastPricesBulk(ctx context.Context, instrumentUIDs []string) ([]handlers.LastPrice, error) {
	prices, err := a.mdSvc.GetLastPrices(ctx, instrumentUIDs)
	if err != nil {
		return nil, err
	}

	out := make([]handlers.LastPrice, 0, len(prices))
	for _, p := range prices {
		out = append(out, handlers.LastPrice{
			InstrumentUID: p.InstrumentUID,
			Price:         p.Price,
			Time:          p.Time,
		})
	}
	return out, nil
}

var _ handlers.InstrumentsService = (*InstrumentsAdapter)(nil)

func quotationToFloat(q *pb.Quotation) float64 {
	if q == nil {
		return 0
	}
	return float64(q.GetUnits()) + float64(q.GetNano())/1e9
}
