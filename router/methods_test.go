package router

import (
	"net/http"
	"testing"
)

func TestRouter_PutAndDeleteRegisterTheirMethod(t *testing.T) {
	r := NewRouter()
	r.Put("/items", writeText("put"))
	r.Delete("/items", writeText("delete"))

	if body := serve(r, http.MethodPut, "/items").Body.String(); body != "put" {
		t.Errorf("PUT: expected %q, got %q", "put", body)
	}
	if body := serve(r, http.MethodDelete, "/items").Body.String(); body != "delete" {
		t.Errorf("DELETE: expected %q, got %q", "delete", body)
	}
}
