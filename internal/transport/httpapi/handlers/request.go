package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
)

func DecodeJSON(r *http.Request, dest any) error {
	if r.Body == nil {
		return errors.New("empty request body")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return errors.New("invalid JSON payload")
	}

	return nil
}
