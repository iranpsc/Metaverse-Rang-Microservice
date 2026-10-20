package handler_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"metarang/commercial-service/internal/handler"
	"metarang/commercial-service/internal/repository"
	pb "metarang/shared/pb/commercial"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestAssetRateHandler_GetAssetRate(t *testing.T) {
	for _, tc := range []struct {
		name string
		rate float64
		err  error
		code codes.Code
	}{
		{"fractional rate", 400.5, nil, codes.OK},
		{"zero", 0, nil, codes.FailedPrecondition},
		{"negative", -1, nil, codes.FailedPrecondition},
		{"nan", math.NaN(), nil, codes.FailedPrecondition},
		{"infinite", math.Inf(1), nil, codes.FailedPrecondition},
		{"database failure", 0, errors.New("database down"), codes.Internal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			query := mock.ExpectQuery("SELECT price").WithArgs("psc")
			if tc.err != nil {
				query.WillReturnError(tc.err)
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"price"}).AddRow(tc.rate))
			}
			h := handler.NewAssetRateHandler(repository.NewVariableRepository(db))
			resp, err := h.GetAssetRate(context.Background(), &pb.GetAssetRateRequest{Asset: "psc"})
			assert.Equal(t, tc.code, status.Code(err))
			if tc.code == codes.OK {
				require.NotNil(t, resp)
				assert.Equal(t, tc.rate, resp.RialPerUnit)
			} else {
				assert.Nil(t, resp)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
	h := handler.NewAssetRateHandler(nil)
	for _, req := range []*pb.GetAssetRateRequest{nil, {}} {
		_, err := h.GetAssetRate(context.Background(), req)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	}
}
