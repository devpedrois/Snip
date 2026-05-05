package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID_IgnoresClientHeader(t *testing.T) {
	clientProvidedID := "client-supplied-id-12345"

	var responseID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responseID = GetRequestID(r.Context())
	})

	mw := RequestID(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", clientProvidedID)

	mw.ServeHTTP(rec, req)

	assert.NotEqual(t, clientProvidedID, responseID, "client X-Request-ID must be ignored")
	assert.NotEmpty(t, responseID)
}

func TestRequestID_SetsResponseHeader(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	mw := RequestID(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mw.ServeHTTP(rec, req)

	headerID := rec.Header().Get("X-Request-ID")
	require.NotEmpty(t, headerID, "X-Request-ID response header must be set")
}

func TestRequestID_AccessibleViaContext(t *testing.T) {
	var contextID string
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contextID = GetRequestID(r.Context())
	})

	mw := RequestID(inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	mw.ServeHTTP(rec, req)

	headerID := rec.Header().Get("X-Request-ID")
	require.NotEmpty(t, contextID, "request ID must be accessible from context")
	assert.Equal(t, headerID, contextID, "context ID must match response header ID")
}

func TestRequestID_UniquePerRequest(t *testing.T) {
	ids := make([]string, 0, 2)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids = append(ids, GetRequestID(r.Context()))
	})

	mw := RequestID(inner)

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		mw.ServeHTTP(rec, req)
	}

	require.Len(t, ids, 2)
	assert.NotEqual(t, ids[0], ids[1], "consecutive requests must get different IDs")
}
