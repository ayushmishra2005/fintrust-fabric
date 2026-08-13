package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHappyPath(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)

	invoiceID := generateInvoiceID("E2E-HAPPY")
	docHash := generateDocHash()
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)

	createInvoice(t, supplierContract, invoiceID, docHash, ct, pd)
	inv := readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "CREATED", inv.Status)
	assert.Equal(t, "SupplierMSP", inv.SupplierMSPID)
	assert.Equal(t, "BuyerMSP", inv.BuyerMSPID)
	assert.False(t, inv.Financed)
	assert.NotEmpty(t, inv.CreatedAt)
	assert.NotEmpty(t, inv.LastTxID)

	approveInvoice(t, buyerContract, invoiceID)
	inv = readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "APPROVED", inv.Status)
	assert.NotEmpty(t, inv.ApprovedAt)

	fr := makeFinancingRequest(invoiceID, suffix, 80000)
	dd := makeDisbursementDetails(invoiceID, suffix)
	requestFinancing(t, supplierContract, invoiceID, ct, fr, dd)
	inv = readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "FINANCING_REQUESTED", inv.Status)
	assert.NotEmpty(t, inv.FinancingRequestedAt)

	fa := makeFinancingAgreement(invoiceID, suffix, 75000)
	financeInvoice(t, financeContract, invoiceID, fa)
	inv = readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "FINANCED", inv.Status)
	assert.True(t, inv.Financed)
	assert.Equal(t, "FinanceMSP", inv.FinancierMSPID)
	assert.NotEmpty(t, inv.FinancedAt)

	settleInvoice(t, buyerContract, invoiceID)
	inv = readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "SETTLED", inv.Status)
	assert.NotEmpty(t, inv.SettledAt)
}

func TestDuplicateInvoice(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	contract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)

	invoiceID := generateInvoiceID("E2E-DUP")
	docHash := generateDocHash()
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)

	createInvoice(t, contract, invoiceID, docHash, ct, pd)

	err := submitTransientWithError(contract, "CreateInvoice",
		map[string][]byte{
			"commercial_terms": mustJSON(ct),
			"payment_details":  mustJSON(pd),
		},
		invoiceID, "BuyerMSP", docHash)
	require.Error(t, err, "duplicate invoice should fail")

	inv := readPublicInvoice(t, contract, invoiceID)
	assert.Equal(t, "CREATED", inv.Status)
}

func TestUnauthorizedCreate(t *testing.T) {
	skipIfNoNetwork(t)

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)

	invoiceID := generateInvoiceID("E2E-UNAUTH-CR")
	docHash := generateDocHash()
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)

	err := submitTransientWithError(buyerContract, "CreateInvoice",
		map[string][]byte{
			"commercial_terms": mustJSON(ct),
			"payment_details":  mustJSON(pd),
		},
		invoiceID, "BuyerMSP", docHash)
	require.Error(t, err, "BuyerMSP should not be able to create invoices")

	ct.InvoiceID = generateInvoiceID("E2E-UNAUTH-CR2")
	pd.InvoiceID = ct.InvoiceID
	err = submitTransientWithError(financeContract, "CreateInvoice",
		map[string][]byte{
			"commercial_terms": mustJSON(ct),
			"payment_details":  mustJSON(pd),
		},
		ct.InvoiceID, "BuyerMSP", docHash)
	require.Error(t, err, "FinanceMSP should not be able to create invoices")
}

func TestUnauthorizedApproval(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	invoiceID := generateInvoiceID("E2E-UNAUTH-AP")
	docHash := generateDocHash()
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	createInvoice(t, supplierContract, invoiceID, docHash, ct, pd)

	err := submitWithError(supplierContract, "ApproveInvoice", invoiceID)
	require.Error(t, err)

	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)
	err = submitWithError(financeContract, "ApproveInvoice", invoiceID)
	require.Error(t, err)

	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	approveInvoice(t, buyerContract, invoiceID)

	inv := readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "APPROVED", inv.Status)
}

