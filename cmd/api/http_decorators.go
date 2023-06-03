package main

import (
	"fmt"
	"net/http"
)

func RequireKey(key string, handler func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(resp http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get(key) == "" {
			resp.WriteHeader(http.StatusBadRequest)
			_, _ = resp.Write(CreateErrResponse(fmt.Sprintf("%s is required", key)))
			return
		}
		handler(resp, req)
	}
}
