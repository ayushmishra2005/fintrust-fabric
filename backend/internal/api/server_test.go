package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeJSON_Valid(t *testing.T) {
	body := `{"invoiceId":"INV-001","buyerMspId":"BuyerMSP","documentHash":"sha256:abc"}`
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	var req CreateInvoiceRequest
	err := decodeJSON(r, &req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.InvoiceID != "INV-001" {
		t.Errorf("got invoiceId=%s, want INV-001", req.InvoiceID)
	}
}

func TestDecodeJSON_UnknownField(t *testing.T) {
	body := `{"invoiceId":"INV-001","unknownField":"value"}`
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	var req CreateInvoiceRequest
	err := decodeJSON(r, &req)
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestDecodeJSON_InvalidJSON(t *testing.T) {
	body := `{"invoiceId":}`
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	var req CreateInvoiceRequest
	err := decodeJSON(r, &req)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestDecodeJSON_BodyTooLarge(t *testing.T) {
	body := strings.Repeat("x", maxBodySize+1)
	r := httptest.NewRequest("POST", "/", strings.NewReader(body))
	var req CreateInvoiceRequest
	err := decodeJSON(r, &req)
	if err == nil {
		t.Fatal("expected error for body too large")
	}
}

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"id": "123"})

	if w.Code != http.StatusCreated {
		t.Errorf("got status %d, want %d", w.Code, http.StatusCreated)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type=%s, want application/json", ct)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["id"] != "123" {
		t.Errorf("got id=%s, want 123", resp["id"])
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "missing field")

	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "missing field" {
		t.Errorf("got error=%s, want missing field", resp["error"])
	}
}

func TestHealthEndpoint(t *testing.T) {
	s := &Server{mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /healthz", s.handleHealth)

	r := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("got status=%s, want ok", resp["status"])
	}
}

func TestCreateInvoiceValidation_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"missing invoiceId", `{"buyerMspId":"BuyerMSP","documentHash":"sha256:abc"}`},
		{"missing buyerMspId", `{"invoiceId":"INV-001","documentHash":"sha256:abc"}`},
		{"missing documentHash", `{"invoiceId":"INV-001","buyerMspId":"BuyerMSP"}`},
		{"missing commercialTerms", `{"invoiceId":"INV-001","buyerMspId":"BuyerMSP","documentHash":"sha256:abc","paymentDetails":{}}`},
	}

	s := &Server{mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /api/v1/invoices", s.handleCreateInvoice)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/v1/invoices", strings.NewReader(tc.body))
			r.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)

			if w.Code != http.StatusBadRequest {
				t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestQueryInvoicesValidation(t *testing.T) {
	s := &Server{mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/v1/invoices", s.handleQueryInvoices)

	t.Run("missing status", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/api/v1/invoices", nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid status", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/api/v1/invoices?status=INVALID", nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != http.StatusBadRequest {
			t.Errorf("got %d, want %d", w.Code, http.StatusBadRequest)
		}
	})
}

func TestPrivateDataNotInErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	handleFabricError(w, &testError{msg: "chaincode error: secret bank account IBAN123 invalid"})

	body := w.Body.String()
	if strings.Contains(body, "IBAN123") {
		t.Error("error response should not contain private data")
	}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

func TestRoutePatterns(t *testing.T) {
	s := &Server{mux: http.NewServeMux()}
	s.routes()

	tests := []struct {
		method string
		path   string
		want   int
	}{
		{"GET", "/healthz", http.StatusOK},
	}

	for _, tc := range tests {
		r := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Errorf("%s %s: got %d, want %d", tc.method, tc.path, w.Code, tc.want)
		}
	}
}

func TestBodySizeLimit(t *testing.T) {
	largeBody := bytes.Repeat([]byte("x"), maxBodySize+100)
	r := httptest.NewRequest("POST", "/", bytes.NewReader(largeBody))
	var req CreateInvoiceRequest
	err := decodeJSON(r, &req)
	if err == nil {
		t.Error("expected error for oversized body")
	}
}
