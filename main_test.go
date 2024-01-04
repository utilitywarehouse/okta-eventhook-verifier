package main

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func Test_verificationHandler(t *testing.T) {
	type req struct {
		method      string
		headerKey   string
		headerValue string
	}

	tests := []struct {
		name      string
		req       req
		respSatus int
		exptBody  string
	}{
		{
			"valid",
			req{"GET", oktaVerificationHeader, "random-test-shared-key"},
			200,
			`{"verification" : "random-test-shared-key"}`,
		},
		{
			"invalid-method",
			req{"POST", oktaVerificationHeader, "random-test-shared-key"},
			400, ``,
		},
		{
			"wrong-header",
			req{"GET", "Authorization", "random-test-shared-key"},
			400, ``,
		},
		{
			"empty-header",
			req{"GET", oktaVerificationHeader, "  "},
			400, ``,
		},
	}
	for _, tt := range tests {
		wg := &sync.WaitGroup{}
		wg.Add(1)
		req, err := http.NewRequest(tt.req.method, "/eventhook/test", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set(tt.req.headerKey, tt.req.headerValue)
		rr := httptest.NewRecorder()
		handler := verificationHandler(wg)
		handler.ServeHTTP(rr, req)

		// Check the status code and body is what we expect.
		if status := rr.Code; status != tt.respSatus {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, tt.respSatus)
		}

		if rr.Body.String() != tt.exptBody {
			t.Errorf("handler returned unexpected body: got %v want %v",
				rr.Body.String(), tt.exptBody)
		}
	}
}
