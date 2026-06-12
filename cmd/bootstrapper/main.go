package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"

	"internalwallet/common/security"
	"internalwallet/common/utils"
)

// This binary is intended to run inside the production cluster as a Kubernetes Job.
// It MUST NOT print any sensitive values (admin password) to stdout/stderr.
// Exception: local debug can enable --skip-aws / BOOTSTRAP_SKIP_AWS to print creds.

type bootstrapConfig struct {
	// DB
	dbHost string
	dbPort string
	dbUser string
	dbPass string
	dbName string

	// Admin
	adminName        string
	adminRoleCode    string
	adminEmailDomain string

	// AWS Secrets Manager output
	secretsEnv      string
	secretID        string
	awsRegion       string
	awsAccountID    string
	bootstrapPrefix string

	// Snowflake node id
	nodeID  int
	skipAWS bool
}

func main() {
	// Flags for local debug (still no sensitive output).
	var (
		flagDBHost           = flag.String("db-host", envDefault("DB_HOST", ""), "DB host (required)")
		flagDBPort           = flag.String("db-port", envDefault("DB_PORT", "3306"), "DB port")
		flagDBUser           = flag.String("db-user", envDefault("DB_USER", "root"), "DB user")
		flagDBPass           = flag.String("db-password", envDefault("DB_PASSWORD", ""), "DB password (required)")
		flagDBName           = flag.String("db-name", envDefault("DB_NAME", "crypto_wallet"), "DB name")
		flagAdminName        = flag.String("admin-name", envDefault("ADMIN_NAME", "Bootstrap Admin"), "Admin display name")
		flagAdminRoleCode    = flag.String("admin-role-code", envDefault("ADMIN_ROLE_CODE", "super_admin"), "RBAC role code to assign")
		flagAdminEmailDomain = flag.String("admin-email-domain", envDefault("ADMIN_EMAIL_DOMAIN", "zinkapi.com"), "Admin email domain for generated username")

		flagSecretsEnv = flag.String("secrets-env", envDefault("SECRETS_ENV", "production"), "Secrets environment (e.g. production)")
		flagSecretID   = flag.String("secret-id", envDefault("SECRET_ID", "crypto-wallet/production/admin-bootstrap"), "AWS Secrets Manager secret id/name to write admin creds")
		flagAWSRegion  = flag.String("aws-region", envDefault("AWS_REGION", "ap-northeast-1"), "AWS region")
		flagAccountID  = flag.String("aws-account-id", envDefault("AWS_ACCOUNT_ID", "301918028034"), "AWS account id (optional, used for notes only)")
		flagPrefix     = flag.String("bootstrap-prefix", envDefault("BOOTSTRAP_PREFIX", "bootstrap-admin"), "Username prefix for generated admin")

		flagSkipAWS = flag.Bool("skip-aws", envDefaultBool("BOOTSTRAP_SKIP_AWS", false), "Skip writing to AWS Secrets Manager; print admin creds to stderr instead")
		flagSnowflakeNodeID = flag.Int("node-id", envDefaultInt("SNOWFLAKE_NODE_ID", 97), "Snowflake node id")
	)
	flag.Parse()

	cfg := bootstrapConfig{
		dbHost:           strings.TrimSpace(*flagDBHost),
		dbPort:           strings.TrimSpace(*flagDBPort),
		dbUser:           strings.TrimSpace(*flagDBUser),
		dbPass:           *flagDBPass,
		dbName:           strings.TrimSpace(*flagDBName),
		adminName:        strings.TrimSpace(*flagAdminName),
		adminRoleCode:    strings.TrimSpace(*flagAdminRoleCode),
		adminEmailDomain: strings.TrimSpace(*flagAdminEmailDomain),

		secretsEnv:      strings.TrimSpace(*flagSecretsEnv),
		secretID:        strings.TrimSpace(*flagSecretID),
		awsRegion:       strings.TrimSpace(*flagAWSRegion),
		awsAccountID:    strings.TrimSpace(*flagAccountID),
		bootstrapPrefix: strings.TrimSpace(*flagPrefix),

		nodeID:  *flagSnowflakeNodeID,
		skipAWS: *flagSkipAWS,
	}

	must(cfg.dbHost != "", "DB_HOST is required")
	must(cfg.dbPass != "", "DB_PASSWORD is required")
	must(cfg.dbName != "", "DB_NAME is required")
	must(cfg.adminEmailDomain != "", "ADMIN_EMAIL_DOMAIN is required")
	must(cfg.bootstrapPrefix != "", "BOOTSTRAP_PREFIX is required")
	if !cfg.skipAWS {
		must(cfg.secretID != "", "SECRET_ID is required when not skipping AWS")
		must(cfg.awsRegion != "", "AWS_REGION is required when not skipping AWS")
	}

	if err := utils.Init(int64(cfg.nodeID)); err != nil {
		fatal("init snowflake", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	db, err := sql.Open("mysql", mysqlDSN(cfg))
	if err != nil {
		fatal("open mysql", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)

	if err := pingDB(ctx, db); err != nil {
		fatal("ping mysql", err)
	}

	// 1) Generate admin credentials (always new)
	adminUsername, adminPassword, err := generateAdminCredentials(ctx, db, cfg)
	if err != nil {
		fatal("generate admin credentials", err)
	}

	// 2) Create admin in DB (stores only password hash)
	adminID, err := createAdmin(ctx, db, cfg, adminUsername, adminPassword)
	if err != nil {
		fatal("create admin", err)
	}

	// 3) Write to AWS Secrets Manager or print creds to stderr
	if cfg.skipAWS {
		fmt.Fprintf(os.Stderr, "bootstrap completed (AWS skipped); admin created in DB only.\n")
		fmt.Fprintf(os.Stderr, "admin_username=%s\n", adminUsername)
		fmt.Fprintf(os.Stderr, "admin_password=%s\n", adminPassword)
		fmt.Fprintf(os.Stderr, "admin_id=%d\n", adminID)
	} else {
		if err := putAdminCredsToSecretsManager(ctx, cfg, adminUsername, adminPassword, adminID); err != nil {
			fatal("write secretsmanager", err)
		}
		// No sensitive output here (do not print username/password).
		fmt.Fprintf(os.Stderr, "bootstrap completed; admin created and stored in secrets manager secret_id=%s\n", cfg.secretID)
	}
}

func createAdmin(ctx context.Context, db *sql.DB, cfg bootstrapConfig, username, plainPassword string) (adminID int64, _ error) {
	roleID, err := queryRoleID(ctx, db, cfg.adminRoleCode)
	if err != nil {
		return 0, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Local()

	adminID = utils.GenerateID()
	// Insert new admin user with 2FA disabled and require_password_change=true
	_, err = tx.ExecContext(ctx, `
INSERT INTO admin_users
  (id, created_at, updated_at, deleted_at, username, password_hash, name, role, status,
   two_factor_secret, two_factor_pending_secret, two_factor_enabled, two_factor_bound_at,
   require_password_change, two_factor_required, failed_login_count, lock_until,
   last_login_at, last_login_ip, password_changed_at, token_version, created_by, updated_by)
VALUES
  (?, ?, ?, NULL, ?, ?, ?, ?, 'active',
   '', '', 0, NULL,
   1, 0, 0, NULL,
   NULL, '', ?, 1, 0, 0)
`, adminID, now, now, username, string(hash), cfg.adminName, cfg.adminRoleCode, now)
	if err != nil {
		return 0, err
	}

	// Soft-delete any existing role relations, then (re)assign the requested role.
	_, err = tx.ExecContext(ctx, `UPDATE admin_user_role SET deleted_at=? WHERE user_id=? AND deleted_at IS NULL`, now, adminID)
	if err != nil {
		return 0, err
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO admin_user_role (user_id, role_id, created_at, updated_at, deleted_at)
VALUES (?, ?, ?, ?, NULL)
`, adminID, roleID, now, now)
	if err != nil {
		// if duplicate active row exists (race), ignore
		if !isMySQLErrorDuplicate(err) {
			return 0, err
		}
	}

	// Password history (best-effort; do not fail bootstrap if missing table)
	hID := utils.GenerateID()
	_, _ = tx.ExecContext(ctx, `
INSERT INTO admin_password_histories (id, created_at, updated_at, deleted_at, admin_id, password_hash)
VALUES (?, ?, ?, NULL, ?, ?)
`, hID, now, now, adminID, string(hash))

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return adminID, nil
}

func generateAdminCredentials(ctx context.Context, db *sql.DB, cfg bootstrapConfig) (username string, password string, _ error) {
	// Generate unique username (best-effort) and a strong password.
	for i := 0; i < 20; i++ {
		u, err := generateAdminUsername(cfg.bootstrapPrefix, cfg.adminEmailDomain)
		if err != nil {
			return "", "", err
		}
		_, found, err := findAdminIDByUsername(ctx, db, u)
		if err != nil {
			return "", "", err
		}
		if found {
			continue
		}
		p, err := generateTempPassword(16)
		if err != nil {
			return "", "", err
		}
		return u, p, nil
	}
	return "", "", errors.New("failed to generate unique admin username")
}

func generateAdminUsername(prefix, domain string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	domain = strings.TrimSpace(domain)
	if prefix == "" {
		prefix = "bootstrap-admin"
	}
	if domain == "" {
		domain = "zinkapi.com"
	}
	suffix, err := randLowerAlphaNum(10)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s-%s@%s", prefix, suffix, domain), nil
}

func randLowerAlphaNum(n int) (string, error) {
	if n <= 0 {
		n = 10
	}
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	out := make([]byte, n)
	for i := range b {
		out[i] = letters[int(b[i])%len(letters)]
	}
	return string(out), nil
}

type adminBootstrapSecretPayload struct {
	AdminUsername   string `json:"admin_username"`
	AdminPassword   string `json:"admin_password"`
	AdminID         int64  `json:"admin_id"`
	GeneratedAtUTC  string `json:"generated_at_utc"`
	SecretsEnv      string `json:"secrets_env,omitempty"`
	Database        string `json:"db_name,omitempty"`
	AWSRegion       string `json:"aws_region,omitempty"`
	AWSAccountID    string `json:"aws_account_id,omitempty"`
	BootstrapPrefix string `json:"bootstrap_prefix,omitempty"`
}

func putAdminCredsToSecretsManager(ctx context.Context, cfg bootstrapConfig, username, password string, adminID int64) error {
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(cfg.awsRegion))
	if err != nil {
		return err
	}
	client := secretsmanager.NewFromConfig(awsCfg)

	payload := adminBootstrapSecretPayload{
		AdminUsername:   username,
		AdminPassword:   password,
		AdminID:         adminID,
		GeneratedAtUTC:  time.Now().UTC().Format(time.RFC3339),
		SecretsEnv:      cfg.secretsEnv,
		Database:        cfg.dbName,
		AWSRegion:       cfg.awsRegion,
		AWSAccountID:    cfg.awsAccountID,
		BootstrapPrefix: cfg.bootstrapPrefix,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	secretString := string(b)

	_, err = client.DescribeSecret(ctx, &secretsmanager.DescribeSecretInput{
		SecretId: aws.String(cfg.secretID),
	})
	if err != nil {
		var notFound *smtypes.ResourceNotFoundException
		if errors.As(err, &notFound) {
			_, cErr := client.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
				Name:         aws.String(cfg.secretID),
				SecretString: aws.String(secretString),
			})
			return cErr
		}
		return err
	}

	_, err = client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(cfg.secretID),
		SecretString: aws.String(secretString),
	})
	return err
}

func queryRoleID(ctx context.Context, db sqlQuerier, roleCode string) (int64, error) {
	var id int64
	err := db.QueryRowContext(ctx, `
SELECT id FROM admin_role
WHERE deleted_at IS NULL AND status = 1 AND code = ?
LIMIT 1
`, roleCode).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("admin_role not found for code=%s: %w", roleCode, err)
	}
	return id, nil
}

type sqlQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func findAdminIDByUsername(ctx context.Context, q sqlQuerier, username string) (id int64, found bool, _ error) {
	err := q.QueryRowContext(ctx, `
SELECT id FROM admin_users
WHERE deleted_at IS NULL AND username = ?
LIMIT 1
`, username).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return id, true, nil
}

func pingDB(ctx context.Context, db *sql.DB) error {
	deadline := time.Now().Add(60 * time.Second)
	for {
		if err := db.PingContext(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("db ping timeout")
		}
		time.Sleep(2 * time.Second)
	}
}

func mysqlDSN(cfg bootstrapConfig) string {
	c := mysql.NewConfig()
	c.Net = "tcp"
	c.Addr = fmt.Sprintf("%s:%s", cfg.dbHost, cfg.dbPort)
	c.User = cfg.dbUser
	c.Passwd = cfg.dbPass
	c.DBName = cfg.dbName
	c.Params = map[string]string{
		"charset":         "utf8mb4",
		"parseTime":       "True",
		"loc":             "Local",
		"multiStatements": "false",
	}
	return c.FormatDSN()
}

func generateTempPassword(length int) (string, error) {
	// Reuse the hardened implementation (handles RNG errors and avoids modulo bias).
	return security.GenerateSecurePassword(length)
}

func isMySQLErrorDuplicate(err error) bool {
	// go-sql-driver/mysql returns *mysql.MySQLError; we avoid importing the type here.
	// Fallback: string match on "Duplicate entry".
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate") && strings.Contains(strings.ToLower(err.Error()), "entry")
}

func envDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envDefaultInt(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	var out int
	_, err := fmt.Sscanf(v, "%d", &out)
	if err != nil {
		return def
	}
	return out
}

func envDefaultBool(k string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(k)))
	if v == "" {
		return def
	}
	switch v {
	case "1", "true", "t", "yes", "y", "on":
		return true
	case "0", "false", "f", "no", "n", "off":
		return false
	default:
		return def
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func must(ok bool, msg string) {
	if ok {
		return
	}
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}

func fatal(step string, err error) {
	fmt.Fprintf(os.Stderr, "bootstrapper failed at %s: %v\n", step, err)
	os.Exit(1)
}
