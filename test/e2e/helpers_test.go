package e2e

import (
	"context"
	"crypto/rand"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

const (
	channelName   = "fintrust"
	chaincodeName = "invoice"
)

type OrgConfig struct {
	MSPID       string
	PeerAddress string
	UserMSPDir  string
	TLSCertPath string
}

func skipIfNoNetwork(t *testing.T) {
	if os.Getenv("FINTRUST_E2E") != "1" {
		t.Skip("Skipping E2E test: set FINTRUST_E2E=1 and run with local Fabric network")
	}
}

func networkDir() string {
	if dir := os.Getenv("FINTRUST_NETWORK_DIR"); dir != "" {
		return dir
	}
	wd, _ := os.Getwd()
	return filepath.Join(wd, "..", "..", "network")
}

func organizationsDir() string {
	return filepath.Join(networkDir(), "organizations")
}

func supplierConfig() OrgConfig {
	base := filepath.Join(organizationsDir(), "supplierOrg")
	return OrgConfig{
		MSPID:       "SupplierMSP",
		PeerAddress: "localhost:7051",
		UserMSPDir:  filepath.Join(base, "users", "supplier-client", "msp"),
		TLSCertPath: filepath.Join(base, "peers", "peer0.supplier", "tls", "ca.crt"),
	}
}

func buyerConfig() OrgConfig {
	base := filepath.Join(organizationsDir(), "buyerOrg")
	return OrgConfig{
		MSPID:       "BuyerMSP",
		PeerAddress: "localhost:8051",
		UserMSPDir:  filepath.Join(base, "users", "buyer-client", "msp"),
		TLSCertPath: filepath.Join(base, "peers", "peer0.buyer", "tls", "ca.crt"),
	}
}

func financeConfig() OrgConfig {
	base := filepath.Join(organizationsDir(), "financeOrg")
	return OrgConfig{
		MSPID:       "FinanceMSP",
		PeerAddress: "localhost:9051",
		UserMSPDir:  filepath.Join(base, "users", "finance-client", "msp"),
		TLSCertPath: filepath.Join(base, "peers", "peer0.finance", "tls", "ca.crt"),
	}
}

func newGateway(t *testing.T, cfg OrgConfig) (*grpc.ClientConn, *client.Gateway) {
	t.Helper()

	certPath := filepath.Join(cfg.UserMSPDir, "signcerts", "cert.pem")
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatalf("failed to read cert: %v", err)
	}
	cert, err := identity.CertificateFromPEM(certPEM)
	if err != nil {
		t.Fatalf("failed to parse cert: %v", err)
	}

	keyPath := findPrivateKey(t, filepath.Join(cfg.UserMSPDir, "keystore"))
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("failed to read key: %v", err)
	}
	key, err := identity.PrivateKeyFromPEM(keyPEM)
	if err != nil {
		t.Fatalf("failed to parse key: %v", err)
	}

	id, err := identity.NewX509Identity(cfg.MSPID, cert)
	if err != nil {
		t.Fatalf("failed to create identity: %v", err)
	}
	sign, err := identity.NewPrivateKeySign(key)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	tlsCertPEM, err := os.ReadFile(cfg.TLSCertPath)
	if err != nil {
		t.Fatalf("failed to read TLS cert: %v", err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(tlsCertPEM) {
		t.Fatal("failed to add TLS cert to pool")
	}
	tlsCreds := credentials.NewClientTLSFromCert(certPool, "")

	conn, err := grpc.NewClient(cfg.PeerAddress, grpc.WithTransportCredentials(tlsCreds))
	if err != nil {
		t.Fatalf("failed to create gRPC connection: %v", err)
	}

	gw, err := client.Connect(id, client.WithSign(sign), client.WithClientConnection(conn))
	if err != nil {
		conn.Close()
		t.Fatalf("failed to connect gateway: %v", err)
	}

	return conn, gw
}

func findPrivateKey(t *testing.T, dir string) string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read keystore: %v", err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "_sk") {
			return filepath.Join(dir, e.Name())
		}
	}
	t.Fatalf("no private key found in %s", dir)
	return ""
}

func generateInvoiceID(prefix string) string {
	b := make([]byte, 4)
	rand.Read(b)
	return fmt.Sprintf("%s-%s", prefix, strings.ToUpper(hex.EncodeToString(b)))
}

