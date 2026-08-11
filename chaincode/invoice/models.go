package main

const (
	DocTypeInvoice = "invoice"
	SchemaVersion  = "1.0"

	StatusCreated            = "CREATED"
	StatusApproved           = "APPROVED"
	StatusRejected           = "REJECTED"
	StatusFinancingRequested = "FINANCING_REQUESTED"
	StatusFinanced           = "FINANCED"
	StatusSettled            = "SETTLED"

	CollectionInvoiceParties  = "collectionInvoiceParties"
	CollectionSupplierFinance = "collectionSupplierFinance"
)

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

type InvoiceEvent struct {
	InvoiceID      string `json:"invoiceId"`
	SupplierMSPID  string `json:"supplierMspId"`
	BuyerMSPID     string `json:"buyerMspId"`
	FinancierMSPID string `json:"financierMspId,omitempty"`
	Status         string `json:"status"`
	Timestamp      string `json:"timestamp"`
	TxID           string `json:"txId"`
}