func TestUnauthorizedRejection(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	invoiceID := generateInvoiceID("E2E-UNAUTH-REJ")
	docHash := generateDocHash()
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	createInvoice(t, supplierContract, invoiceID, docHash, ct, pd)

	err := submitWithError(supplierContract, "RejectInvoice", invoiceID)
	require.Error(t, err)

	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)
	err = submitWithError(financeContract, "RejectInvoice", invoiceID)
	require.Error(t, err)

	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	rejectInvoice(t, buyerContract, invoiceID)

	inv := readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "REJECTED", inv.Status)

	err = submitWithError(buyerContract, "ApproveInvoice", invoiceID)
	require.Error(t, err, "terminal REJECTED should not allow approval")
}

func TestInvalidStateTransitions(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)

	t.Run("RequestFinancing_while_CREATED", func(t *testing.T) {
		invoiceID := generateInvoiceID("E2E-ST-REQ")
		suffix := invoiceID[len(invoiceID)-8:]
		ct := makeCommercialTerms(invoiceID, suffix, 100000)
		pd := makePaymentDetails(invoiceID, suffix)
		createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)

		fr := makeFinancingRequest(invoiceID, suffix, 80000)
		dd := makeDisbursementDetails(invoiceID, suffix)
		err := submitTransientWithError(supplierContract, "RequestFinancing",
			map[string][]byte{
				"invoice_disclosure":   mustJSON(ct),
				"financing_request":    mustJSON(fr),
				"disbursement_details": mustJSON(dd),
			}, invoiceID)
		require.Error(t, err)
	})

	t.Run("FinanceInvoice_while_CREATED", func(t *testing.T) {
		invoiceID := generateInvoiceID("E2E-ST-FIN1")
		suffix := invoiceID[len(invoiceID)-8:]
		ct := makeCommercialTerms(invoiceID, suffix, 100000)
		pd := makePaymentDetails(invoiceID, suffix)
		createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)

		fa := makeFinancingAgreement(invoiceID, suffix, 75000)
		err := submitTransientWithError(financeContract, "FinanceInvoice",
			map[string][]byte{"financing_agreement": mustJSON(fa)}, invoiceID)
		require.Error(t, err)
	})

	t.Run("FinanceInvoice_while_APPROVED", func(t *testing.T) {
		invoiceID := generateInvoiceID("E2E-ST-FIN2")
		suffix := invoiceID[len(invoiceID)-8:]
		ct := makeCommercialTerms(invoiceID, suffix, 100000)
		pd := makePaymentDetails(invoiceID, suffix)
		createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
		approveInvoice(t, buyerContract, invoiceID)

		fa := makeFinancingAgreement(invoiceID, suffix, 75000)
		err := submitTransientWithError(financeContract, "FinanceInvoice",
			map[string][]byte{"financing_agreement": mustJSON(fa)}, invoiceID)
		require.Error(t, err)
	})

	t.Run("SettleInvoice_while_APPROVED", func(t *testing.T) {
		invoiceID := generateInvoiceID("E2E-ST-SET")
		suffix := invoiceID[len(invoiceID)-8:]
		ct := makeCommercialTerms(invoiceID, suffix, 100000)
		pd := makePaymentDetails(invoiceID, suffix)
		createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
		approveInvoice(t, buyerContract, invoiceID)

		err := submitWithError(buyerContract, "SettleInvoice", invoiceID)
		require.Error(t, err)
	})

	t.Run("ApproveInvoice_twice", func(t *testing.T) {
		invoiceID := generateInvoiceID("E2E-ST-APP2")
		suffix := invoiceID[len(invoiceID)-8:]
		ct := makeCommercialTerms(invoiceID, suffix, 100000)
		pd := makePaymentDetails(invoiceID, suffix)
		createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
		approveInvoice(t, buyerContract, invoiceID)

		err := submitWithError(buyerContract, "ApproveInvoice", invoiceID)
		require.Error(t, err)
	})

	t.Run("RejectInvoice_while_APPROVED", func(t *testing.T) {
		invoiceID := generateInvoiceID("E2E-ST-REJ2")
		suffix := invoiceID[len(invoiceID)-8:]
		ct := makeCommercialTerms(invoiceID, suffix, 100000)
		pd := makePaymentDetails(invoiceID, suffix)
		createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
		approveInvoice(t, buyerContract, invoiceID)

		err := submitWithError(buyerContract, "RejectInvoice", invoiceID)
		require.Error(t, err)
	})
}