func generateSalt() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateDocHash() string {
	b := make([]byte, 32)
	rand.Read(b)
	return "sha256:" + hex.EncodeToString(b)
}

type CommercialTerms struct {
	SchemaVersion string `json:"schemaVersion"`
	InvoiceID     string `json:"invoiceId"`
	AmountMinor   int64  `json:"amountMinor"`
	Currency      string `json:"currency"`
	DueDate       string `json:"dueDate"`
	PaymentTerms  string `json:"paymentTerms"`
	Salt          string `json:"salt"`
}

type PaymentDetails struct {
	InvoiceID         string `json:"invoiceId"`
	AccountName       string `json:"accountName"`
	BankName          string `json:"bankName"`
	AccountIdentifier string `json:"accountIdentifier"`
	RoutingCode       string `json:"routingCode"`
	PaymentReference  string `json:"paymentReference"`
	Salt              string `json:"salt"`
}

type FinancingRequest struct {
	InvoiceID            string `json:"invoiceId"`
	RequestedAmountMinor int64  `json:"requestedAmountMinor"`
	RequestedTenor       string `json:"requestedTenor"`
	Salt                 string `json:"salt"`
}

type DisbursementDetails struct {
	InvoiceID         string `json:"invoiceId"`
	AccountName       string `json:"accountName"`
	BankName          string `json:"bankName"`
	AccountIdentifier string `json:"accountIdentifier"`
	RoutingCode       string `json:"routingCode"`
	Salt              string `json:"salt"`
}

type FinancingAgreement struct {
	InvoiceID           string `json:"invoiceId"`
	FinancedAmountMinor int64  `json:"financedAmountMinor"`
	DiscountBps         int    `json:"discountBps"`
	MaturityTerms       string `json:"maturityTerms"`
	Salt                string `json:"salt"`
}

type Invoice struct {
	DocType              string `json:"docType"`
	SchemaVersion        string `json:"schemaVersion"`
	InvoiceID            string `json:"invoiceId"`
	SupplierMSPID        string `json:"supplierMspId"`
	BuyerMSPID           string `json:"buyerMspId"`
	DocumentHash         string `json:"documentHash"`
	Status               string `json:"status"`
	Financed             bool   `json:"financed"`
	FinancierMSPID       string `json:"financierMspId,omitempty"`
	CreatedAt            string `json:"createdAt"`
	ApprovedAt           string `json:"approvedAt,omitempty"`
	RejectedAt           string `json:"rejectedAt,omitempty"`
	FinancingRequestedAt string `json:"financingRequestedAt,omitempty"`
	FinancedAt           string `json:"financedAt,omitempty"`
	SettledAt            string `json:"settledAt,omitempty"`
	UpdatedAt            string `json:"updatedAt"`
	LastTxID             string `json:"lastTxId"`
}

func makeCommercialTerms(invoiceID, suffix string, amount int64) CommercialTerms {
	return CommercialTerms{
		SchemaVersion: "1.0",
		InvoiceID:     invoiceID,
		AmountMinor:   amount,
		Currency:      "USD",
		DueDate:       "2026-12-31",
		PaymentTerms:  fmt.Sprintf("CONFIDENTIAL-NET-30-%s", suffix),
		Salt:          generateSalt(),
	}
}

func makePaymentDetails(invoiceID, suffix string) PaymentDetails {
	return PaymentDetails{
		InvoiceID:         invoiceID,
		AccountName:       fmt.Sprintf("SECRET-ACCOUNT-%s", suffix),
		BankName:          fmt.Sprintf("SECRET-BANK-%s", suffix),
		AccountIdentifier: fmt.Sprintf("SECRET-IBAN-%s", suffix),
		RoutingCode:       "ABCD1234",
		PaymentReference:  fmt.Sprintf("SECRET-REF-%s", suffix),
		Salt:              generateSalt(),
	}
}

func makeFinancingRequest(invoiceID, suffix string, amount int64) FinancingRequest {
	return FinancingRequest{
		InvoiceID:            invoiceID,
		RequestedAmountMinor: amount,
		RequestedTenor:       fmt.Sprintf("SECRET-TENOR-%s", suffix),
		Salt:                 generateSalt(),
	}
}

