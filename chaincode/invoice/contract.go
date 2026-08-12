package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

type InvoiceContract struct {
	contractapi.Contract
}

func (c *InvoiceContract) CreateInvoice(ctx contractapi.TransactionContextInterface, invoiceID, buyerMspID, documentHash string) error {
	if err := requireSupplier(ctx); err != nil {
		return err
	}
	canonicalID, err := canonicalizeInvoiceID(invoiceID)
	if err != nil {
		return err
	}
	if err := validateBuyerMSP(buyerMspID); err != nil {
		return err
	}
	if err := validateDocumentHash(documentHash); err != nil {
		return err
	}

	key := invoiceKey(canonicalID)
	existing, err := ctx.GetStub().GetState(key)
	if err != nil {
		return fmt.Errorf("failed to read state: %w", err)
	}
	if existing != nil {
		return fmt.Errorf("invoice already exists")
	}

	ct, err := parseTransient[CommercialTerms](ctx, "commercial_terms")
	if err != nil {
		return err
	}
	ct.SchemaVersion = SchemaVersion
	if err := validateCommercialTerms(ct, canonicalID); err != nil {
		return err
	}

	pd, err := parseTransient[PaymentDetails](ctx, "payment_details")
	if err != nil {
		return err
	}
	if err := validatePaymentDetails(pd, canonicalID); err != nil {
		return err
	}

	if err := putPrivateData(ctx, CollectionInvoiceParties, commercialTermsKey(canonicalID), ct); err != nil {
		return fmt.Errorf("failed to store commercial terms: %w", err)
	}
	if err := putPrivateData(ctx, CollectionInvoiceParties, paymentDetailsKey(canonicalID), pd); err != nil {
		return fmt.Errorf("failed to store payment details: %w", err)
	}

	supplierMSP, _ := getMSPID(ctx)
	ts, txID := getTxInfo(ctx)

	invoice := &Invoice{
		DocType:       DocTypeInvoice,
		SchemaVersion: SchemaVersion,
		InvoiceID:     canonicalID,
		SupplierMSPID: supplierMSP,
		BuyerMSPID:    buyerMspID,
		DocumentHash:  documentHash,
		Status:        StatusCreated,
		Financed:      false,
		CreatedAt:     ts,
		UpdatedAt:     ts,
		LastTxID:      txID,
	}

	if err := putInvoice(ctx, key, invoice); err != nil {
		return err
	}
	if err := setSBE(ctx, key, SupplierMSP, BuyerMSP); err != nil {
		return fmt.Errorf("failed to set SBE: %w", err)
	}

	return emitEvent(ctx, "InvoiceCreated", invoice, ts, txID)
}

func (c *InvoiceContract) ApproveInvoice(ctx contractapi.TransactionContextInterface, invoiceID string) error {
	if err := requireBuyer(ctx); err != nil {
		return err
	}
	canonicalID, err := canonicalizeInvoiceID(invoiceID)
	if err != nil {
		return err
	}

	key := invoiceKey(canonicalID)
	invoice, err := getInvoice(ctx, key)
	if err != nil {
		return err
	}
	if invoice == nil {
		return fmt.Errorf("invoice not found")
	}
	if err := requireInvoiceBuyer(ctx, invoice); err != nil {
		return err
	}
	if invoice.Status != StatusCreated {
		return fmt.Errorf("invalid status for approval: %s", invoice.Status)
	}

	exists, err := privateDataExists(ctx, CollectionInvoiceParties, commercialTermsKey(canonicalID))
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("commercial terms not found")
	}

	ts, txID := getTxInfo(ctx)
	invoice.Status = StatusApproved
	invoice.ApprovedAt = ts
	invoice.UpdatedAt = ts
	invoice.LastTxID = txID

	if err := putInvoice(ctx, key, invoice); err != nil {
		return err
	}
	if err := setSBE(ctx, key, SupplierMSP, FinanceMSP); err != nil {
		return fmt.Errorf("failed to rotate SBE: %w", err)
	}

	return emitEvent(ctx, "InvoiceApproved", invoice, ts, txID)
}

