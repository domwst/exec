package main

import (
	"encoding/json"
	"exec/common"
)

func CreateErrResponse(errMsg string) []byte {
	type Err struct {
		Error string `json:"error"`
	}
	data, e := json.Marshal(&Err{Error: errMsg})
	common.HandlePanic(e)
	return data
}