func makeDisbursementDetails(invoiceID, suffix string) DisbursementDetails {
	return DisbursementDetails{
		InvoiceID:         invoiceID,
		AccountName:       fmt.Sprintf("SECRET-DISB-ACCOUNT-%s", suffix),
		BankName:          fmt.Sprintf("SECRET-DISB-BANK-%s", suffix),
		AccountIdentifier: fmt.Sprintf("SECRET-DISB-IBAN-%s", suffix),
		RoutingCode:       "WXYZ9876",
		Salt:              generateSalt(),
	}
}

func makeFinancingAgreement(invoiceID, suffix string, amount int64) FinancingAgreement {
	return FinancingAgreement{
		InvoiceID:           invoiceID,
		FinancedAmountMinor: amount,
		DiscountBps:         250,
		MaturityTerms:       fmt.Sprintf("SECRET-FINANCE-TERMS-%s", suffix),
		Salt:                generateSalt(),
	}
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func createInvoice(t *testing.T, contract *client.Contract, invoiceID, docHash string, ct CommercialTerms, pd PaymentDetails) {
	t.Helper()
	_, err := contract.Submit("CreateInvoice",
		client.WithArguments(invoiceID, "BuyerMSP", docHash),
		client.WithTransient(map[string][]byte{
			"commercial_terms": mustJSON(ct),
			"payment_details":  mustJSON(pd),
		}),
	)
	if err != nil {
		t.Fatalf("CreateInvoice failed: %v", err)
	}
}

func approveInvoice(t *testing.T, contract *client.Contract, invoiceID string) {
	t.Helper()
	_, err := contract.Submit("ApproveInvoice", client.WithArguments(invoiceID))
	if err != nil {
		t.Fatalf("ApproveInvoice failed: %v", err)
	}
}

func rejectInvoice(t *testing.T, contract *client.Contract, invoiceID string) {
	t.Helper()
	_, err := contract.Submit("RejectInvoice", client.WithArguments(invoiceID))
	if err != nil {
		t.Fatalf("RejectInvoice failed: %v", err)
	}
}

func requestFinancing(t *testing.T, contract *client.Contract, invoiceID string, ct CommercialTerms, fr FinancingRequest, dd DisbursementDetails) {
	t.Helper()
	_, err := contract.Submit("RequestFinancing",
		client.WithArguments(invoiceID),
		client.WithTransient(map[string][]byte{
			"invoice_disclosure":   mustJSON(ct),
			"financing_request":    mustJSON(fr),
			"disbursement_details": mustJSON(dd),
		}),
	)
	if err != nil {
		t.Fatalf("RequestFinancing failed: %v", err)
	}
}

func financeInvoice(t *testing.T, contract *client.Contract, invoiceID string, fa FinancingAgreement) {
	t.Helper()
	_, err := contract.Submit("FinanceInvoice",
		client.WithArguments(invoiceID),
		client.WithTransient(map[string][]byte{
			"financing_agreement": mustJSON(fa),
		}),
	)
	if err != nil {
		t.Fatalf("FinanceInvoice failed: %v", err)
	}
}

func settleInvoice(t *testing.T, contract *client.Contract, invoiceID string) {
	t.Helper()
	_, err := contract.Submit("SettleInvoice", client.WithArguments(invoiceID))
	if err != nil {
		t.Fatalf("SettleInvoice failed: %v", err)
	}
}

func readPublicInvoice(t *testing.T, contract *client.Contract, invoiceID string) Invoice {
	t.Helper()
	result, err := contract.EvaluateTransaction("ReadPublicInvoice", invoiceID)
	if err != nil {
		t.Fatalf("ReadPublicInvoice failed: %v", err)
	}
	var jsonStr string
	if err := json.Unmarshal(result, &jsonStr); err != nil {
		jsonStr = string(result)
	}
	var inv Invoice
	if err := json.Unmarshal([]byte(jsonStr), &inv); err != nil {
		t.Fatalf("failed to parse invoice: %v", err)
	}
	return inv
}

func submitWithError(contract *client.Contract, fn string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := contract.SubmitWithContext(ctx, fn, client.WithArguments(args...))
	return err
}

func submitTransientWithError(contract *client.Contract, fn string, transient map[string][]byte, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := contract.SubmitWithContext(ctx, fn,
		client.WithArguments(args...),
		client.WithTransient(transient),
	)
	return err
}
