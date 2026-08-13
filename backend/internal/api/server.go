package api

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/fintrust-fabric/backend/internal/fabric"
	"github.com/fintrust-fabric/backend/internal/projection"
)

const maxBodySize = 64 * 1024

type Server struct {
	fabric *fabric.Client
	store  *projection.Store
	mux    *http.ServeMux
}

func NewServer(fc *fabric.Client, store *projection.Store) *Server {
	s := &Server{fabric: fc, store: store, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealth)
	s.mux.HandleFunc("POST /api/v1/invoices", s.handleCreateInvoice)
	s.mux.HandleFunc("GET /api/v1/invoices", s.handleQueryInvoices)
	s.mux.HandleFunc("GET /api/v1/invoices/{id}", s.handleGetInvoice)
	s.mux.HandleFunc("GET /api/v1/invoices/{id}/private", s.handleGetPrivateData)
	s.mux.HandleFunc("GET /api/v1/invoices/{id}/financing", s.handleGetFinancingTerms)
	s.mux.HandleFunc("GET /api/v1/invoices/{id}/events", s.handleGetInvoiceEvents)
	s.mux.HandleFunc("POST /api/v1/invoices/{id}/approve", s.handleApprove)
	s.mux.HandleFunc("POST /api/v1/invoices/{id}/reject", s.handleReject)
	s.mux.HandleFunc("POST /api/v1/invoices/{id}/financing-request", s.handleRequestFinancing)
	s.mux.HandleFunc("POST /api/v1/invoices/{id}/finance", s.handleFinance)
	s.mux.HandleFunc("POST /api/v1/invoices/{id}/settle", s.handleSettle)
	s.mux.HandleFunc("GET /api/v1/events", s.handleQueryEvents)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type CreateInvoiceRequest struct {
	InvoiceID    string           `json:"invoiceId"`
	BuyerMSPID   string           `json:"buyerMspId"`
	DocumentHash string           `json:"documentHash"`
	Commercial   *CommercialTerms `json:"commercialTerms"`
	Payment      *PaymentDetails  `json:"paymentDetails"`
}

type CommercialTerms struct {
	AmountMinor  int64  `json:"amountMinor"`
	Currency     string `json:"currency"`
	DueDate      string `json:"dueDate"`
	PaymentTerms string `json:"paymentTerms"`
	Salt         string `json:"salt"`
}

type PaymentDetails struct {
	AccountName       string `json:"accountName"`
	BankName          string `json:"bankName"`
	AccountIdentifier string `json:"accountIdentifier"`
	RoutingCode       string `json:"routingCode"`
	PaymentReference  string `json:"paymentReference"`
	Salt              string `json:"salt"`
}

func (s *Server) handleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	var req CreateInvoiceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.InvoiceID == "" || req.BuyerMSPID == "" || req.DocumentHash == "" {
		writeError(w, http.StatusBadRequest, "missing required fields")
		return
	}
	if req.Commercial == nil || req.Payment == nil {
		writeError(w, http.StatusBadRequest, "missing commercial or payment data")
		return
	}

	ct := map[string]any{
		"schemaVersion": "1.0",
		"invoiceId":     req.InvoiceID,
		"amountMinor":   req.Commercial.AmountMinor,
		"currency":      req.Commercial.Currency,
		"dueDate":       req.Commercial.DueDate,
		"paymentTerms":  req.Commercial.PaymentTerms,
		"salt":          req.Commercial.Salt,
	}
	pd := map[string]any{
		"invoiceId":         req.InvoiceID,
		"accountName":       req.Payment.AccountName,
		"bankName":          req.Payment.BankName,
		"accountIdentifier": req.Payment.AccountIdentifier,
		"routingCode":       req.Payment.RoutingCode,
		"paymentReference":  req.Payment.PaymentReference,
		"salt":              req.Payment.Salt,
	}

	ctBytes, _ := json.Marshal(ct)
	pdBytes, _ := json.Marshal(pd)

	_, err := s.fabric.SubmitWithTransient(r.Context(), "CreateInvoice",
		map[string][]byte{
			"commercial_terms": ctBytes,
			"payment_details":  pdBytes,
		},
		req.InvoiceID, req.BuyerMSPID, req.DocumentHash,
	)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	log.Printf("invoice created: %s", req.InvoiceID)
	writeJSON(w, http.StatusCreated, map[string]string{"invoiceId": req.InvoiceID, "status": "CREATED"})
}

