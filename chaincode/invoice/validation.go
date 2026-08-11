package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	invoiceIDPattern    = regexp.MustCompile(`^[A-Z0-9_-]{3,64}$`)
	documentHashPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
	currencyPattern     = regexp.MustCompile(`^[A-Z]{3}$`)
	dueDatePattern      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

const (
	maxPaymentTermsLen  = 256
	maxAccountNameLen   = 128
	maxBankNameLen      = 128
	maxAccountIDLen     = 64
	maxRoutingCodeLen   = 32
	maxPaymentRefLen    = 128
	maxTenorLen         = 64
	maxMaturityTermsLen = 256
	minSaltLen          = 16
	maxDiscountBps      = 10000
)

func canonicalizeInvoiceID(id string) (string, error) {
	trimmed := strings.TrimSpace(id)
	upper := strings.ToUpper(trimmed)
	for _, r := range upper {
		if unicode.IsSpace(r) {
			return "", fmt.Errorf("invoice ID contains internal whitespace")
		}
	}
	if !invoiceIDPattern.MatchString(upper) {
		return "", fmt.Errorf("invalid invoice ID format")
	}
	return upper, nil
}

func validateDocumentHash(hash string) error {
	if !documentHashPattern.MatchString(hash) {
		return fmt.Errorf("invalid document hash format")
	}
	return nil
}

func validateBuyerMSP(mspID string) error {
	if mspID != "BuyerMSP" {
		return fmt.Errorf("invalid buyer MSP")
	}
	return nil
}

func validateCommercialTerms(ct *CommercialTerms, expectedID string) error {
	if ct.InvoiceID != expectedID {
		return fmt.Errorf("invoice ID mismatch in commercial terms")
	}
	if ct.AmountMinor <= 0 {
		return fmt.Errorf("amount must be positive")
	}
	if !currencyPattern.MatchString(ct.Currency) {
		return fmt.Errorf("invalid currency format")
	}
	if !dueDatePattern.MatchString(ct.DueDate) {
		return fmt.Errorf("invalid due date format")
	}
	if len(ct.PaymentTerms) > maxPaymentTermsLen {
		return fmt.Errorf("payment terms too long")
	}
	if len(ct.Salt) < minSaltLen {
		return fmt.Errorf("salt too short")
	}
	return nil
}

func validatePaymentDetails(pd *PaymentDetails, expectedID string) error {
	if pd.InvoiceID != expectedID {
		return fmt.Errorf("invoice ID mismatch in payment details")
	}
	if len(pd.AccountName) == 0 || len(pd.AccountName) > maxAccountNameLen {
		return fmt.Errorf("invalid account name length")
	}
	if len(pd.BankName) == 0 || len(pd.BankName) > maxBankNameLen {
		return fmt.Errorf("invalid bank name length")
	}
	if len(pd.AccountIdentifier) == 0 || len(pd.AccountIdentifier) > maxAccountIDLen {
		return fmt.Errorf("invalid account identifier length")
	}
	if len(pd.RoutingCode) > maxRoutingCodeLen {
		return fmt.Errorf("routing code too long")
	}
	if len(pd.PaymentReference) > maxPaymentRefLen {
		return fmt.Errorf("payment reference too long")
	}
	if len(pd.Salt) < minSaltLen {
		return fmt.Errorf("salt too short")
	}
	return nil
}

func validateFinancingRequest(fr *FinancingRequest, expectedID string, maxAmount int64) error {
	if fr.InvoiceID != expectedID {
		return fmt.Errorf("invoice ID mismatch in financing request")
	}
	if fr.RequestedAmountMinor <= 0 {
		return fmt.Errorf("requested amount must be positive")
	}
	if fr.RequestedAmountMinor > maxAmount {
		return fmt.Errorf("requested amount exceeds invoice amount")
	}
	if len(fr.RequestedTenor) == 0 || len(fr.RequestedTenor) > maxTenorLen {
		return fmt.Errorf("invalid tenor length")
	}
	if len(fr.Salt) < minSaltLen {
		return fmt.Errorf("salt too short")
	}
	return nil
}

func validateDisbursementDetails(dd *DisbursementDetails, expectedID string) error {
	if dd.InvoiceID != expectedID {
		return fmt.Errorf("invoice ID mismatch in disbursement details")
	}
	if len(dd.AccountName) == 0 || len(dd.AccountName) > maxAccountNameLen {
		return fmt.Errorf("invalid account name length")
	}
	if len(dd.BankName) == 0 || len(dd.BankName) > maxBankNameLen {
		return fmt.Errorf("invalid bank name length")
	}
	if len(dd.AccountIdentifier) == 0 || len(dd.AccountIdentifier) > maxAccountIDLen {
		return fmt.Errorf("invalid account identifier length")
	}
	if len(dd.RoutingCode) > maxRoutingCodeLen {
		return fmt.Errorf("routing code too long")
	}
	if len(dd.Salt) < minSaltLen {
		return fmt.Errorf("salt too short")
	}
	return nil
}

func validateFinancingAgreement(fa *FinancingAgreement, expectedID string, maxAmount, requestedAmount int64) error {
	if fa.InvoiceID != expectedID {
		return fmt.Errorf("invoice ID mismatch in financing agreement")
	}
	if fa.FinancedAmountMinor <= 0 {
		return fmt.Errorf("financed amount must be positive")
	}
	if fa.FinancedAmountMinor > maxAmount {
		return fmt.Errorf("financed amount exceeds invoice amount")
	}
	if fa.FinancedAmountMinor > requestedAmount {
		return fmt.Errorf("financed amount exceeds requested amount")
	}
	if fa.DiscountBps < 0 || fa.DiscountBps > maxDiscountBps {
		return fmt.Errorf("invalid discount basis points")
	}
	if len(fa.MaturityTerms) == 0 || len(fa.MaturityTerms) > maxMaturityTermsLen {
		return fmt.Errorf("invalid maturity terms length")
	}
	if len(fa.Salt) < minSaltLen {
		return fmt.Errorf("salt too short")
	}
	return nil
}

func isValidStatus(status string) bool {
	switch status {
	case StatusCreated, StatusApproved, StatusRejected,
		StatusFinancingRequested, StatusFinanced, StatusSettled:
		return true
	}
	return false
}
