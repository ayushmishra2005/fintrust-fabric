package main

import (
	"fmt"

	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

const (
	SupplierMSP = "SupplierMSP"
	BuyerMSP    = "BuyerMSP"
	FinanceMSP  = "FinanceMSP"
)

func getMSPID(ctx contractapi.TransactionContextInterface) (string, error) {
	return ctx.GetClientIdentity().GetMSPID()
}

func requireMSP(ctx contractapi.TransactionContextInterface, allowed ...string) (string, error) {
	mspID, err := getMSPID(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get client MSP: %w", err)
	}
	for _, a := range allowed {
		if mspID == a {
			return mspID, nil
		}
	}
	return "", fmt.Errorf("unauthorized MSP")
}

func requireSupplier(ctx contractapi.TransactionContextInterface) error {
	_, err := requireMSP(ctx, SupplierMSP)
	return err
}

func requireBuyer(ctx contractapi.TransactionContextInterface) error {
	_, err := requireMSP(ctx, BuyerMSP)
	return err
}

func requireFinance(ctx contractapi.TransactionContextInterface) error {
	_, err := requireMSP(ctx, FinanceMSP)
	return err
}

func requireInvoiceSupplier(ctx contractapi.TransactionContextInterface, inv *Invoice) error {
	mspID, err := getMSPID(ctx)
	if err != nil {
		return err
	}
	if mspID != inv.SupplierMSPID {
		return fmt.Errorf("caller is not the invoice supplier")
	}
	return nil
}

func requireInvoiceBuyer(ctx contractapi.TransactionContextInterface, inv *Invoice) error {
	mspID, err := getMSPID(ctx)
	if err != nil {
		return err
	}
	if mspID != inv.BuyerMSPID {
		return fmt.Errorf("caller is not the invoice buyer")
	}
	return nil
}

func isParty(mspID string, inv *Invoice) bool {
	return mspID == inv.SupplierMSPID || mspID == inv.BuyerMSPID
}

func isFinancingParty(mspID string, inv *Invoice) bool {
	return mspID == inv.SupplierMSPID || mspID == FinanceMSP || mspID == inv.FinancierMSPID
}