func (c *InvoiceContract) RejectInvoice(ctx contractapi.TransactionContextInterface, invoiceID string) error {
	if err := requireBuyer(ctx); err != nil {
		return err
	}
	canonicalID, err := canonicalizeInvoiceID(invoiceID)
	if err != nil {
		return err
	}

	key := invoiceKey(canonicalID)
	invoice, err := getInvoice(ctx, key)
	if err != nil {
		return err
	}
	if invoice == nil {
		return fmt.Errorf("invoice not found")
	}
	if err := requireInvoiceBuyer(ctx, invoice); err != nil {
		return err
	}
	if invoice.Status != StatusCreated {
		return fmt.Errorf("invalid status for rejection: %s", invoice.Status)
	}

	ts, txID := getTxInfo(ctx)
	invoice.Status = StatusRejected
	invoice.RejectedAt = ts
	invoice.UpdatedAt = ts
	invoice.LastTxID = txID

	if err := putInvoice(ctx, key, invoice); err != nil {
		return err
	}

	return emitEvent(ctx, "InvoiceRejected", invoice, ts, txID)
}

func (c *InvoiceContract) RequestFinancing(ctx contractapi.TransactionContextInterface, invoiceID string) error {
	if err := requireSupplier(ctx); err != nil {
		return err
	}
	canonicalID, err := canonicalizeInvoiceID(invoiceID)
	if err != nil {
		return err
	}

	key := invoiceKey(canonicalID)
	invoice, err := getInvoice(ctx, key)
	if err != nil {
		return err
	}
	if invoice == nil {
		return fmt.Errorf("invoice not found")
	}
	if err := requireInvoiceSupplier(ctx, invoice); err != nil {
		return err
	}
	if invoice.Status != StatusApproved {
		return fmt.Errorf("invalid status for financing request: %s", invoice.Status)
	}
	if invoice.Financed {
		return fmt.Errorf("invoice already financed")
	}

	exists, err := privateDataExists(ctx, CollectionSupplierFinance, financingAgreementKey(canonicalID))
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("financing agreement already exists")
	}
	exists, err = privateDataExists(ctx, CollectionSupplierFinance, financingRequestKey(canonicalID))
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("financing request already exists")
	}

	originalHash, err := getPrivateDataHash(ctx, CollectionInvoiceParties, commercialTermsKey(canonicalID))
	if err != nil {
		return fmt.Errorf("failed to get commercial terms hash: %w", err)
	}
	if originalHash == nil {
		return fmt.Errorf("commercial terms not found")
	}

	disclosure, err := parseTransient[CommercialTerms](ctx, "invoice_disclosure")
	if err != nil {
		return err
	}
	disclosure.SchemaVersion = SchemaVersion
	if err := validateCommercialTerms(disclosure, canonicalID); err != nil {
		return err
	}

	disclosureBytes, err := canonicalJSON(disclosure)
	if err != nil {
		return err
	}
	disclosureHash := sha256.Sum256(disclosureBytes)
	if hex.EncodeToString(disclosureHash[:]) != hex.EncodeToString(originalHash) {
		return fmt.Errorf("disclosure does not match approved commercial terms")
	}

	fr, err := parseTransient[FinancingRequest](ctx, "financing_request")
	if err != nil {
		return err
	}
	if err := validateFinancingRequest(fr, canonicalID, disclosure.AmountMinor); err != nil {
		return err
	}

	dd, err := parseTransient[DisbursementDetails](ctx, "disbursement_details")
	if err != nil {
		return err
	}
	if err := validateDisbursementDetails(dd, canonicalID); err != nil {
		return err
	}

	if err := putPrivateData(ctx, CollectionSupplierFinance, disclosureKey(canonicalID), disclosure); err != nil {
		return fmt.Errorf("failed to store disclosure: %w", err)
	}
	if err := putPrivateData(ctx, CollectionSupplierFinance, financingRequestKey(canonicalID), fr); err != nil {
		return fmt.Errorf("failed to store financing request: %w", err)
	}
	if err := putPrivateData(ctx, CollectionSupplierFinance, disbursementDetailsKey(canonicalID), dd); err != nil {
		return fmt.Errorf("failed to store disbursement details: %w", err)
	}

	ts, txID := getTxInfo(ctx)
	invoice.Status = StatusFinancingRequested
	invoice.FinancingRequestedAt = ts
	invoice.UpdatedAt = ts
	invoice.LastTxID = txID

	if err := putInvoice(ctx, key, invoice); err != nil {
		return err
	}

	return emitEvent(ctx, "FinancingRequested", invoice, ts, txID)
}

