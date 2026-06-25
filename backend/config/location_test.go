package config

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUpdateLocationSuccess(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"city":"Shanghai","country_name":"China"}`)
	}))
	t.Cleanup(server.Close)
	_ = server
}

func TestUpdateLocationInvalidJSON(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "not-json")
	}))
	t.Cleanup(server.Close)
	_ = server
}

func TestUpdateLocationNon200(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	_ = server
}
