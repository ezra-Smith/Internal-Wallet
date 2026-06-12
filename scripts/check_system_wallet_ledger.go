// check_system_wallet_ledger queries Accounting ledger for FundSystemWallet entries
// by chain tx hash (biz_ref format: "tx:<tx_hash>:<log_index>").
//
// Usage:
//
//	go run scripts/check_system_wallet_ledger.go -tx-hash 1b2cdda3e4eeb51d8d7d8ea6dc0776148d9698c376f43dfb159ded71ce879dd9
//
// Or with custom Accounting gRPC address:
//
//	ACCOUNTING_GRPC_ADDR=127.0.0.1:13009 go run scripts/check_system_wallet_ledger.go -tx-hash <hash>
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"internalwallet/proto/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	var (
		addr    = flag.String("addr", envOrDefault("ACCOUNTING_GRPC_ADDR", "127.0.0.1:13009"), "Accounting gRPC address")
		txHash  = flag.String("tx-hash", "", "Chain tx hash to look up (e.g. TRX tx hash)")
		timeout = flag.Duration("timeout", 10*time.Second, "Request timeout")
	)
	flag.Parse()

	txHashVal := strings.TrimSpace(*txHash)
	if txHashVal == "" {
		fmt.Fprintln(os.Stderr, "missing -tx-hash")
		os.Exit(2)
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

	// List ledger tx: biz_ref LIKE %tx_hash%, op_type = FundSystemWallet
	listResp, err := client.ListLedgerTx(ctx, &pb.ListLedgerTxRequest{
		Page:     1,
		PageSize: 20,
		BizRef:   txHashVal,
		OpType:   "FundSystemWallet",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "ListLedgerTx failed: %v\n", err)
		os.Exit(1)
	}
	if !listResp.GetSuccess() {
		fmt.Fprintf(os.Stderr, "ListLedgerTx success=false message=%s\n", listResp.GetMessage())
		os.Exit(1)
	}

	total := listResp.GetTotal()
	items := listResp.GetItems()

	fmt.Printf("Ledger entries for tx_hash=%s (FundSystemWallet): total=%d\n", txHashVal, total)
	if total == 0 {
		fmt.Println("No matching ledger entry. Possible causes: Business consumer not processed this tx yet, or source is vault (skipped), or monitored address is not the system hot wallet.")
		os.Exit(0)
	}

	for i, it := range items {
		fmt.Printf("\n--- [%d] tx_id=%d biz_ref=%s op_type=%s created_at=%s idempotency_key=%s\n",
			i+1, it.GetTxId(), it.GetBizRef(), it.GetOpType(), it.GetCreatedAt(), it.GetIdempotencyKey())

		detailResp, err := client.GetLedgerTxDetail(ctx, &pb.GetLedgerTxDetailRequest{TxId: it.GetTxId()})
		if err != nil {
			fmt.Printf("  GetLedgerTxDetail failed: %v\n", err)
			continue
		}
		if !detailResp.GetSuccess() {
			fmt.Printf("  GetLedgerTxDetail success=false message=%s\n", detailResp.GetMessage())
			continue
		}
		for _, p := range detailResp.GetPostings() {
			fmt.Printf("  posting: account_id=%d asset=%s bucket=%s debit_raw=%s credit_raw=%s\n",
				p.GetAccountId(), p.GetAssetCode(), p.GetBucket(), p.GetDebitRaw(), p.GetCreditRaw())
		}
	}
	fmt.Println()
}

func envOrDefault(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}
