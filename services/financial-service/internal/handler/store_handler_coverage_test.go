package handler

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	"metargb/financial-service/internal/service"
	pb "metargb/shared/pb/financial"
)

type stubStoreSvc struct{}

func (stubStoreSvc) GetStorePackages(ctx context.Context, codes []string) ([]*service.PackageResource, error) {
	return []*service.PackageResource{
		{ID: 1, Code: "PK1", Asset: "psc", Amount: 10, UnitPrice: 100},
		{ID: 2, Code: "PK2", Asset: "red", Amount: 5, UnitPrice: 200},
	}, nil
}

func TestStoreHandler_GetStorePackages_ok(t *testing.T) {
	h := NewStoreHandler(stubStoreSvc{})
	req := &pb.GetStorePackagesRequest{Codes: []string{"PK1", "PK2"}}
	resp, err := h.GetStorePackages(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Packages) != 2 {
		t.Fatalf("packages=%d", len(resp.Packages))
	}
}

func TestRegisterStoreOrderHandlers_noPanic(t *testing.T) {
	s := grpc.NewServer()
	RegisterStoreHandler(s, stubStoreSvc{})
	RegisterOrderHandler(s, &stubOrderSvc{})
}

type stubOrderSvc struct{}

func (stubOrderSvc) CreateOrder(ctx context.Context, userID uint64, amount int32, asset string) (string, error) {
	return "https://pay.example", nil
}

func (stubOrderSvc) HandleCallback(ctx context.Context, orderID uint64, status int32, token int64, additionalParams map[string]string) (string, error) {
	return "https://example.com/verify", nil
}
