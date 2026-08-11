package main

import (
	"encoding/json"
	"fmt"

	"github.com/hyperledger/fabric-chaincode-go/v2/pkg/statebased"
	"github.com/hyperledger/fabric-contract-api-go/v2/contractapi"
)

func invoiceKey(id string) string {
	return "invoice~" + id
}

func commercialTermsKey(id string) string {
	return "commercialTerms~" + id
}

func paymentDetailsKey(id string) string {
	return "paymentDetails~" + id
}

func disclosureKey(id string) string {
	return "disclosure~" + id
}

func financingRequestKey(id string) string {
	return "financingRequest~" + id
}

func disbursementDetailsKey(id string) string {
	return "disbursementDetails~" + id
}

func financingAgreementKey(id string) string {
	return "financingAgreement~" + id
}

func getTransientField(ctx contractapi.TransactionContextInterface, field string) ([]byte, error) {
	transient, err := ctx.GetStub().GetTransient()
	if err != nil {
		return nil, fmt.Errorf("failed to get transient map: %w", err)
	}
	data, ok := transient[field]
	if !ok || len(data) == 0 {
		return nil, fmt.Errorf("missing transient field: %s", field)
	}
	return data, nil
}

func parseTransient[T any](ctx contractapi.TransactionContextInterface, field string) (*T, error) {
	data, err := getTransientField(ctx, field)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(jsonReader(data))
	decoder.DisallowUnknownFields()
	var result T
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid %s format", field)
	}
	return &result, nil
}

type byteReader struct {
	data []byte
	pos  int
}

func jsonReader(data []byte) *byteReader {
	return &byteReader{data: data}
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func canonicalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}

func setSBE(ctx contractapi.TransactionContextInterface, key string, orgs ...string) error {
	ep, err := statebased.NewStateEP(nil)
	if err != nil {
		return err
	}
	for _, org := range orgs {
		err = ep.AddOrgs(statebased.RoleTypePeer, org)
		if err != nil {
			return err
		}
	}
	policy, err := ep.Policy()
	if err != nil {
		return err
	}
	return ctx.GetStub().SetStateValidationParameter(key, policy)
}

func putPrivateData(ctx contractapi.TransactionContextInterface, collection, key string, value any) error {
	data, err := canonicalJSON(value)
	if err != nil {
		return err
	}
	return ctx.GetStub().PutPrivateData(collection, key, data)
}

func getPrivateData[T any](ctx contractapi.TransactionContextInterface, collection, key string) (*T, error) {
	data, err := ctx.GetStub().GetPrivateData(collection, key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var result T
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func privateDataExists(ctx contractapi.TransactionContextInterface, collection, key string) (bool, error) {
	data, err := ctx.GetStub().GetPrivateData(collection, key)
	if err != nil {
		return false, err
	}
	return data != nil, nil
}

func getPrivateDataHash(ctx contractapi.TransactionContextInterface, collection, key string) ([]byte, error) {
	return ctx.GetStub().GetPrivateDataHash(collection, key)
}