func (c *InvoiceContract) FinanceInvoice(ctx contractapi.TransactionContextInterface, invoiceID string) error {
	if err := requireFinance(ctx); err != nil {
		return err
	}
	canonicalID, err := canonicalizeInvoiceID(invoiceID)
	if err != nil {
		return err
	}

	key := invoiceKey(canonicalID)
	invoice, err := getInvoice(ctx, key)
	if err != nil {
		return err
	}
	if invoice == nil {
		return fmt.Errorf("invoice not found")
	}
	if invoice.Status != StatusFinancingRequested {
		return fmt.Errorf("invalid status for financing: %s", invoice.Status)
	}
	if invoice.Financed {
		return fmt.Errorf("invoice already financed")
	}

	exists, err := privateDataExists(ctx, CollectionSupplierFinance, financingAgreementKey(canonicalID))
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("financing agreement already exists")
	}

	disclosure, err := getPrivateData[CommercialTerms](ctx, CollectionSupplierFinance, disclosureKey(canonicalID))
	if err != nil {
		return err
	}
	if disclosure == nil {
		return fmt.Errorf("disclosure not found")
	}

	fr, err := getPrivateData[FinancingRequest](ctx, CollectionSupplierFinance, financingRequestKey(canonicalID))
	if err != nil {
		return err
	}
	if fr == nil {
		return fmt.Errorf("financing request not found")
	}

	fa, err := parseTransient[FinancingAgreement](ctx, "financing_agreement")
	if err != nil {
		return err
	}
	if err := validateFinancingAgreement(fa, canonicalID, disclosure.AmountMinor, fr.RequestedAmountMinor); err != nil {
		return err
	}

	if err := putPrivateData(ctx, CollectionSupplierFinance, financingAgreementKey(canonicalID), fa); err != nil {
		return fmt.Errorf("failed to store financing agreement: %w", err)
	}

	ts, txID := getTxInfo(ctx)
	financierMSP, _ := getMSPID(ctx)
	invoice.Status = StatusFinanced
	invoice.Financed = true
	invoice.FinancierMSPID = financierMSP
	invoice.FinancedAt = ts
	invoice.UpdatedAt = ts
	invoice.LastTxID = txID

	if err := putInvoice(ctx, key, invoice); err != nil {
		return err
	}
	if err := setSBE(ctx, key, BuyerMSP, FinanceMSP); err != nil {
		return fmt.Errorf("failed to rotate SBE: %w", err)
	}

	return emitEvent(ctx, "InvoiceFinanced", invoice, ts, txID)
}

func (c *InvoiceContract) SettleInvoice(ctx contractapi.TransactionContextInterface, invoiceID string) error {
	if err := requireBuyer(ctx); err != nil {
		return err
	}
	canonicalID, err := canonicalizeInvoiceID(invoiceID)
	if err != nil {
		return err
	}

	key := invoiceKey(canonicalID)
	invoice, err := getInvoice(ctx, key)
	if err != nil {
		return err
	}
	if invoice == nil {
		return fmt.Errorf("invoice not found")
	}
	if err := requireInvoiceBuyer(ctx, invoice); err != nil {
		return err
	}
	if invoice.Status != StatusFinanced {
		return fmt.Errorf("invalid status for settlement: %s", invoice.Status)
	}
	if !invoice.Financed {
		return fmt.Errorf("invoice not financed")
	}

	ts, txID := getTxInfo(ctx)
	invoice.Status = StatusSettled
	invoice.SettledAt = ts
	invoice.UpdatedAt = ts
	invoice.LastTxID = txID

	if err := putInvoice(ctx, key, invoice); err != nil {
		return err
	}

	return emitEvent(ctx, "InvoiceSettled", invoice, ts, txID)
}

func (c *InvoiceContract) ReadPublicInvoice(ctx contractapi.TransactionContextInterface, invoiceID string) (string, error) {
	if _, err := requireMSP(ctx, SupplierMSP, BuyerMSP, FinanceMSP); err != nil {
		return "", err
	}
	canonicalID, err := canonicalizeInvoiceID(invoiceID)
	if err != nil {
		return "", err
	}
	data, err := ctx.GetStub().GetState(invoiceKey(canonicalID))
	if err != nil {
		return "", fmt.Errorf("failed to read state: %w", err)
	}
	if data == nil {
		return "", fmt.Errorf("invoice not found")
	}
	return string(data), nil
}

