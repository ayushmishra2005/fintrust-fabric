package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fintrust-fabric/backend/internal/api"
	"github.com/fintrust-fabric/backend/internal/fabric"
	"github.com/fintrust-fabric/backend/internal/projection"
)

func main() {
	cfg := loadConfig()

	log.Printf("starting fintrust-api for %s", cfg.MSPID)

	store, err := projection.NewStore(cfg.DBPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	fc, err := fabric.NewClient(fabric.Config{
		MSPID:        cfg.MSPID,
		PeerAddress:  cfg.PeerAddress,
		PeerHostname: cfg.PeerHostname,
		TLSCertPath:  cfg.TLSCertPath,
		CertPath:     cfg.CertPath,
		KeyPath:      cfg.KeyPath,
		Channel:      cfg.Channel,
		Chaincode:    cfg.Chaincode,
	})
	if err != nil {
		log.Fatalf("connect fabric: %v", err)
	}
	defer fc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runEventConsumer(ctx, fc, store)

	server := api.NewServer(fc, store)
	httpServer := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      server,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("listening on %s", cfg.ListenAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}

	log.Println("shutdown complete")
}

func runEventConsumer(ctx context.Context, fc *fabric.Client, store *projection.Store) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := consumeEvents(ctx, fc, store); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("event consumer error: %v (retrying in 5s)", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func consumeEvents(ctx context.Context, fc *fabric.Client, store *projection.Store) error {
	checkpoint, exists, err := store.GetCheckpoint(ctx)
	if err != nil {
		return err
	}

	var startBlock uint64
	if exists {
		// Resume from block after checkpoint to avoid reprocessing
		startBlock = checkpoint + 1
	} else {
		// Fresh start: begin from block 0 to replay all events
		startBlock = 0
	}

	log.Printf("starting event consumer from block %d", startBlock)

	events, err := fc.Events(ctx, startBlock)
	if err != nil {
		return err
	}

	for evt := range events {
		if err := store.InsertEventAndCheckpoint(ctx, evt.BlockNumber, evt.TransactionID, evt.EventName, evt.Payload); err != nil {
			log.Printf("store event: %v", err)
		} else {
			log.Printf("event: block=%d tx=%s name=%s", evt.BlockNumber, evt.TransactionID[:8], evt.EventName)
		}
	}

	return nil
}

type Config struct {
	ListenAddr   string
	Channel      string
	Chaincode    string
	MSPID        string
	PeerAddress  string
	PeerHostname string
	TLSCertPath  string
	CertPath     string
	KeyPath      string
	DBPath       string
}

func loadConfig() Config {
	cfg := Config{
		ListenAddr:   envOr("LISTEN_ADDR", ":8080"),
		Channel:      envOr("FABRIC_CHANNEL", "fintrust"),
		Chaincode:    envOr("FABRIC_CHAINCODE", "invoice"),
		MSPID:        os.Getenv("FABRIC_MSP_ID"),
		PeerAddress:  os.Getenv("FABRIC_PEER_ADDRESS"),
		PeerHostname: os.Getenv("FABRIC_PEER_HOSTNAME"),
		TLSCertPath:  os.Getenv("FABRIC_TLS_CERT_PATH"),
		CertPath:     os.Getenv("FABRIC_CERT_PATH"),
		KeyPath:      os.Getenv("FABRIC_KEY_PATH"),
		DBPath:       envOr("DATABASE_PATH", "fintrust.db"),
	}

	org := os.Getenv("FINTRUST_ORG")
	networkDir := envOr("FINTRUST_NETWORK_DIR", findNetworkDir())

	if org != "" && networkDir != "" {
		applyOrgDefaults(&cfg, org, networkDir)
	}

	if cfg.MSPID == "" {
		log.Fatal("FABRIC_MSP_ID or FINTRUST_ORG required")
	}
	if cfg.PeerAddress == "" {
		log.Fatal("FABRIC_PEER_ADDRESS required")
	}
	if cfg.TLSCertPath == "" || cfg.CertPath == "" {
		log.Fatal("FABRIC_TLS_CERT_PATH and FABRIC_CERT_PATH required")
	}

	return cfg
}

func applyOrgDefaults(cfg *Config, org, networkDir string) {
	orgsDir := filepath.Join(networkDir, "organizations")

	switch org {
	case "supplier":
		if cfg.MSPID == "" {
			cfg.MSPID = "SupplierMSP"
		}
		if cfg.PeerAddress == "" {
			cfg.PeerAddress = "localhost:7051"
		}
		base := filepath.Join(orgsDir, "supplierOrg")
		if cfg.TLSCertPath == "" {
			cfg.TLSCertPath = filepath.Join(base, "peers", "peer0.supplier", "tls", "ca.crt")
		}
		if cfg.CertPath == "" {
			cfg.CertPath = filepath.Join(base, "users", "supplier-client", "msp", "signcerts", "cert.pem")
		}
		if cfg.KeyPath == "" {
			cfg.KeyPath = filepath.Join(base, "users", "supplier-client", "msp", "keystore")
		}
		if cfg.DBPath == "fintrust.db" {
			cfg.DBPath = "fintrust-supplier.db"
		}
		if cfg.ListenAddr == ":8080" {
			cfg.ListenAddr = ":8081"
		}

	case "buyer":
		if cfg.MSPID == "" {
			cfg.MSPID = "BuyerMSP"
		}
		if cfg.PeerAddress == "" {
			cfg.PeerAddress = "localhost:8051"
		}
		base := filepath.Join(orgsDir, "buyerOrg")
		if cfg.TLSCertPath == "" {
			cfg.TLSCertPath = filepath.Join(base, "peers", "peer0.buyer", "tls", "ca.crt")
		}
		if cfg.CertPath == "" {
			cfg.CertPath = filepath.Join(base, "users", "buyer-client", "msp", "signcerts", "cert.pem")
		}
		if cfg.KeyPath == "" {
			cfg.KeyPath = filepath.Join(base, "users", "buyer-client", "msp", "keystore")
		}
		if cfg.DBPath == "fintrust.db" {
			cfg.DBPath = "fintrust-buyer.db"
		}
		if cfg.ListenAddr == ":8080" {
			cfg.ListenAddr = ":8082"
		}

	case "finance":
		if cfg.MSPID == "" {
			cfg.MSPID = "FinanceMSP"
		}
		if cfg.PeerAddress == "" {
			cfg.PeerAddress = "localhost:9051"
		}
		base := filepath.Join(orgsDir, "financeOrg")
		if cfg.TLSCertPath == "" {
			cfg.TLSCertPath = filepath.Join(base, "peers", "peer0.finance", "tls", "ca.crt")
		}
		if cfg.CertPath == "" {
			cfg.CertPath = filepath.Join(base, "users", "finance-client", "msp", "signcerts", "cert.pem")
		}
		if cfg.KeyPath == "" {
			cfg.KeyPath = filepath.Join(base, "users", "finance-client", "msp", "keystore")
		}
		if cfg.DBPath == "fintrust.db" {
			cfg.DBPath = "fintrust-finance.db"
		}
		if cfg.ListenAddr == ":8080" {
			cfg.ListenAddr = ":8083"
		}
	}
}

func findNetworkDir() string {
	wd, _ := os.Getwd()
	candidates := []string{
		filepath.Join(wd, "network"),
		filepath.Join(wd, "..", "network"),
		filepath.Join(wd, "..", "..", "network"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			return c
		}
	}
	return ""
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