func TestPrivateInvoiceDataAccess(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	invoiceID := generateInvoiceID("E2E-PRIV-INV")
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)

	result, err := supplierContract.EvaluateTransaction("ReadPrivateInvoiceData", invoiceID)
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	result, err = buyerContract.EvaluateTransaction("ReadPrivateInvoiceData", invoiceID)
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)
	_, err = financeContract.EvaluateTransaction("ReadPrivateInvoiceData", invoiceID)
	require.Error(t, err, "Finance should not access invoice private data")
}

func TestPrivateFinancingDataAccess(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	invoiceID := generateInvoiceID("E2E-PRIV-FIN")
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)
	fr := makeFinancingRequest(invoiceID, suffix, 80000)
	dd := makeDisbursementDetails(invoiceID, suffix)
	fa := makeFinancingAgreement(invoiceID, suffix, 75000)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)

	createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
	approveInvoice(t, buyerContract, invoiceID)
	requestFinancing(t, supplierContract, invoiceID, ct, fr, dd)
	financeInvoice(t, financeContract, invoiceID, fa)

	result, err := supplierContract.EvaluateTransaction("ReadFinancingTerms", invoiceID)
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	result, err = financeContract.EvaluateTransaction("ReadFinancingTerms", invoiceID)
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	_, err = buyerContract.EvaluateTransaction("ReadFinancingTerms", invoiceID)
	require.Error(t, err, "Buyer should not access financing terms")
}

func TestPublicStateLeakage(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	invoiceID := generateInvoiceID("E2E-LEAK-PUB")
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)

	createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
	approveInvoice(t, buyerContract, invoiceID)

	result, err := supplierContract.EvaluateTransaction("ReadPublicInvoice", invoiceID)
	require.NoError(t, err)

	publicStr := string(result)
	assert.NotContains(t, publicStr, ct.PaymentTerms, "payment terms leaked")
	assert.NotContains(t, publicStr, pd.AccountName, "account name leaked")
	assert.NotContains(t, publicStr, pd.BankName, "bank name leaked")
	assert.NotContains(t, publicStr, pd.AccountIdentifier, "account identifier leaked")
	assert.NotContains(t, publicStr, ct.Salt, "commercial terms salt leaked")
	assert.NotContains(t, publicStr, pd.Salt, "payment details salt leaked")
	assert.NotContains(t, publicStr, "100000", "amount leaked in public state")
}

func TestSequentialDoubleFinancing(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	invoiceID := generateInvoiceID("E2E-DBL-FIN")
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)
	fr := makeFinancingRequest(invoiceID, suffix, 80000)
	dd := makeDisbursementDetails(invoiceID, suffix)
	fa := makeFinancingAgreement(invoiceID, suffix, 75000)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)

	createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
	approveInvoice(t, buyerContract, invoiceID)
	requestFinancing(t, supplierContract, invoiceID, ct, fr, dd)
	financeInvoice(t, financeContract, invoiceID, fa)

	fa2 := makeFinancingAgreement(invoiceID, suffix+"2", 70000)
	err := submitTransientWithError(financeContract, "FinanceInvoice",
		map[string][]byte{"financing_agreement": mustJSON(fa2)}, invoiceID)
	require.Error(t, err, "second financing should fail")

	inv := readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "FINANCED", inv.Status)
	assert.True(t, inv.Financed)
	assert.Equal(t, "FinanceMSP", inv.FinancierMSPID)
}

