package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"internalwallet/proto/pb"
	"internalwallet/services/business/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

// assetInfoCacheKeyV2 caches full AcctAsset info as JSON (v2 format).
// NOTE: use v2 key to avoid coupling with older "icon-only string" cache values.
func assetInfoCacheKeyV2(assetCode string) string {
	return fmt.Sprintf("asset:info:v2:%s", strings.ToUpper(strings.TrimSpace(assetCode)))
}

type cachedAcctAssetV2 struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	IconUrl   string `json:"icon_url"`
	Precision int32  `json:"precision"`
	Status    int32  `json:"status"`
	IsHot     bool   `json:"is_hot"`
}

func toCachedAcctAssetV2(a *pb.AcctAsset) *cachedAcctAssetV2 {
	if a == nil {
		return nil
	}
	return &cachedAcctAssetV2{
		Code:      strings.ToUpper(strings.TrimSpace(a.Code)),
		Name:      strings.TrimSpace(a.Name),
		IconUrl:   strings.TrimSpace(a.IconUrl),
		Precision: a.Precision,
		Status:    a.Status,
		IsHot:     a.IsHot,
	}
}

func (c *cachedAcctAssetV2) toPB() *pb.AcctAsset {
	if c == nil {
		return nil
	}
	return &pb.AcctAsset{
		Code:      c.Code,
		Name:      c.Name,
		IconUrl:   c.IconUrl,
		Precision: c.Precision,
		Status:    c.Status,
		IsHot:     c.IsHot,
	}
}

// GetAssetInfoWithCache returns asset info from Redis cache if possible, otherwise calls AccountingRpc.GetAsset.
// Cache TTL: 1 hour.
func GetAssetInfoWithCache(ctx context.Context, svcCtx *svc.ServiceContext, assetCode string) *pb.AcctAsset {
	assetCode = strings.ToUpper(strings.TrimSpace(assetCode))
	if assetCode == "" || svcCtx == nil || svcCtx.AccountingRpc == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	logger := logx.WithContext(ctx)

	// 1) Redis cache
	if svcCtx.RedisClient != nil {
		key := assetInfoCacheKeyV2(assetCode)
		redisCtx, redisCancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
		cached, err := svcCtx.RedisClient.Get(redisCtx, key).Result()
		redisCancel()
		if err == nil && strings.TrimSpace(cached) != "" {
			var v cachedAcctAssetV2
			if uerr := json.Unmarshal([]byte(cached), &v); uerr == nil {
				if pbItem := v.toPB(); pbItem != nil && pbItem.Code != "" {
					return pbItem
				}
			}
			// bad cache payload -> ignore (will be overwritten)
			logger.Infof("asset info cache decode failed, will refresh: asset=%s", assetCode)
		}
	}

	// 2) RPC fallback
	rpcCtx, rpcCancel := context.WithTimeout(context.Background(), 3*time.Second)
	resp, err := svcCtx.AccountingRpc.GetAsset(rpcCtx, &pb.GetAssetRequest{Code: assetCode})
	rpcCancel()
	if err != nil || resp == nil || !resp.Success || resp.Item == nil {
		return nil
	}
	item := resp.Item

	// 3) Write cache
	if svcCtx.RedisClient != nil {
		key := assetInfoCacheKeyV2(assetCode)
		if payload, merr := json.Marshal(toCachedAcctAssetV2(item)); merr == nil {
			cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
			_ = svcCtx.RedisClient.Set(cacheCtx, key, string(payload), 1*time.Hour).Err()
			cacheCancel()
		}
	}

	return item
}