func (s *Server) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing invoice id")
		return
	}

	result, err := s.fabric.Evaluate(r.Context(), "ReadPublicInvoice", id)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	var jsonStr string
	if err := json.Unmarshal(result, &jsonStr); err != nil {
		jsonStr = string(result)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(jsonStr))
}

func (s *Server) handleQueryInvoices(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		writeError(w, http.StatusBadRequest, "status parameter required")
		return
	}

	validStatuses := map[string]bool{
		"CREATED": true, "APPROVED": true, "REJECTED": true,
		"FINANCING_REQUESTED": true, "FINANCED": true, "SETTLED": true,
	}
	if !validStatuses[status] {
		writeError(w, http.StatusBadRequest, "invalid status")
		return
	}

	result, err := s.fabric.Evaluate(r.Context(), "QueryPublicInvoicesByStatus", status)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if result == nil || string(result) == "null" {
		w.Write([]byte("[]"))
	} else {
		w.Write(result)
	}
}

func (s *Server) handleGetPrivateData(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing invoice id")
		return
	}

	result, err := s.fabric.Evaluate(r.Context(), "ReadPrivateInvoiceData", id)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(result)
}

func (s *Server) handleGetFinancingTerms(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing invoice id")
		return
	}

	result, err := s.fabric.Evaluate(r.Context(), "ReadFinancingTerms", id)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(result)
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing invoice id")
		return
	}

	_, err := s.fabric.Submit(r.Context(), "ApproveInvoice", id)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	log.Printf("invoice approved: %s", id)
	writeJSON(w, http.StatusOK, map[string]string{"invoiceId": id, "status": "APPROVED"})
}

func (s *Server) handleReject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing invoice id")
		return
	}

	_, err := s.fabric.Submit(r.Context(), "RejectInvoice", id)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	log.Printf("invoice rejected: %s", id)
	writeJSON(w, http.StatusOK, map[string]string{"invoiceId": id, "status": "REJECTED"})
}

type FinancingRequestInput struct {
	Disclosure   *CommercialTerms     `json:"disclosure"`
	Request      *FinancingRequest    `json:"financingRequest"`
	Disbursement *DisbursementDetails `json:"disbursementDetails"`
}

type FinancingRequest struct {
	RequestedAmountMinor int64  `json:"requestedAmountMinor"`
	RequestedTenor       string `json:"requestedTenor"`
	Salt                 string `json:"salt"`
}

type DisbursementDetails struct {
	AccountName       string `json:"accountName"`
	BankName          string `json:"bankName"`
	AccountIdentifier string `json:"accountIdentifier"`
	RoutingCode       string `json:"routingCode"`
	Salt              string `json:"salt"`
}

func (s *Server) handleRequestFinancing(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing invoice id")
		return
	}

	var req FinancingRequestInput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Disclosure == nil || req.Request == nil || req.Disbursement == nil {
		writeError(w, http.StatusBadRequest, "missing required financing data")
		return
	}

	disclosure := map[string]any{
		"schemaVersion": "1.0",
		"invoiceId":     id,
		"amountMinor":   req.Disclosure.AmountMinor,
		"currency":      req.Disclosure.Currency,
		"dueDate":       req.Disclosure.DueDate,
		"paymentTerms":  req.Disclosure.PaymentTerms,
		"salt":          req.Disclosure.Salt,
	}
	fr := map[string]any{
		"invoiceId":            id,
		"requestedAmountMinor": req.Request.RequestedAmountMinor,
		"requestedTenor":       req.Request.RequestedTenor,
		"salt":                 req.Request.Salt,
	}
	dd := map[string]any{
		"invoiceId":         id,
		"accountName":       req.Disbursement.AccountName,
		"bankName":          req.Disbursement.BankName,
		"accountIdentifier": req.Disbursement.AccountIdentifier,
		"routingCode":       req.Disbursement.RoutingCode,
		"salt":              req.Disbursement.Salt,
	}

	discBytes, _ := json.Marshal(disclosure)
	frBytes, _ := json.Marshal(fr)
	ddBytes, _ := json.Marshal(dd)

	_, err := s.fabric.SubmitWithTransient(r.Context(), "RequestFinancing",
		map[string][]byte{
			"invoice_disclosure":   discBytes,
			"financing_request":    frBytes,
			"disbursement_details": ddBytes,
		},
		id,
	)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	log.Printf("financing requested: %s", id)
	writeJSON(w, http.StatusOK, map[string]string{"invoiceId": id, "status": "FINANCING_REQUESTED"})
}