// TestConcurrentFinanceSingleWinner verifies that concurrent FinanceInvoice calls
// result in exactly one successful financing. The losing transaction fails with
// ENDORSEMENT_POLICY_FAILURE because FinanceInvoice rotates the public invoice
// SBE from Supplier+Finance to Buyer+Finance, invalidating the loser's endorsements.
func TestConcurrentFinanceSingleWinner(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	invoiceID := generateInvoiceID("E2E-CONC-FIN")
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)
	fr := makeFinancingRequest(invoiceID, suffix, 80000)
	dd := makeDisbursementDetails(invoiceID, suffix)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)

	createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
	approveInvoice(t, buyerContract, invoiceID)
	requestFinancing(t, supplierContract, invoiceID, ct, fr, dd)

	inv := readPublicInvoice(t, supplierContract, invoiceID)
	require.Equal(t, "FINANCING_REQUESTED", inv.Status)

	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)

	fa1 := makeFinancingAgreement(invoiceID, suffix+"A", 75000)
	fa2 := makeFinancingAgreement(invoiceID, suffix+"B", 74000)

	ctx := context.Background()

	proposalA, err := financeContract.NewProposal("FinanceInvoice",
		client.WithArguments(invoiceID),
		client.WithTransient(map[string][]byte{"financing_agreement": mustJSON(fa1)}),
	)
	require.NoError(t, err, "failed to create proposal A")

	proposalB, err := financeContract.NewProposal("FinanceInvoice",
		client.WithArguments(invoiceID),
		client.WithTransient(map[string][]byte{"financing_agreement": mustJSON(fa2)}),
	)
	require.NoError(t, err, "failed to create proposal B")

	txA, err := proposalA.Endorse()
	require.NoError(t, err, "endorsement A failed")
	t.Logf("Proposal A endorsed: txID=%s", txA.TransactionID())

	txB, err := proposalB.Endorse()
	require.NoError(t, err, "endorsement B failed")
	t.Logf("Proposal B endorsed: txID=%s", txB.TransactionID())

	commitA, err := txA.Submit()
	require.NoError(t, err, "submit A failed")

	commitB, err := txB.Submit()
	require.NoError(t, err, "submit B failed")

	ctxTimeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	statusA, errA := commitA.StatusWithContext(ctxTimeout)
	statusB, errB := commitB.StatusWithContext(ctxTimeout)

	t.Logf("Tx A status: code=%v, err=%v", statusA, errA)
	t.Logf("Tx B status: code=%v, err=%v", statusB, errB)

	aValid := errA == nil && statusA.Code == peer.TxValidationCode_VALID
	bValid := errB == nil && statusB.Code == peer.TxValidationCode_VALID

	// Expected: loser fails with ENDORSEMENT_POLICY_FAILURE due to SBE rotation
	aFailed := statusA.Code == peer.TxValidationCode_ENDORSEMENT_POLICY_FAILURE
	bFailed := statusB.Code == peer.TxValidationCode_ENDORSEMENT_POLICY_FAILURE

	require.True(t, (aValid && bFailed) || (bValid && aFailed),
		"expected one VALID and one ENDORSEMENT_POLICY_FAILURE: A=%v B=%v",
		statusA.Code, statusB.Code)

	if aValid {
		t.Log("Tx A: VALID")
		t.Log("Tx B: ENDORSEMENT_POLICY_FAILURE (SBE rotated by first tx)")
	} else {
		t.Log("Tx A: ENDORSEMENT_POLICY_FAILURE (SBE rotated by first tx)")
		t.Log("Tx B: VALID")
	}

	inv = readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "FINANCED", inv.Status)
	assert.True(t, inv.Financed)
	assert.Equal(t, "FinanceMSP", inv.FinancierMSPID)
}

