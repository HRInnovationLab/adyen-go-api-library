package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/HRInnovationLab/adyen-go-api-library/v5/src/adyen"
	"github.com/HRInnovationLab/adyen-go-api-library/v5/src/checkout"
	"github.com/HRInnovationLab/adyen-go-api-library/v5/src/common"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

var idempotencyKeys *sync.Map

type referenceAndIdempotencyKeyTransport struct {
	errChan chan error
}

func (rl *referenceAndIdempotencyKeyTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, _ := ioutil.ReadAll(req.Body)
	data := map[string]interface{}{}
	json.Unmarshal(body, &data)
	reference := data["reference"]
	idempotencyKey := req.Header.Get("Idempotency-Key")
	idempotencyKeys.Store(reference.(string), idempotencyKey)
	resp := httptest.NewRecorder()
	resp.WriteHeader(http.StatusOK)
	return resp.Result(), nil
}

func Test_Checkout_Idempotency_Race(t *testing.T) {
	idempotencyKeys = &sync.Map{}
	client := adyen.NewClient(&common.Config{
		HTTPClient: &http.Client{
			Transport: &referenceAndIdempotencyKeyTransport{},
		},
	})

	for r := 0; r < 10; r++ {
		t.Run(fmt.Sprintf("Routine %d", r), func(t *testing.T) {
			ir := r
			t.Parallel()
			for i := 0; i < 100; i++ {
				idempotencyKey := uuid.Must(uuid.NewRandom()).String()
				ref := fmt.Sprintf("%d/%d", ir, i)
				ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
				defer cancel()
				ctx = common.WithIdempotencyKey(ctx, idempotencyKey)
				_, _, err := client.Checkout.Payments(&checkout.PaymentRequest{
					Reference: ref,
				}, ctx)
				require.NoError(t, err)
				v, ok := idempotencyKeys.Load(ref)
				require.True(t, ok)
				require.Equal(t, v.(string), idempotencyKey)
			}
		})
	}
}
