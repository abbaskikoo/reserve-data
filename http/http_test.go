package http

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/KyberNetwork/reserve-data/common"
	"github.com/KyberNetwork/reserve-data/core"
	"github.com/KyberNetwork/reserve-data/data"
	"github.com/KyberNetwork/reserve-data/data/storage"
	"github.com/KyberNetwork/reserve-data/http/httputil"
	"github.com/KyberNetwork/reserve-data/metric"
	ethereum "github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
)

type assertFn func(t *testing.T, resp *httptest.ResponseRecorder)

type testCase struct {
	msg      string
	endpoint string
	method   string
	data     string
	assert   assertFn
}

func testHTTPRequest(t *testing.T, tc testCase, handler http.Handler) {
	t.Helper()

	req, tErr := http.NewRequest(tc.method, tc.endpoint, nil)
	if tErr != nil {
		t.Fatal(tErr)
	}

	if tc.data != "" {
		form := url.Values{}
		form.Add("data", tc.data)
		req.PostForm = form
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	tc.assert(t, resp)
}

func TestHTTPServerPWIEquationV2(t *testing.T) {
	const (
		storePendingPWIEquationV2Endpoint = "/v2/set-pwis-equation"
		getPendingPWIEquationV2Endpoint   = "/v2/pending-pwis-equation"
		confirmPWIEquationV2              = "/v2/confirm-pwis-equation"
		testData                          = `{
  "EOS": {
    "bid": {
      "a": 750,
      "b": 500,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    },
    "ask": {
      "a": 800,
      "b": 600,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    }
  },
  "ETH": {
    "bid": {
      "a": 750,
      "b": 500,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    },
    "ask": {
      "a": 800,
      "b": 600,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    }
  }
}
`
		testDataWrongConfirmation = `{
  "EOS": {
    "bid": {
      "a": 751,
      "b": 500,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    },
    "ask": {
      "a": 800,
      "b": 600,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    }
  },
  "ETH": {
    "bid": {
      "a": 750,
      "b": 500,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    },
    "ask": {
      "a": 800,
      "b": 600,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    }
  }
}
`
		testDataUnsupported = `{
  "OMG": {
    "bid": {
      "a": 750,
      "b": 500,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    },
    "ask": {
      "a": 800,
      "b": 600,
      "c": 0,
      "MinMinSpread": 0,
      "PriceMultiplyFactor": 0
    }
  }
`
	)

	common.RegisterInternalActiveToken(common.Token{ID: "EOS"})
	common.RegisterInternalActiveToken(common.Token{ID: "ETH"})

	tmpDir, err := ioutil.TempDir("", "test_pwi_equation_v2")
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if rErr := os.RemoveAll(tmpDir); rErr != nil {
			t.Error(rErr)
		}
	}()

	st, err := storage.NewBoltStorage(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}

	s := HTTPServer{
		app:         data.NewReserveData(st, nil, nil, nil, nil),
		core:        core.NewReserveCore(nil, st, ethereum.Address{}),
		metric:      st,
		authEnabled: false,
		r:           gin.Default()}
	s.register()

	var tests = []testCase{
		{
			msg:      "invalid post form",
			endpoint: storePendingPWIEquationV2Endpoint,
			method:   http.MethodPost,
			data:     `{"invalid_key": "invalid_value"}`,
			assert:   httputil.ExpectFailure,
		},
		{
			msg:      "getting non exists pending equation",
			endpoint: getPendingPWIEquationV2Endpoint,
			method:   http.MethodGet,
			assert:   httputil.ExpectFailure,
		},
		{
			msg:      "unsupported token",
			endpoint: storePendingPWIEquationV2Endpoint,
			method:   http.MethodPost,
			data:     testDataUnsupported,
			assert:   httputil.ExpectFailure,
		},
		{
			msg:      "valid post form",
			endpoint: storePendingPWIEquationV2Endpoint,
			method:   http.MethodPost,
			data:     testData,
			assert:   httputil.ExpectSuccess,
		},
		{
			msg:      "setting when pending exists",
			endpoint: storePendingPWIEquationV2Endpoint,
			method:   http.MethodPost,
			data:     testData,
			assert:   httputil.ExpectFailure,
		},
		{
			msg:      "getting existing pending equation",
			endpoint: getPendingPWIEquationV2Endpoint,
			method:   http.MethodGet,
			assert: func(t *testing.T, resp *httptest.ResponseRecorder) {
				if resp.Code != http.StatusOK {
					t.Fatalf("wrong return code, expected: %d, got: %d", http.StatusOK, resp.Code)
				}

				type responseBody struct {
					Success bool
					Data    metric.PWIEquationRequestV2
				}

				decoded := responseBody{}
				if aErr := json.NewDecoder(resp.Body).Decode(&decoded); aErr != nil {
					t.Fatal(aErr)
				}

				if decoded.Success != true {
					t.Errorf("wrong success status, expected: %t, got: %t", true, decoded.Success)
				}

				t.Logf("returned pending PWI equation request: %v", decoded.Data)

				if len(decoded.Data) != 2 {
					t.Fatalf("wrong number of tokens, expected: %d, got %d", 2, len(decoded.Data))
				}
			},
		},
		{
			msg:      "confirm when no pending equation request exists",
			endpoint: confirmPWIEquationV2,
			method:   http.MethodPost,
			assert:   httputil.ExpectFailure,
		},
		{
			msg:      "confirm with wrong data",
			endpoint: confirmPWIEquationV2,
			method:   http.MethodPost,
			data:     testDataWrongConfirmation,
			assert:   httputil.ExpectFailure,
		},
		{
			msg:      "confirm with correct data",
			endpoint: confirmPWIEquationV2,
			method:   http.MethodPost,
			data:     testData,
			assert:   httputil.ExpectSuccess,
		},
	}

	for _, tc := range tests {
		t.Run(tc.msg, func(t *testing.T) { testHTTPRequest(t, tc, s.r) })
	}
}
