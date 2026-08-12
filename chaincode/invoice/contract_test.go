package main

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/hyperledger/fabric-chaincode-go/v2/pkg/cid"
	"github.com/hyperledger/fabric-chaincode-go/v2/shim"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
	"github.com/hyperledger/fabric-protos-go-apiv2/ledger/queryresult"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type mockStub struct {
	state           map[string][]byte
	privateData     map[string]map[string][]byte
	transient       map[string][]byte
	events          map[string][]byte
	validationParam map[string][]byte
	txID            string
	txTimestamp     *timestamppb.Timestamp
}

func newMockStub() *mockStub {
	return &mockStub{
		state:           make(map[string][]byte),
		privateData:     make(map[string]map[string][]byte),
		transient:       make(map[string][]byte),
		events:          make(map[string][]byte),
		validationParam: make(map[string][]byte),
		txID:            "tx-123",
		txTimestamp:     &timestamppb.Timestamp{Seconds: 1700000000, Nanos: 0},
	}
}

func (s *mockStub) GetState(key string) ([]byte, error) {
	return s.state[key], nil
}

func (s *mockStub) PutState(key string, value []byte) error {
	s.state[key] = value
	return nil
}

func (s *mockStub) DelState(key string) error {
	delete(s.state, key)
	return nil
}

func (s *mockStub) GetPrivateData(collection, key string) ([]byte, error) {
	if s.privateData[collection] == nil {
		return nil, nil
	}
	return s.privateData[collection][key], nil
}

func (s *mockStub) PutPrivateData(collection, key string, value []byte) error {
	if s.privateData[collection] == nil {
		s.privateData[collection] = make(map[string][]byte)
	}
	s.privateData[collection][key] = value
	return nil
}

func (s *mockStub) GetPrivateDataHash(collection, key string) ([]byte, error) {
	data, err := s.GetPrivateData(collection, key)
	if err != nil || data == nil {
		return nil, err
	}
	hash := sha256.Sum256(data)
	return hash[:], nil
}

func (s *mockStub) GetTransient() (map[string][]byte, error) {
	return s.transient, nil
}

func (s *mockStub) SetEvent(name string, payload []byte) error {
	s.events[name] = payload
	return nil
}

func (s *mockStub) GetTxID() string {
	return s.txID
}

func (s *mockStub) GetTxTimestamp() (*timestamppb.Timestamp, error) {
	return s.txTimestamp, nil
}

func (s *mockStub) SetStateValidationParameter(key string, ep []byte) error {
	s.validationParam[key] = ep
	return nil
}

func (s *mockStub) GetQueryResult(query string) (shim.StateQueryIteratorInterface, error) {
	return &mockIterator{}, nil
}