// TestConcurrentRequestFinancingMVCC demonstrates Fabric MVCC conflict detection.
// Two RequestFinancing transactions endorsed against the same invoice version
// both attempt to write the same key. The first committed wins; the second
// receives MVCC_READ_CONFLICT because its read-set version is stale.
func TestConcurrentRequestFinancingMVCC(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	invoiceID := generateInvoiceID("E2E-MVCC")
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)

	createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
	approveInvoice(t, buyerContract, invoiceID)

	inv := readPublicInvoice(t, supplierContract, invoiceID)
	require.Equal(t, "APPROVED", inv.Status)

	frA := makeFinancingRequest(invoiceID, suffix+"A", 80000)
	ddA := makeDisbursementDetails(invoiceID, suffix+"A")

	frB := makeFinancingRequest(invoiceID, suffix+"B", 75000)
	ddB := makeDisbursementDetails(invoiceID, suffix+"B")

	ctx := context.Background()

	proposalA, err := supplierContract.NewProposal("RequestFinancing",
		client.WithArguments(invoiceID),
		client.WithTransient(map[string][]byte{
			"invoice_disclosure":   mustJSON(ct),
			"financing_request":    mustJSON(frA),
			"disbursement_details": mustJSON(ddA),
		}),
	)
	require.NoError(t, err, "failed to create proposal A")

	proposalB, err := supplierContract.NewProposal("RequestFinancing",
		client.WithArguments(invoiceID),
		client.WithTransient(map[string][]byte{
			"invoice_disclosure":   mustJSON(ct),
			"financing_request":    mustJSON(frB),
			"disbursement_details": mustJSON(ddB),
		}),
	)
	require.NoError(t, err, "failed to create proposal B")

	txA, err := proposalA.Endorse()
	require.NoError(t, err, "endorsement A failed")
	t.Logf("Proposal A endorsed: txID=%s", txA.TransactionID())

	txB, err := proposalB.Endorse()
	require.NoError(t, err, "endorsement B failed")
	t.Logf("Proposal B endorsed: txID=%s", txB.TransactionID())

	commitA, err := txA.Submit()
	require.NoError(t, err, "submit A failed")

	commitB, err := txB.Submit()
	require.NoError(t, err, "submit B failed")

	ctxTimeout, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	statusA, errA := commitA.StatusWithContext(ctxTimeout)
	statusB, errB := commitB.StatusWithContext(ctxTimeout)

	t.Logf("Tx A status: code=%v, err=%v", statusA, errA)
	t.Logf("Tx B status: code=%v, err=%v", statusB, errB)

	aValid := errA == nil && statusA.Code == peer.TxValidationCode_VALID
	bValid := errB == nil && statusB.Code == peer.TxValidationCode_VALID

	aMVCC := statusA.Code == peer.TxValidationCode_MVCC_READ_CONFLICT
	bMVCC := statusB.Code == peer.TxValidationCode_MVCC_READ_CONFLICT

	require.True(t, (aValid && bMVCC) || (bValid && aMVCC),
		"expected exactly one VALID and one MVCC_READ_CONFLICT: A=%v B=%v",
		statusA.Code, statusB.Code)

	if aValid {
		t.Log("Tx A: VALID")
		t.Log("Tx B: MVCC_READ_CONFLICT")
	} else {
		t.Log("Tx A: MVCC_READ_CONFLICT")
		t.Log("Tx B: VALID")
	}

	inv = readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "FINANCING_REQUESTED", inv.Status)
	assert.False(t, inv.Financed)
}

func TestEventPayloadLeakage(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	invoiceID := generateInvoiceID("E2E-EVT-LEAK")
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)
	fr := makeFinancingRequest(invoiceID, suffix, 80000)
	dd := makeDisbursementDetails(invoiceID, suffix)
	fa := makeFinancingAgreement(invoiceID, suffix, 75000)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)

	supplierNetwork := supplierGW.GetNetwork(channelName)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := supplierNetwork.ChaincodeEvents(ctx, chaincodeName)
	require.NoError(t, err)

	createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)

	var createdEvent *client.ChaincodeEvent
	timeout := time.After(10 * time.Second)
	for createdEvent == nil {
		select {
		case e := <-events:
			if e.EventName == "InvoiceCreated" {
				var payload map[string]any
				json.Unmarshal(e.Payload, &payload)
				if id, ok := payload["invoiceId"].(string); ok && id == invoiceID {
					createdEvent = e
				}
			}
		case <-timeout:
			t.Fatal("timeout waiting for InvoiceCreated event")
		}
	}

	payloadStr := string(createdEvent.Payload)
	assert.NotContains(t, payloadStr, ct.PaymentTerms)
	assert.NotContains(t, payloadStr, pd.AccountName)
	assert.NotContains(t, payloadStr, ct.Salt)
	assert.NotContains(t, payloadStr, "100000")

	approveInvoice(t, buyerContract, invoiceID)
	requestFinancing(t, supplierContract, invoiceID, ct, fr, dd)
	financeInvoice(t, financeContract, invoiceID, fa)
	settleInvoice(t, buyerContract, invoiceID)

	cancel()
}

