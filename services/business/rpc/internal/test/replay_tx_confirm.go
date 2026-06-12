package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"strconv"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"internalwallet/common/mq"
)

type unconfirmedTxRow struct {
	source                string
	direction             string
	counterpartyAddress   string
	monitoredIsInternal   bool
	counterpartyIsInternal bool
	counterpartySourceBits uint8
	txHash                string
	chain                 string
	blockNumber           uint64
	blockHash             string
	fromAddress           string
	toAddress             string
	monitoredAddress      string
	value                 string
	gasPrice              string
	gasUsed               uint64
	gasFee                string
	transactionIndex      uint32
	blockTimestamp        int64
	confirmations         int32
	requiredConfirmations int32
	createdAt             time.Time
	transactionType       string
	logIndex              int32
	tokenAddress          string
	tokenName             string
	tokenSymbol           string
	tokenDecimals         uint8
	tokenAmount           string
}

func envOr(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func dsnFromEnv() string {
	host := envOr("DB_HOST", envOr("BUSINESS_MYSQL_HOST", "mariadb"))
	port := envOr("DB_PORT", envOr("BUSINESS_MYSQL_PORT", "3306"))
	user := envOr("DB_USER", envOr("BUSINESS_MYSQL_USERNAME", "crypto"))
	pass := envOr("DB_PASS", envOr("BUSINESS_MYSQL_PASSWORD", "crypto123456"))
	name := envOr("DB_NAME", envOr("BUSINESS_MYSQL_DATABASE", "crypto_wallet"))

	// loc=Local to match service conventions.
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&loc=Local", user, pass, host, port, name)
}

func chainIDFromEnvOrDefault(chain string) int64 {
	chain = strings.ToUpper(strings.TrimSpace(chain))
	chainCode := chain
	switch chain {
	case "CHAIN_TYPE_ETHEREUM", "ETHEREUM", "ETH":
		chainCode = "ETH"
	case "CHAIN_TYPE_BSC", "BSC":
		chainCode = "BSC"
	case "CHAIN_TYPE_TRON", "TRON", "TRX":
		chainCode = "TRON"
	}

	if v := strings.TrimSpace(os.Getenv("CHAIN_ID_" + chainCode)); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil && id > 0 {
			return id
		}
	}

	switch chainCode {
	case "ETH":
		return 1
	case "BSC":
		return 56
	case "TRON":
		return 728126428
	default:
		return 0
	}
}