type FinanceInput struct {
	Agreement *FinancingAgreement `json:"financingAgreement"`
}

type FinancingAgreement struct {
	FinancedAmountMinor int64  `json:"financedAmountMinor"`
	DiscountBps         int    `json:"discountBps"`
	MaturityTerms       string `json:"maturityTerms"`
	Salt                string `json:"salt"`
}

func (s *Server) handleFinance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing invoice id")
		return
	}

	var req FinanceInput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Agreement == nil {
		writeError(w, http.StatusBadRequest, "missing financing agreement")
		return
	}

	fa := map[string]any{
		"invoiceId":           id,
		"financedAmountMinor": req.Agreement.FinancedAmountMinor,
		"discountBps":         req.Agreement.DiscountBps,
		"maturityTerms":       req.Agreement.MaturityTerms,
		"salt":                req.Agreement.Salt,
	}
	faBytes, _ := json.Marshal(fa)

	_, err := s.fabric.SubmitWithTransient(r.Context(), "FinanceInvoice",
		map[string][]byte{"financing_agreement": faBytes},
		id,
	)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	log.Printf("invoice financed: %s", id)
	writeJSON(w, http.StatusOK, map[string]string{"invoiceId": id, "status": "FINANCED"})
}

func (s *Server) handleSettle(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing invoice id")
		return
	}

	_, err := s.fabric.Submit(r.Context(), "SettleInvoice", id)
	if err != nil {
		handleFabricError(w, err)
		return
	}

	log.Printf("invoice settled: %s", id)
	writeJSON(w, http.StatusOK, map[string]string{"invoiceId": id, "status": "SETTLED"})
}

func (s *Server) handleQueryEvents(w http.ResponseWriter, r *http.Request) {
	filter := projection.EventFilter{
		InvoiceID: r.URL.Query().Get("invoice_id"),
		EventName: r.URL.Query().Get("event_name"),
		Limit:     50,
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			filter.Limit = n
		}
	}

	events, err := s.store.QueryEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if events == nil {
		events = []projection.InvoiceEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) handleGetInvoiceEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing invoice id")
		return
	}

	filter := projection.EventFilter{
		InvoiceID: id,
		Limit:     100,
	}

	events, err := s.store.QueryEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if events == nil {
		events = []projection.InvoiceEvent{}
	}
	writeJSON(w, http.StatusOK, events)
}

func decodeJSON(r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(nil, r.Body, maxBodySize)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if dec.More() {
		return io.EOF
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func handleFabricError(w http.ResponseWriter, err error) {
	msg := err.Error()
	if idx := strings.LastIndex(msg, ": "); idx != -1 {
		msg = msg[idx+2:]
	}
	if len(msg) > 200 {
		msg = msg[:200]
	}

	if fabric.IsNotFoundError(err) {
		writeError(w, http.StatusNotFound, msg)
		return
	}
	if fabric.IsAuthorizationError(err) {
		writeError(w, http.StatusForbidden, msg)
		return
	}
	if fabric.IsConflictError(err) {
		writeError(w, http.StatusConflict, msg)
		return
	}

	log.Printf("fabric error: %v", err)
	writeError(w, http.StatusBadGateway, "transaction failed")
}
