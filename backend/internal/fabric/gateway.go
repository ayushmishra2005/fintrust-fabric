package fabric

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	MSPID        string
	PeerAddress  string
	PeerHostname string
	TLSCertPath  string
	CertPath     string
	KeyPath      string
	Channel      string
	Chaincode    string
}

type Client struct {
	conn     *grpc.ClientConn
	gateway  *client.Gateway
	contract *client.Contract
	network  *client.Network
	config   Config
}

func NewClient(cfg Config) (*Client, error) {
	certPEM, err := os.ReadFile(cfg.CertPath)
	if err != nil {
		return nil, fmt.Errorf("read cert: %w", err)
	}
	cert, err := identity.CertificateFromPEM(certPEM)
	if err != nil {
		return nil, fmt.Errorf("parse cert: %w", err)
	}

	keyPath := cfg.KeyPath
	if keyPath == "" || isDir(keyPath) {
		keyPath, err = findPrivateKey(keyPath)
		if err != nil {
			return nil, err
		}
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key: %w", err)
	}
	key, err := identity.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse key: %w", err)
	}

	id, err := identity.NewX509Identity(cfg.MSPID, cert)
	if err != nil {
		return nil, fmt.Errorf("create identity: %w", err)
	}
	sign, err := identity.NewPrivateKeySign(key)
	if err != nil {
		return nil, fmt.Errorf("create signer: %w", err)
	}

	tlsCertPEM, err := os.ReadFile(cfg.TLSCertPath)
	if err != nil {
		return nil, fmt.Errorf("read TLS cert: %w", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(tlsCertPEM) {
		return nil, fmt.Errorf("add TLS cert to pool")
	}

	tlsCreds := credentials.NewClientTLSFromCert(certPool, cfg.PeerHostname)
	conn, err := grpc.NewClient(cfg.PeerAddress, grpc.WithTransportCredentials(tlsCreds))
	if err != nil {
		return nil, fmt.Errorf("create grpc connection: %w", err)
	}

	gw, err := client.Connect(id, client.WithSign(sign), client.WithClientConnection(conn))
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("connect gateway: %w", err)
	}

	network := gw.GetNetwork(cfg.Channel)
	contract := network.GetContract(cfg.Chaincode)

	return &Client{
		conn:     conn,
		gateway:  gw,
		contract: contract,
		network:  network,
		config:   cfg,
	}, nil
}

func (c *Client) Close() {
	if c.gateway != nil {
		c.gateway.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) Submit(ctx context.Context, fn string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.contract.SubmitWithContext(ctx, fn, client.WithArguments(args...))
}

func (c *Client) SubmitWithTransient(ctx context.Context, fn string, transient map[string][]byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	return c.contract.SubmitWithContext(ctx, fn,
		client.WithArguments(args...),
		client.WithTransient(transient),
	)
}

func (c *Client) Evaluate(ctx context.Context, fn string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return c.contract.EvaluateWithContext(ctx, fn, client.WithArguments(args...))
}

type ChaincodeEvent struct {
	BlockNumber   uint64
	TransactionID string
	EventName     string
	Payload       []byte
}

func (c *Client) Events(ctx context.Context, startBlock uint64) (<-chan *ChaincodeEvent, error) {
	var opts []client.ChaincodeEventsOption
	opts = append(opts, client.WithStartBlock(startBlock))

	events, err := c.network.ChaincodeEvents(ctx, c.config.Chaincode, opts...)
	if err != nil {
		return nil, fmt.Errorf("subscribe events: %w", err)
	}

	out := make(chan *ChaincodeEvent)
	go func() {
		defer close(out)
		for evt := range events {
			select {
			case out <- &ChaincodeEvent{
				BlockNumber:   evt.BlockNumber,
				TransactionID: evt.TransactionID,
				EventName:     evt.EventName,
				Payload:       evt.Payload,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

type InvoiceEvent struct {
	InvoiceID      string `json:"invoiceId"`
	SupplierMSPID  string `json:"supplierMspId"`
	BuyerMSPID     string `json:"buyerMspId"`
	FinancierMSPID string `json:"financierMspId,omitempty"`
	Status         string `json:"status"`
	Timestamp      string `json:"timestamp"`
	TxID           string `json:"txId"`
}

func ParseEventPayload(data []byte) (*InvoiceEvent, error) {
	var evt InvoiceEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return nil, err
	}
	return &evt, nil
}

func IsFabricError(err error) bool {
	return err != nil
}

func IsNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not found") || strings.Contains(msg, "does not exist")
}

func IsAuthorizationError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "unauthorized") ||
		strings.Contains(msg, "not authorized") ||
		strings.Contains(msg, "not a party") ||
		strings.Contains(msg, "caller is not")
}

func IsConflictError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "invalid status") ||
		strings.Contains(msg, "already financed")
}

func GetTxValidationCode(err error) peer.TxValidationCode {
	if commitErr, ok := err.(*client.CommitError); ok {
		return commitErr.Code
	}
	return peer.TxValidationCode_VALID
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func findPrivateKey(dir string) (string, error) {
	if dir == "" {
		return "", fmt.Errorf("key path not specified")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", fmt.Errorf("read keystore: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_sk") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no private key found in %s", dir)
}
