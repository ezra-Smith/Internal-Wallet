package logic

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/shopspring/decimal"
	"github.com/zeromicro/go-zero/core/logx"
)

type ListFiatRatesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListFiatRatesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListFiatRatesLogic {
	return &ListFiatRatesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// 资产页-法币汇率列表（1 USDT = n 法币）
func (l *ListFiatRatesLogic) ListFiatRates(in *pb.ListFiatRatesReq) (*pb.ListFiatRatesResp, error) {
	if l.svcCtx.RedisClient == nil {
		return &pb.ListFiatRatesResp{
			Success: false,
			Message: "price service unavailable",
		}, nil
	}

	fiats := normalizeFiatList(in.GetFiatCurrencies())
	if len(fiats) == 0 {
		return l.listAll(l.ctx)
	}

	items := make([]*pb.FiatRateItem, 0, len(fiats))
	missing := make([]string, 0)

	fields := make([]string, len(fiats))
	for i, fiat := range fiats {
		fields[i] = "USDT" + fiat
	}

	vals, err := l.svcCtx.RedisClient.HMGet(l.ctx, FiatTickerKey, fields...).Result()
	if err != nil {
		return &pb.ListFiatRatesResp{
			Success: false,
			Message: "price service unavailable",
		}, nil
	}

	for i, v := range vals {
		fiat := fiats[i]
		s, ok := v.(string)
		if v == nil || !ok || strings.TrimSpace(s) == "" {
			missing = append(missing, fiat)
			continue
		}

		item, ok := parseFiatRateItem(fiat, s)
		if !ok {
			missing = append(missing, fiat)
			continue
		}
		items = append(items, item)
	}

	// USD 使用 binance:tickers 真实市场价格覆盖 C2C 场外汇率
	items = l.overrideUSDWithMarketPrice(items)

	msg := "success"
	if len(missing) > 0 {
		msg = "unknown fiat currencies: " + strings.Join(missing, ",")
	}

	return &pb.ListFiatRatesResp{
		Success:               true,
		Message:               msg,
		Items:                 items,
		MissingFiatCurrencies: missing,
	}, nil
}

func (l *ListFiatRatesLogic) listAll(ctx context.Context) (*pb.ListFiatRatesResp, error) {
	fields, err := l.svcCtx.RedisClient.HKeys(ctx, FiatTickerKey).Result()
	if err != nil {
		return &pb.ListFiatRatesResp{
			Success: false,
			Message: "price service unavailable",
		}, nil
	}

	type pair struct {
		fiat  string
		field string
	}

	pairs := make([]pair, 0, len(fields))
	for _, f := range fields {
		f = strings.ToUpper(strings.TrimSpace(f))
		if !strings.HasPrefix(f, "USDT") || len(f) <= len("USDT") {
			continue
		}
		fiat := strings.TrimPrefix(f, "USDT")
		if fiat == "" {
			continue
		}
		pairs = append(pairs, pair{fiat: fiat, field: f})
	}

	if len(pairs) == 0 {
		return &pb.ListFiatRatesResp{
			Success: true,
			Message: "success",
			Items:   []*pb.FiatRateItem{},
		}, nil
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].fiat < pairs[j].fiat
	})

	sortedFields := make([]string, len(pairs))
	for i, p := range pairs {
		sortedFields[i] = p.field
	}

	vals, err := l.svcCtx.RedisClient.HMGet(ctx, FiatTickerKey, sortedFields...).Result()
	if err != nil {
		return &pb.ListFiatRatesResp{
			Success: false,
			Message: "price service unavailable",
		}, nil
	}

	items := make([]*pb.FiatRateItem, 0, len(pairs))
	for i, v := range vals {
		s, ok := v.(string)
		if v == nil || !ok || strings.TrimSpace(s) == "" {
			continue
		}

		item, ok := parseFiatRateItem(pairs[i].fiat, s)
		if !ok {
			continue
		}
		items = append(items, item)
	}

	// USD 使用 binance:tickers 真实市场价格覆盖 C2C 场外汇率
	items = l.overrideUSDWithMarketPrice(items)

	return &pb.ListFiatRatesResp{
		Success: true,
		Message: "success",
		Items:   items,
	}, nil
}

// overrideUSDWithMarketPrice 将 USD 汇率替换为 binance:tickers 的真实市场价格
// 因为 fiat:tickers 中的 USDTUSD 是 C2C 场外汇率（约1.05），不是真实市场价格（约0.998）
func (l *ListFiatRatesLogic) overrideUSDWithMarketPrice(items []*pb.FiatRateItem) []*pb.FiatRateItem {
	if l.svcCtx.RedisClient == nil {
		return items
	}

	// 查找 USD 项
	usdIndex := -1
	for i, item := range items {
		if item != nil && strings.ToUpper(item.FiatCurrency) == "USD" {
			usdIndex = i
			break
		}
	}

	if usdIndex < 0 {
		return items
	}

	// 从 binance:tickers 获取 USDTUSD 真实市场价格
	val, err := l.svcCtx.RedisClient.HGet(l.ctx, MarketTickerKey, "USDTUSD").Result()
	if err != nil {
		l.Logger.Infof("无法获取 USDTUSD 真实市场价格，保留 C2C 汇率: %v", err)
		return items
	}

	rate, ts, ok := parseUSDRateFromMarket(val)
	if !ok {
		return items
	}

	// 覆盖 USD 项
	items[usdIndex].Rate = rate
	items[usdIndex].Ts = ts

	return items
}

func normalizeFiatList(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	out := make([]string, 0, len(input))
	for _, s := range input {
		fiat := strings.ToUpper(strings.TrimSpace(s))
		if fiat == "" {
			continue
		}
		if _, ok := seen[fiat]; ok {
			continue
		}
		seen[fiat] = struct{}{}
		out = append(out, fiat)
	}
	sort.Strings(out)
	return out
}

func parseFiatRateItem(fiat string, raw string) (*pb.FiatRateItem, bool) {
	var tv FiatTickerValue
	if err := json.Unmarshal([]byte(raw), &tv); err != nil {
		return nil, false
	}

	midStr := strings.TrimSpace(tv.Mid)
	if midStr == "" {
		return nil, false
	}

	mid, err := decimal.NewFromString(midStr)
	if err != nil || !mid.GreaterThan(decimal.Zero) {
		return nil, false
	}

	return &pb.FiatRateItem{
		FiatCurrency:   fiat,
		Rate:           mid.String(),
		CurrencySymbol: strings.TrimSpace(tv.CurrencySymbol),
		Ts:             tv.Ts,
	}, true
}

// parseUSDRateFromMarket 从 binance:tickers 获取 USD 真实市场价格
// 返回 (rate, ts, ok)
func parseUSDRateFromMarket(raw string) (string, int64, bool) {
	var tv TickerValue
	if err := json.Unmarshal([]byte(raw), &tv); err != nil {
		return "", 0, false
	}

	priceStr := strings.TrimSpace(tv.Price)
	if priceStr == "" {
		return "", 0, false
	}

	price, err := decimal.NewFromString(priceStr)
	if err != nil || !price.GreaterThan(decimal.Zero) {
		return "", 0, false
	}

	return price.String(), tv.Ts, true
}