func (c *InvoiceContract) ReadPrivateInvoiceData(ctx contractapi.TransactionContextInterface, invoiceID string) (map[string]any, error) {
	mspID, err := requireMSP(ctx, SupplierMSP, BuyerMSP)
	if err != nil {
		return nil, err
	}
	canonicalID, err := canonicalizeInvoiceID(invoiceID)
	if err != nil {
		return nil, err
	}

	invoice, err := getInvoice(ctx, invoiceKey(canonicalID))
	if err != nil {
		return nil, err
	}
	if invoice == nil {
		return nil, fmt.Errorf("invoice not found")
	}
	if !isParty(mspID, invoice) {
		return nil, fmt.Errorf("caller is not a party to this invoice")
	}

	ct, err := getPrivateData[CommercialTerms](ctx, CollectionInvoiceParties, commercialTermsKey(canonicalID))
	if err != nil {
		return nil, err
	}
	pd, err := getPrivateData[PaymentDetails](ctx, CollectionInvoiceParties, paymentDetailsKey(canonicalID))
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"commercialTerms": ct,
		"paymentDetails":  pd,
	}, nil
}

func (c *InvoiceContract) ReadFinancingTerms(ctx contractapi.TransactionContextInterface, invoiceID string) (map[string]any, error) {
	mspID, err := requireMSP(ctx, SupplierMSP, FinanceMSP)
	if err != nil {
		return nil, err
	}
	canonicalID, err := canonicalizeInvoiceID(invoiceID)
	if err != nil {
		return nil, err
	}

	invoice, err := getInvoice(ctx, invoiceKey(canonicalID))
	if err != nil {
		return nil, err
	}
	if invoice == nil {
		return nil, fmt.Errorf("invoice not found")
	}
	if !isFinancingParty(mspID, invoice) {
		return nil, fmt.Errorf("caller is not a financing party")
	}

	result := make(map[string]any)

	disclosure, err := getPrivateData[CommercialTerms](ctx, CollectionSupplierFinance, disclosureKey(canonicalID))
	if err != nil {
		return nil, err
	}
	if disclosure != nil {
		result["disclosure"] = disclosure
	}

	fr, err := getPrivateData[FinancingRequest](ctx, CollectionSupplierFinance, financingRequestKey(canonicalID))
	if err != nil {
		return nil, err
	}
	if fr != nil {
		result["financingRequest"] = fr
	}

	fa, err := getPrivateData[FinancingAgreement](ctx, CollectionSupplierFinance, financingAgreementKey(canonicalID))
	if err != nil {
		return nil, err
	}
	if fa != nil {
		result["financingAgreement"] = fa
	}

	return result, nil
}

func (c *InvoiceContract) QueryPublicInvoicesByStatus(ctx contractapi.TransactionContextInterface, status string) ([]*Invoice, error) {
	if _, err := requireMSP(ctx, SupplierMSP, BuyerMSP, FinanceMSP); err != nil {
		return nil, err
	}
	if !isValidStatus(status) {
		return nil, fmt.Errorf("invalid status")
	}

	query := fmt.Sprintf(`{"selector":{"docType":"%s","status":"%s"}}`, DocTypeInvoice, status)
	iter, err := ctx.GetStub().GetQueryResult(query)
	if err != nil {
		return nil, err
	}
	defer iter.Close()

	var invoices []*Invoice
	for iter.HasNext() {
		result, err := iter.Next()
		if err != nil {
			return nil, err
		}
		var inv Invoice
		if err := json.Unmarshal(result.Value, &inv); err != nil {
			return nil, err
		}
		invoices = append(invoices, &inv)
	}
	return invoices, nil
}

func getTxInfo(ctx contractapi.TransactionContextInterface) (string, string) {
	txTS, _ := ctx.GetStub().GetTxTimestamp()
	ts := time.Unix(txTS.Seconds, int64(txTS.Nanos)).UTC().Format(time.RFC3339Nano)
	txID := ctx.GetStub().GetTxID()
	return ts, txID
}

func getInvoice(ctx contractapi.TransactionContextInterface, key string) (*Invoice, error) {
	data, err := ctx.GetStub().GetState(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read state: %w", err)
	}
	if data == nil {
		return nil, nil
	}
	var inv Invoice
	if err := json.Unmarshal(data, &inv); err != nil {
		return nil, err
	}
	return &inv, nil
}

func putInvoice(ctx contractapi.TransactionContextInterface, key string, inv *Invoice) error {
	data, err := json.Marshal(inv)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutState(key, data)
}

func emitEvent(ctx contractapi.TransactionContextInterface, name string, inv *Invoice, ts, txID string) error {
	event := InvoiceEvent{
		InvoiceID:      inv.InvoiceID,
		SupplierMSPID:  inv.SupplierMSPID,
		BuyerMSPID:     inv.BuyerMSPID,
		FinancierMSPID: inv.FinancierMSPID,
		Status:         inv.Status,
		Timestamp:      ts,
		TxID:           txID,
	}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return ctx.GetStub().SetEvent(name, data)
}