func main() {
	var (
		txHash     string
		topic      string
		brokers    string
		source     string
		monitored  string
	)

	flag.StringVar(&txHash, "tx", "", "tx hash (0x...)")
	flag.StringVar(&topic, "topic", "wallet.transactions.confirm.deposit", "kafka topic (e.g. wallet.transactions.confirm.deposit)")
	flag.StringVar(&brokers, "brokers", "kafka:9092", "kafka brokers, comma-separated")
	flag.StringVar(&source, "source", "web3", "unconfirmed_transaction.source filter (e.g. web3/deposit/vault/manual)")
	flag.StringVar(&monitored, "monitored", "", "unconfirmed_transaction.monitored_address filter (optional)")
	flag.Parse()

	txHash = strings.TrimSpace(txHash)
	if txHash == "" {
		fmt.Fprintln(os.Stderr, "--tx is required")
		os.Exit(2)
	}

	dsn := dsnFromEnv()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db failed: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctxTimeout); err != nil {
		fmt.Fprintf(os.Stderr, "db ping failed: %v\n", err)
		os.Exit(1)
	}

	row, err := loadUnconfirmedTx(db, txHash, source, monitored)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load unconfirmed_transaction failed: %v\n", err)
		os.Exit(1)
	}
	if row == nil {
		fmt.Fprintf(os.Stderr, "unconfirmed_transaction not found for tx_hash=%s\n", txHash)
		os.Exit(1)
	}

	confirmations := row.confirmations
	if confirmations < row.requiredConfirmations {
		confirmations = row.requiredConfirmations
	}

	msg := mq.TransactionConfirmMessage{
		Version:               1,
		Source:                mq.AddressMonitorSource(row.source),
		Direction:             row.direction,
		CounterpartyAddress:   row.counterpartyAddress,
		MonitoredIsInternal:   row.monitoredIsInternal,
		CounterpartyIsInternal: row.counterpartyIsInternal,
		CounterpartySourceBits: row.counterpartySourceBits,
		TxHash:                row.txHash,
		Chain:                 row.chain,
		ChainId:               chainIDFromEnvOrDefault(row.chain),
		BlockNumber:           row.blockNumber,
		BlockHash:             row.blockHash,
		FromAddress:           row.fromAddress,
		ToAddress:             row.toAddress,
		MonitoredAddress:      row.monitoredAddress,
		Value:                 row.value,
		GasPrice:              row.gasPrice,
		GasUsed:               row.gasUsed,
		GasFee:                row.gasFee,
		TransactionIndex:      row.transactionIndex,
		BlockTimestamp:        row.blockTimestamp,
		Confirmations:         confirmations,
		RequiredConfirmations: row.requiredConfirmations,
		CreatedAt:             row.createdAt,
		TransactionType:       row.transactionType,
		LogIndex:              row.logIndex,
		TokenAddress:          row.tokenAddress,
		TokenName:             row.tokenName,
		TokenSymbol:           row.tokenSymbol,
		TokenDecimals:         row.tokenDecimals,
		TokenAmount:           row.tokenAmount,
	}

	producer, err := mq.NewSaramaProducer(mq.KafkaProducerConfig{
		Brokers:     splitCSV(brokers),
		Security:    "PLAINTEXT",
		SASLMech:    "PLAIN",
		ClientID:    "replay-tx-confirm",
		Compression: "gzip",
		MaxRetries:  3,
		Timeout:     30 * time.Second,
		UseAsync:    false,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "create kafka producer failed: %v\n", err)
		os.Exit(1)
	}
	defer producer.Close()

	messageID, err := producer.SendMessage(topic, row.monitoredAddress, msg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "send kafka message failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("sent kafka message: topic=%s key=%s id=%s\n", topic, row.monitoredAddress, messageID)

	// Give consumer a moment.
	time.Sleep(2 * time.Second)

	// Basic DB checks (best-effort).
	checkWeb3Tables(db, row)
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func loadUnconfirmedTx(db *sql.DB, txHash string, source string, monitored string) (*unconfirmedTxRow, error) {
	txHash = strings.TrimSpace(txHash)
	source = strings.TrimSpace(source)
	monitored = strings.TrimSpace(monitored)

	where := "WHERE tx_hash = ?"
	args := []interface{}{txHash}
	if source != "" {
		where += " AND source = ?"
		args = append(args, source)
	}
	if monitored != "" {
		where += " AND monitored_address = ?"
		args = append(args, monitored)
	}

	q := `
SELECT
  source,
  direction,
  counterparty_address,
  monitored_is_internal,
  counterparty_is_internal,
  counterparty_source_bits,
  tx_hash,
  chain,
  block_number,
  block_hash,
  from_address,
  to_address,
  monitored_address,
  value,
  gas_price,
  gas_used,
  gas_fee,
  transaction_index,
  block_timestamp,
  confirmations,
  required_confirmations,
  created_at,
  transaction_type,
  log_index,
  token_address,
  token_name,
  token_symbol,
  token_decimals,
  token_amount
FROM unconfirmed_transaction
` + where + `
ORDER BY id DESC
LIMIT 1
`

	var (
		row          unconfirmedTxRow
		tokenDecNull sql.NullInt64
	)

	err := db.QueryRow(q, args...).Scan(
		&row.source,
		&row.direction,
		&row.counterpartyAddress,
		&row.monitoredIsInternal,
		&row.counterpartyIsInternal,
		&row.counterpartySourceBits,
		&row.txHash,
		&row.chain,
		&row.blockNumber,
		&row.blockHash,
		&row.fromAddress,
		&row.toAddress,
		&row.monitoredAddress,
		&row.value,
		&row.gasPrice,
		&row.gasUsed,
		&row.gasFee,
		&row.transactionIndex,
		&row.blockTimestamp,
		&row.confirmations,
		&row.requiredConfirmations,
		&row.createdAt,
		&row.transactionType,
		&row.logIndex,
		&row.tokenAddress,
		&row.tokenName,
		&row.tokenSymbol,
		&tokenDecNull,
		&row.tokenAmount,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if tokenDecNull.Valid && tokenDecNull.Int64 >= 0 && tokenDecNull.Int64 <= 255 {
		row.tokenDecimals = uint8(tokenDecNull.Int64)
	}
	return &row, nil
}

func checkWeb3Tables(db *sql.DB, row *unconfirmedTxRow) {
	if db == nil || row == nil {
		return
	}

	// 1) web3_balance_changes
	{
		var cnt int
		err := db.QueryRow(`SELECT COUNT(1) FROM crypto_wallet.web3_balance_changes WHERE tx_hash = ? AND deleted_at IS NULL`, row.txHash).Scan(&cnt)
		if err != nil {
			fmt.Printf("check web3_balance_changes failed: %v\n", err)
		} else {
			fmt.Printf("web3_balance_changes: count=%d (tx_hash=%s)\n", cnt, row.txHash)
		}
	}

	// 2) web3_transactions
	{
		var id sql.NullInt64
		var userAddress sql.NullString
		var network sql.NullString
		var amount sql.NullString
		err := db.QueryRow(`SELECT id, user_address, network, amount FROM crypto_wallet.web3_transactions WHERE tx_hash = ? LIMIT 1`, row.txHash).Scan(&id, &userAddress, &network, &amount)
		if err == sql.ErrNoRows {
			fmt.Printf("web3_transactions: MISSING (tx_hash=%s)\n", row.txHash)
		} else if err != nil {
			fmt.Printf("check web3_transactions failed: %v\n", err)
		} else {
			fmt.Printf("web3_transactions: id=%d user_address=%s network=%s amount=%s\n",
				id.Int64,
				strings.TrimSpace(userAddress.String),
				strings.TrimSpace(network.String),
				strings.TrimSpace(amount.String),
			)
		}
	}
}