func TestBlockPrivateDataLeakage(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	invoiceID := generateInvoiceID("E2E-BLK-LEAK")
	suffix := invoiceID[len(invoiceID)-8:]

	secretPaymentTerms := "SECRET-BLOCK-TEST-TERMS-" + suffix
	secretAccountName := "SECRET-BLOCK-TEST-ACCOUNT-" + suffix
	secretSalt := generateSalt()

	ct := CommercialTerms{
		SchemaVersion: "1.0",
		InvoiceID:     invoiceID,
		AmountMinor:   100000,
		Currency:      "USD",
		DueDate:       "2026-12-31",
		PaymentTerms:  secretPaymentTerms,
		Salt:          secretSalt,
	}
	pd := PaymentDetails{
		InvoiceID:         invoiceID,
		AccountName:       secretAccountName,
		BankName:          "Secret Bank",
		AccountIdentifier: "SECRET-IBAN-12345",
		RoutingCode:       "ABCD1234",
		PaymentReference:  "REF-001",
		Salt:              generateSalt(),
	}

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	supplierNetwork := supplierGW.GetNetwork(channelName)

	_, err := supplierContract.Submit("CreateInvoice",
		client.WithArguments(invoiceID, "BuyerMSP", generateDocHash()),
		client.WithTransient(map[string][]byte{
			"commercial_terms": mustJSON(ct),
			"payment_details":  mustJSON(pd),
		}),
	)
	require.NoError(t, err)

	approveInvoice(t, buyerContract, invoiceID)

	inv := readPublicInvoice(t, supplierContract, invoiceID)
	require.NotEmpty(t, inv.LastTxID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	events, err := supplierNetwork.ChaincodeEvents(ctx, chaincodeName, client.WithStartBlock(1))
	require.NoError(t, err)

	var targetBlockNum uint64
	timeout := time.After(15 * time.Second)
	for targetBlockNum == 0 {
		select {
		case e := <-events:
			if strings.Contains(string(e.Payload), invoiceID) {
				targetBlockNum = e.BlockNumber
			}
		case <-timeout:
			t.Fatal("timeout finding block with our transaction")
		}
	}
	cancel()

	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel2()

	blockIter, err := supplierNetwork.BlockEvents(ctx2, client.WithStartBlock(targetBlockNum))
	require.NoError(t, err)

	var blockData []byte
	select {
	case block := <-blockIter:
		blockData, _ = json.Marshal(block)
	case <-time.After(10 * time.Second):
		t.Fatal("timeout getting block")
	}
	cancel2()

	blockStr := string(blockData)
	assert.NotContains(t, blockStr, secretPaymentTerms, "payment terms found in public block")
	assert.NotContains(t, blockStr, secretAccountName, "account name found in public block")
	assert.NotContains(t, blockStr, secretSalt, "salt found in public block")
}

func TestSettleInvoiceTwice(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	buyerConn, buyerGW := newGateway(t, buyerConfig())
	defer buyerConn.Close()
	defer buyerGW.Close()

	financeConn, financeGW := newGateway(t, financeConfig())
	defer financeConn.Close()
	defer financeGW.Close()

	invoiceID := generateInvoiceID("E2E-SETTLE2")
	suffix := invoiceID[len(invoiceID)-8:]
	ct := makeCommercialTerms(invoiceID, suffix, 100000)
	pd := makePaymentDetails(invoiceID, suffix)
	fr := makeFinancingRequest(invoiceID, suffix, 80000)
	dd := makeDisbursementDetails(invoiceID, suffix)
	fa := makeFinancingAgreement(invoiceID, suffix, 75000)

	supplierContract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)
	buyerContract := buyerGW.GetNetwork(channelName).GetContract(chaincodeName)
	financeContract := financeGW.GetNetwork(channelName).GetContract(chaincodeName)

	createInvoice(t, supplierContract, invoiceID, generateDocHash(), ct, pd)
	approveInvoice(t, buyerContract, invoiceID)
	requestFinancing(t, supplierContract, invoiceID, ct, fr, dd)
	financeInvoice(t, financeContract, invoiceID, fa)
	settleInvoice(t, buyerContract, invoiceID)

	err := submitWithError(buyerContract, "SettleInvoice", invoiceID)
	require.Error(t, err, "second settle should fail")

	inv := readPublicInvoice(t, supplierContract, invoiceID)
	assert.Equal(t, "SETTLED", inv.Status)
}

func TestChaincodeReady(t *testing.T) {
	skipIfNoNetwork(t)

	supplierConn, supplierGW := newGateway(t, supplierConfig())
	defer supplierConn.Close()
	defer supplierGW.Close()

	contract := supplierGW.GetNetwork(channelName).GetContract(chaincodeName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := contract.EvaluateWithContext(ctx, "ReadPublicInvoice", client.WithArguments("nonexistent-probe"))
	if err == nil {
		t.Fatal("expected error for nonexistent invoice")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("unexpected error (chaincode may not be ready): %v", err)
	}
}
