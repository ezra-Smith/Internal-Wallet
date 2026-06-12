package logic

import (
	"strings"
	"testing"

	"github.com/tyler-smith/go-bip39"
)

func TestCompanyHotPrimaryDerivationPaths(t *testing.T) {
	p, err := BuildCompanyWalletPath("ETH", 1)
	if err != nil {
		t.Fatalf("BuildCompanyWalletPath ETH failed: %v", err)
	}
	if p != "m/44'/60'/0'/0/1" {
		t.Fatalf("unexpected ETH path: %s", p)
	}

	p2, err := BuildCompanyWalletPath("TRON", 1)
	if err != nil {
		t.Fatalf("BuildCompanyWalletPath TRON failed: %v", err)
	}
	if p2 != "m/44'/195'/0'/0/1" {
		t.Fatalf("unexpected TRON path: %s", p2)
	}
}

func TestCompanyHotPrimaryDerivedAddresses(t *testing.T) {
	// Deterministic seed for test: mnemonic + passphrase.
	mnemonic := "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	if !bip39.IsMnemonicValid(mnemonic) {
		t.Fatalf("test mnemonic invalid")
	}
	seed := bip39.NewSeed(mnemonic, "test-passphrase")

	ethAddr, ethPath := deriveCompanyHotPrimaryAddress(t, seed, "ETH")
	bscAddr, bscPath := deriveCompanyHotPrimaryAddress(t, seed, "BSC")
	tronAddr, tronPath := deriveCompanyHotPrimaryAddress(t, seed, "TRON")

	if ethPath != bscPath {
		t.Fatalf("expected ETH and BSC path to match; eth=%s bsc=%s", ethPath, bscPath)
	}
	if !strings.EqualFold(ethAddr, bscAddr) {
		t.Fatalf("expected ETH and BSC addresses to match (case-insensitive); eth=%s bsc=%s", ethAddr, bscAddr)
	}

	if !strings.HasPrefix(ethAddr, "0x") || len(ethAddr) != 42 {
		t.Fatalf("unexpected ETH address format: %s", ethAddr)
	}
	if !strings.HasPrefix(tronAddr, "T") || len(tronAddr) < 33 || len(tronAddr) > 35 {
		t.Fatalf("unexpected TRON address format: %s (path=%s)", tronAddr, tronPath)
	}
}

func TestCompanyWalletAddressEqual(t *testing.T) {
	if !companyWalletAddressEqual("ETH", "0xAbC", "0xabc") {
		t.Fatalf("expected evm address compare case-insensitive")
	}
	if companyWalletAddressEqual("TRON", "Tabc", "tAbc") {
		t.Fatalf("expected tron address compare case-sensitive")
	}
}

func TestVaultMetaForChain(t *testing.T) {
	m, ok := vaultMetaForChain("ETH")
	if !ok || m.ChainID != 1 || m.Network != "ETH" || m.ChainType != "ethereum" {
		t.Fatalf("unexpected eth meta: ok=%v meta=%+v", ok, m)
	}

	m, ok = vaultMetaForChain("BSC")
	if !ok || m.ChainID != 56 || m.Network != "BSC" || m.ChainType != "bsc" {
		t.Fatalf("unexpected bsc meta: ok=%v meta=%+v", ok, m)
	}

	m, ok = vaultMetaForChain("TRON")
	if !ok || m.ChainID == 0 || m.Network != "TRON" || m.ChainType != "tron" {
		t.Fatalf("unexpected tron meta: ok=%v meta=%+v", ok, m)
	}

	if _, ok := vaultMetaForChain("SOL"); ok {
		t.Fatalf("expected sol to be unsupported")
	}
}

func deriveCompanyHotPrimaryAddress(t *testing.T, seed []byte, chain string) (addr string, path string) {
	t.Helper()
	path, err := BuildCompanyWalletPath(chain, 1)
	if err != nil {
		t.Fatalf("BuildCompanyWalletPath failed: chain=%s err=%v", chain, err)
	}
	w, err := NewHDWalletFromSeed(seed, chain)
	if err != nil {
		t.Fatalf("NewHDWalletFromSeed failed: %v", err)
	}
	uncompressed, _, err := w.GetPublicKey(path)
	if err != nil {
		t.Fatalf("GetPublicKey failed: %v", err)
	}
	addr, err = PublicKeyToAddress(chain, uncompressed)
	if err != nil {
		t.Fatalf("PublicKeyToAddress failed: chain=%s err=%v", chain, err)
	}
	return strings.TrimSpace(addr), strings.TrimSpace(path)
}