func (s *mockStub) GetStateByRange(startKey, endKey string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (s *mockStub) GetStateByRangeWithPagination(startKey, endKey string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	return nil, nil, nil
}
func (s *mockStub) GetStateByPartialCompositeKey(objectType string, keys []string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (s *mockStub) GetStateByPartialCompositeKeyWithPagination(objectType string, keys []string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	return nil, nil, nil
}
func (s *mockStub) GetAllStatesCompositeKeyWithPagination(pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	return nil, nil, nil
}
func (s *mockStub) GetMultiplePrivateData(collection string, keys ...string) ([][]byte, error) {
	return nil, nil
}
func (s *mockStub) CreateCompositeKey(objectType string, attributes []string) (string, error) {
	return "", nil
}
func (s *mockStub) SplitCompositeKey(compositeKey string) (string, []string, error) {
	return "", nil, nil
}
func (s *mockStub) GetQueryResultWithPagination(query string, pageSize int32, bookmark string) (shim.StateQueryIteratorInterface, *peer.QueryResponseMetadata, error) {
	return nil, nil, nil
}
func (s *mockStub) GetHistoryForKey(key string) (shim.HistoryQueryIteratorInterface, error) {
	return nil, nil
}
func (s *mockStub) GetPrivateDataByRange(collection, startKey, endKey string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (s *mockStub) GetPrivateDataByPartialCompositeKey(collection, objectType string, keys []string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (s *mockStub) GetPrivateDataQueryResult(collection, query string) (shim.StateQueryIteratorInterface, error) {
	return nil, nil
}
func (s *mockStub) DelPrivateData(collection, key string) error {
	return nil
}
func (s *mockStub) PurgePrivateData(collection, key string) error {
	return nil
}
func (s *mockStub) GetStateValidationParameter(key string) ([]byte, error) {
	return s.validationParam[key], nil
}
func (s *mockStub) GetPrivateDataValidationParameter(collection, key string) ([]byte, error) {
	return nil, nil
}
func (s *mockStub) SetPrivateDataValidationParameter(collection, key string, ep []byte) error {
	return nil
}
func (s *mockStub) InvokeChaincode(chaincodeName string, args [][]byte, channel string) *peer.Response {
	return &peer.Response{}
}
func (s *mockStub) GetArgs() [][]byte                                { return nil }
func (s *mockStub) GetStringArgs() []string                          { return nil }
func (s *mockStub) GetFunctionAndParameters() (string, []string)     { return "", nil }
func (s *mockStub) GetArgsSlice() ([]byte, error)                    { return nil, nil }
func (s *mockStub) GetChannelID() string                             { return "fintrust" }
func (s *mockStub) GetCreator() ([]byte, error)                      { return nil, nil }
func (s *mockStub) GetSignedProposal() (*peer.SignedProposal, error) { return nil, nil }
func (s *mockStub) GetBinding() ([]byte, error)                      { return nil, nil }
func (s *mockStub) GetDecorations() map[string][]byte                { return nil }
func (s *mockStub) GetMSPID() (string, error)                        { return "", nil }
func (s *mockStub) GetMultipleStates(keys ...string) ([][]byte, error) {
	return nil, nil
}
func (s *mockStub) StartWriteBatch()        {}
func (s *mockStub) FinishWriteBatch() error { return nil }

type mockIterator struct {
	closed bool
}

func (i *mockIterator) HasNext() bool                  { return false }
func (i *mockIterator) Next() (*queryresult.KV, error) { return nil, nil }
func (i *mockIterator) Close() error                   { i.closed = true; return nil }

type mockClientIdentity struct {
	mspID string
}

func (ci *mockClientIdentity) GetID() (string, error)    { return "user1", nil }
func (ci *mockClientIdentity) GetMSPID() (string, error) { return ci.mspID, nil }
func (ci *mockClientIdentity) GetAttributeValue(attrName string) (string, bool, error) {
	return "", false, nil
}
func (ci *mockClientIdentity) AssertAttributeValue(attrName, attrValue string) error { return nil }
func (ci *mockClientIdentity) GetX509Certificate() (*x509.Certificate, error)        { return nil, nil }

type mockTxContext struct {
	contractapi.TransactionContext
	stub     *mockStub
	identity *mockClientIdentity
}

func newMockContext(mspID string) *mockTxContext {
	stub := newMockStub()
	return &mockTxContext{
		stub:     stub,
		identity: &mockClientIdentity{mspID: mspID},
	}
}

func (ctx *mockTxContext) GetStub() shim.ChaincodeStubInterface {
	return ctx.stub
}

func (ctx *mockTxContext) GetClientIdentity() cid.ClientIdentity {
	return ctx.identity
}

func validCommercialTerms(invoiceID string) *CommercialTerms {
	return &CommercialTerms{
		SchemaVersion: SchemaVersion,
		InvoiceID:     invoiceID,
		AmountMinor:   100000,
		Currency:      "USD",
		DueDate:       "2026-12-31",
		PaymentTerms:  "Net 30",
		Salt:          "abcdef1234567890",
	}
}

func validPaymentDetails(invoiceID string) *PaymentDetails {
	return &PaymentDetails{
		InvoiceID:         invoiceID,
		AccountName:       "Supplier Inc",
		BankName:          "First Bank",
		AccountIdentifier: "123456789",
		RoutingCode:       "ABCD1234",
		PaymentReference:  "REF-001",
		Salt:              "1234567890abcdef",
	}
}

func validFinancingRequest(invoiceID string) *FinancingRequest {
	return &FinancingRequest{
		InvoiceID:            invoiceID,
		RequestedAmountMinor: 80000,
		RequestedTenor:       "30 days",
		Salt:                 "reqsalt123456789",
	}
}

func validDisbursementDetails(invoiceID string) *DisbursementDetails {
	return &DisbursementDetails{
		InvoiceID:         invoiceID,
		AccountName:       "Supplier Inc",
		BankName:          "Second Bank",
		AccountIdentifier: "987654321",
		RoutingCode:       "WXYZ9876",
		Salt:              "disbsalt12345678",
	}
}

func validFinancingAgreement(invoiceID string) *FinancingAgreement {
	return &FinancingAgreement{
		InvoiceID:           invoiceID,
		FinancedAmountMinor: 75000,
		DiscountBps:         250,
		MaturityTerms:       "Due in 30 days",
		Salt:                "agrsalt1234567890",
	}
}

func setTransient(ctx *mockTxContext, key string, value any) {
	data, _ := json.Marshal(value)
	ctx.stub.transient[key] = data
}

func parseInvoice(t *testing.T, jsonStr string) *Invoice {
	t.Helper()
	var inv Invoice
	err := json.Unmarshal([]byte(jsonStr), &inv)
	require.NoError(t, err)
	return &inv
}

func TestCreateInvoice_Valid(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	invoiceID := "INV-001"
	setTransient(ctx, "commercial_terms", validCommercialTerms(invoiceID))
	setTransient(ctx, "payment_details", validPaymentDetails(invoiceID))

	err := contract.CreateInvoice(ctx, invoiceID, BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)

	invJSON, err := contract.ReadPublicInvoice(ctx, invoiceID)
	require.NoError(t, err)
	inv := parseInvoice(t, invJSON)
	assert.Equal(t, StatusCreated, inv.Status)
	assert.Equal(t, SupplierMSP, inv.SupplierMSPID)
	assert.NotNil(t, ctx.stub.events["InvoiceCreated"])
}

func TestCreateInvoice_UnauthorizedMSP(t *testing.T) {
	ctx := newMockContext(BuyerMSP)
	contract := &InvoiceContract{}

	err := contract.CreateInvoice(ctx, "INV-001", BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestCreateInvoice_InvalidInvoiceID(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	setTransient(ctx, "commercial_terms", validCommercialTerms("AB"))
	setTransient(ctx, "payment_details", validPaymentDetails("AB"))

	err := contract.CreateInvoice(ctx, "AB", BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid invoice ID")
}

func TestCreateInvoice_InvalidBuyerMSP(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	setTransient(ctx, "commercial_terms", validCommercialTerms("INV-001"))
	setTransient(ctx, "payment_details", validPaymentDetails("INV-001"))

	err := contract.CreateInvoice(ctx, "INV-001", "InvalidMSP", "sha256:"+hex.EncodeToString(make([]byte, 32)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid buyer MSP")
}

func TestCreateInvoice_InvalidDocumentHash(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	setTransient(ctx, "commercial_terms", validCommercialTerms("INV-001"))
	setTransient(ctx, "payment_details", validPaymentDetails("INV-001"))

	err := contract.CreateInvoice(ctx, "INV-001", BuyerMSP, "invalid-hash")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid document hash")
}

func TestCreateInvoice_DuplicateID(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	invoiceID := "INV-DUP"
	setTransient(ctx, "commercial_terms", validCommercialTerms(invoiceID))
	setTransient(ctx, "payment_details", validPaymentDetails(invoiceID))

	err := contract.CreateInvoice(ctx, invoiceID, BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)

	setTransient(ctx, "commercial_terms", validCommercialTerms(invoiceID))
	setTransient(ctx, "payment_details", validPaymentDetails(invoiceID))

	err = contract.CreateInvoice(ctx, invoiceID, BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestCreateInvoice_MissingTransient(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	err := contract.CreateInvoice(ctx, "INV-001", BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing transient")
}

func TestCreateInvoice_MismatchedInvoiceID(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	setTransient(ctx, "commercial_terms", validCommercialTerms("OTHER-ID"))
	setTransient(ctx, "payment_details", validPaymentDetails("INV-001"))

	err := contract.CreateInvoice(ctx, "INV-001", BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

func TestCreateInvoice_InvalidAmount(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	ct := validCommercialTerms("INV-001")
	ct.AmountMinor = 0
	setTransient(ctx, "commercial_terms", ct)
	setTransient(ctx, "payment_details", validPaymentDetails("INV-001"))

	err := contract.CreateInvoice(ctx, "INV-001", BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestCreateInvoice_InvalidCurrency(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	ct := validCommercialTerms("INV-001")
	ct.Currency = "us"
	setTransient(ctx, "commercial_terms", ct)
	setTransient(ctx, "payment_details", validPaymentDetails("INV-001"))

	err := contract.CreateInvoice(ctx, "INV-001", BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "currency")
}

func TestCreateInvoice_ShortSalt(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	ct := validCommercialTerms("INV-001")
	ct.Salt = "short"
	setTransient(ctx, "commercial_terms", ct)
	setTransient(ctx, "payment_details", validPaymentDetails("INV-001"))

	err := contract.CreateInvoice(ctx, "INV-001", BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "salt")
}

func createTestInvoice(t *testing.T, ctx *mockTxContext, contract *InvoiceContract, invoiceID string) {
	setTransient(ctx, "commercial_terms", validCommercialTerms(invoiceID))
	setTransient(ctx, "payment_details", validPaymentDetails(invoiceID))
	err := contract.CreateInvoice(ctx, invoiceID, BuyerMSP, "sha256:"+hex.EncodeToString(make([]byte, 32)))
	require.NoError(t, err)
}

func TestApproveInvoice_Valid(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-APPROVE"

	createTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	err := contract.ApproveInvoice(ctx, invoiceID)
	require.NoError(t, err)

	invJSON, _ := contract.ReadPublicInvoice(ctx, invoiceID)
	inv := parseInvoice(t, invJSON)
	assert.Equal(t, StatusApproved, inv.Status)
	assert.NotEmpty(t, inv.ApprovedAt)
	assert.NotNil(t, ctx.stub.events["InvoiceApproved"])
}

func TestApproveInvoice_WrongMSP(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-APPROVE-WRONG"

	createTestInvoice(t, ctx, contract, invoiceID)

	err := contract.ApproveInvoice(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestApproveInvoice_WrongStatus(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-STATUS"

	createTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	_ = contract.ApproveInvoice(ctx, invoiceID)
	err := contract.ApproveInvoice(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestRejectInvoice_Valid(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-REJECT"

	createTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	err := contract.RejectInvoice(ctx, invoiceID)
	require.NoError(t, err)

	invJSON, _ := contract.ReadPublicInvoice(ctx, invoiceID)
	inv := parseInvoice(t, invJSON)
	assert.Equal(t, StatusRejected, inv.Status)
	assert.NotNil(t, ctx.stub.events["InvoiceRejected"])
}

func TestRejectInvoice_WrongMSP(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-REJECT-WRONG"

	createTestInvoice(t, ctx, contract, invoiceID)

	err := contract.RejectInvoice(ctx, invoiceID)
	assert.Error(t, err)
}

func TestRejectInvoice_WrongStatus(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-REJECT-STATUS"

	createTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	_ = contract.ApproveInvoice(ctx, invoiceID)
	err := contract.RejectInvoice(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func approveTestInvoice(t *testing.T, ctx *mockTxContext, contract *InvoiceContract, invoiceID string) {
	ctx.identity.mspID = BuyerMSP
	err := contract.ApproveInvoice(ctx, invoiceID)
	require.NoError(t, err)
	ctx.identity.mspID = SupplierMSP
}

func TestRequestFinancing_Valid(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FINANCE-REQ"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)

	setTransient(ctx, "invoice_disclosure", validCommercialTerms(invoiceID))
	setTransient(ctx, "financing_request", validFinancingRequest(invoiceID))
	setTransient(ctx, "disbursement_details", validDisbursementDetails(invoiceID))

	err := contract.RequestFinancing(ctx, invoiceID)
	require.NoError(t, err)

	invJSON, _ := contract.ReadPublicInvoice(ctx, invoiceID)
	inv := parseInvoice(t, invJSON)
	assert.Equal(t, StatusFinancingRequested, inv.Status)
	assert.NotNil(t, ctx.stub.events["FinancingRequested"])
}

func TestRequestFinancing_WrongMSP(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FIN-WRONGMSP"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	err := contract.RequestFinancing(ctx, invoiceID)
	assert.Error(t, err)
}

func TestRequestFinancing_WrongStatus(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FIN-WRONGST"

	createTestInvoice(t, ctx, contract, invoiceID)

	setTransient(ctx, "invoice_disclosure", validCommercialTerms(invoiceID))
	setTransient(ctx, "financing_request", validFinancingRequest(invoiceID))
	setTransient(ctx, "disbursement_details", validDisbursementDetails(invoiceID))

	err := contract.RequestFinancing(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestRequestFinancing_MismatchedDisclosure(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FIN-MISMATCH"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)

	disclosure := validCommercialTerms(invoiceID)
	disclosure.AmountMinor = 999999
	setTransient(ctx, "invoice_disclosure", disclosure)
	setTransient(ctx, "financing_request", validFinancingRequest(invoiceID))
	setTransient(ctx, "disbursement_details", validDisbursementDetails(invoiceID))

	err := contract.RequestFinancing(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestRequestFinancing_ZeroAmount(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FIN-ZERO"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)

	fr := validFinancingRequest(invoiceID)
	fr.RequestedAmountMinor = 0
	setTransient(ctx, "invoice_disclosure", validCommercialTerms(invoiceID))
	setTransient(ctx, "financing_request", fr)
	setTransient(ctx, "disbursement_details", validDisbursementDetails(invoiceID))

	err := contract.RequestFinancing(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestRequestFinancing_ExceedsInvoice(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FIN-EXCEED"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)

	fr := validFinancingRequest(invoiceID)
	fr.RequestedAmountMinor = 999999999
	setTransient(ctx, "invoice_disclosure", validCommercialTerms(invoiceID))
	setTransient(ctx, "financing_request", fr)
	setTransient(ctx, "disbursement_details", validDisbursementDetails(invoiceID))

	err := contract.RequestFinancing(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func requestFinancing(t *testing.T, ctx *mockTxContext, contract *InvoiceContract, invoiceID string) {
	setTransient(ctx, "invoice_disclosure", validCommercialTerms(invoiceID))
	setTransient(ctx, "financing_request", validFinancingRequest(invoiceID))
	setTransient(ctx, "disbursement_details", validDisbursementDetails(invoiceID))
	err := contract.RequestFinancing(ctx, invoiceID)
	require.NoError(t, err)
}

func TestFinanceInvoice_Valid(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FINANCED"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)
	requestFinancing(t, ctx, contract, invoiceID)

	ctx.identity.mspID = FinanceMSP
	setTransient(ctx, "financing_agreement", validFinancingAgreement(invoiceID))

	err := contract.FinanceInvoice(ctx, invoiceID)
	require.NoError(t, err)

	invJSON, _ := contract.ReadPublicInvoice(ctx, invoiceID)
	inv := parseInvoice(t, invJSON)
	assert.Equal(t, StatusFinanced, inv.Status)
	assert.True(t, inv.Financed)
	assert.Equal(t, FinanceMSP, inv.FinancierMSPID)
	assert.NotNil(t, ctx.stub.events["InvoiceFinanced"])
}

func TestFinanceInvoice_WrongMSP(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FIN-WRONGM"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)
	requestFinancing(t, ctx, contract, invoiceID)

	setTransient(ctx, "financing_agreement", validFinancingAgreement(invoiceID))
	err := contract.FinanceInvoice(ctx, invoiceID)
	assert.Error(t, err)
}

func TestFinanceInvoice_WrongStatus(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FIN-WRONGS"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = FinanceMSP
	setTransient(ctx, "financing_agreement", validFinancingAgreement(invoiceID))

	err := contract.FinanceInvoice(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestFinanceInvoice_ZeroAmount(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FIN-ZEROA"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)
	requestFinancing(t, ctx, contract, invoiceID)

	ctx.identity.mspID = FinanceMSP
	fa := validFinancingAgreement(invoiceID)
	fa.FinancedAmountMinor = 0
	setTransient(ctx, "financing_agreement", fa)

	err := contract.FinanceInvoice(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "positive")
}

func TestFinanceInvoice_ExceedsRequested(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FIN-EXCR"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)
	requestFinancing(t, ctx, contract, invoiceID)

	ctx.identity.mspID = FinanceMSP
	fa := validFinancingAgreement(invoiceID)
	fa.FinancedAmountMinor = 90000
	setTransient(ctx, "financing_agreement", fa)

	err := contract.FinanceInvoice(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds requested")
}

func financeInvoice(t *testing.T, ctx *mockTxContext, contract *InvoiceContract, invoiceID string) {
	ctx.identity.mspID = FinanceMSP
	setTransient(ctx, "financing_agreement", validFinancingAgreement(invoiceID))
	err := contract.FinanceInvoice(ctx, invoiceID)
	require.NoError(t, err)
}

func TestSettleInvoice_Valid(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-SETTLED"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)
	requestFinancing(t, ctx, contract, invoiceID)
	financeInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	err := contract.SettleInvoice(ctx, invoiceID)
	require.NoError(t, err)

	invJSON, _ := contract.ReadPublicInvoice(ctx, invoiceID)
	inv := parseInvoice(t, invJSON)
	assert.Equal(t, StatusSettled, inv.Status)
	assert.NotNil(t, ctx.stub.events["InvoiceSettled"])
}

func TestSettleInvoice_WrongMSP(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-SETTLEWM"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)
	requestFinancing(t, ctx, contract, invoiceID)
	financeInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = SupplierMSP
	err := contract.SettleInvoice(ctx, invoiceID)
	assert.Error(t, err)
}

func TestSettleInvoice_WrongStatus(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-SETTLEWS"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	err := contract.SettleInvoice(ctx, invoiceID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}

func TestTerminalStateCannotTransition(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	invoiceID := "INV-TERMINAL"
	createTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	_ = contract.RejectInvoice(ctx, invoiceID)

	err := contract.ApproveInvoice(ctx, invoiceID)
	assert.Error(t, err)
}

func TestReadPrivateData_Supplier(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-READ-SUP"

	createTestInvoice(t, ctx, contract, invoiceID)

	data, err := contract.ReadPrivateInvoiceData(ctx, invoiceID)
	require.NoError(t, err)
	assert.NotNil(t, data["commercialTerms"])
	assert.NotNil(t, data["paymentDetails"])
}

func TestReadPrivateData_Buyer(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-READ-BUY"

	createTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	data, err := contract.ReadPrivateInvoiceData(ctx, invoiceID)
	require.NoError(t, err)
	assert.NotNil(t, data["commercialTerms"])
}

func TestReadPrivateData_UnauthorizedFinance(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-READ-UNAUTH"

	createTestInvoice(t, ctx, contract, invoiceID)

	ctx.identity.mspID = FinanceMSP
	_, err := contract.ReadPrivateInvoiceData(ctx, invoiceID)
	assert.Error(t, err)
}

func TestReadFinancingTerms_Supplier(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FINT-SUP"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)
	requestFinancing(t, ctx, contract, invoiceID)

	data, err := contract.ReadFinancingTerms(ctx, invoiceID)
	require.NoError(t, err)
	assert.NotNil(t, data["disclosure"])
	assert.NotNil(t, data["financingRequest"])
}

func TestReadFinancingTerms_Finance(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FINT-FIN"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)
	requestFinancing(t, ctx, contract, invoiceID)

	ctx.identity.mspID = FinanceMSP
	data, err := contract.ReadFinancingTerms(ctx, invoiceID)
	require.NoError(t, err)
	assert.NotNil(t, data["financingRequest"])
}

func TestReadFinancingTerms_UnauthorizedBuyer(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-FINT-UNAUTH"

	createTestInvoice(t, ctx, contract, invoiceID)
	approveTestInvoice(t, ctx, contract, invoiceID)
	requestFinancing(t, ctx, contract, invoiceID)

	ctx.identity.mspID = BuyerMSP
	_, err := contract.ReadFinancingTerms(ctx, invoiceID)
	assert.Error(t, err)
}

func TestEventDoesNotContainPrivateData(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-EVENT"

	createTestInvoice(t, ctx, contract, invoiceID)

	eventData := ctx.stub.events["InvoiceCreated"]
	require.NotNil(t, eventData)

	var event InvoiceEvent
	err := json.Unmarshal(eventData, &event)
	require.NoError(t, err)

	eventStr := string(eventData)
	assert.NotContains(t, eventStr, "amount")
	assert.NotContains(t, eventStr, "currency")
	assert.NotContains(t, eventStr, "salt")
	assert.NotContains(t, eventStr, "bank")
}

func TestSBESetAfterCreate(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}
	invoiceID := "INV-SBE"

	createTestInvoice(t, ctx, contract, invoiceID)

	key := invoiceKey(invoiceID)
	sbe := ctx.stub.validationParam[key]
	assert.NotNil(t, sbe)
}

func TestValidation_InvoiceIDCanonicalization(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		valid    bool
	}{
		{"inv-001", "INV-001", true},
		{"  INV-002  ", "INV-002", true},
		{"ABC_123", "ABC_123", true},
		{"ab", "", false},
		{"inv 001", "", false},
	}

	for _, tt := range tests {
		result, err := canonicalizeInvoiceID(tt.input)
		if tt.valid {
			assert.NoError(t, err, "input: %s", tt.input)
			assert.Equal(t, tt.expected, result)
		} else {
			assert.Error(t, err, "input: %s", tt.input)
		}
	}
}

func TestValidation_DocumentHash(t *testing.T) {
	validHash := "sha256:" + hex.EncodeToString(make([]byte, 32))
	assert.NoError(t, validateDocumentHash(validHash))
	assert.Error(t, validateDocumentHash("invalid"))
	assert.Error(t, validateDocumentHash("sha256:short"))
	assert.Error(t, validateDocumentHash("md5:"+hex.EncodeToString(make([]byte, 32))))
}

func TestQueryPublicInvoicesByStatus(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	invoices, err := contract.QueryPublicInvoicesByStatus(ctx, StatusCreated)
	require.NoError(t, err)
	assert.Empty(t, invoices)
}

func TestQueryPublicInvoicesByStatus_InvalidStatus(t *testing.T) {
	ctx := newMockContext(SupplierMSP)
	contract := &InvoiceContract{}

	_, err := contract.QueryPublicInvoicesByStatus(ctx, "INVALID")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid status")
}
