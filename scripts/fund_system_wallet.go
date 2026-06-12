package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"internalwallet/proto/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var (
		addr    = flag.String("addr", envOrDefault("ACCOUNTING_GRPC_ADDR", "127.0.0.1:13009"), "Accounting gRPC address")
		chain   = flag.String("chain", "", "Chain code (e.g. ETH)")
		asset   = flag.String("asset", "", "Asset code (e.g. USDC)")
		amount  = flag.String("amount", "", "Amount decimal (e.g. 1.0)")
		bizRef  = flag.String("biz-ref", "", "Biz ref (optional)")
		idemKey = flag.String("idempotency-key", "", "Idempotency key (optional)")
		timeout = flag.Duration("timeout", 5*time.Second, "Request timeout")
	)
	flag.Parse()

	if *chain == "" || *asset == "" || *amount == "" {
		fmt.Fprintln(os.Stderr, "missing required flags: -chain, -asset, -amount")
		os.Exit(2)
	}
	if *idemKey == "" {
		*idemKey = fmt.Sprintf("ops:fund_system_wallet:%s:%s:%d", *chain, *asset, time.Now().Unix())
	}
	if *bizRef == "" {
		*bizRef = *idemKey
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, *addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial accounting grpc failed: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close()

	client := pb.NewAccountingClient(conn)
	resp, err := client.FundSystemWallet(ctx, &pb.FundSystemWalletRequest{
		IdempotencyKey: *idemKey,
		BizRef:         *bizRef,
		ChainCode:      *chain,
		AssetCode:      *asset,
		AmountDecimal:  *amount,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "FundSystemWallet rpc failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("success=%v message=%q tx_id=%d duplicate=%v\n", resp.GetSuccess(), resp.GetMessage(), resp.GetTxId(), resp.GetDuplicate())
	if !resp.GetSuccess() {
		os.Exit(1)
	}
}

func envOrDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}
