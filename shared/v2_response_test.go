package shared

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestV2ErrorResponse(t *testing.T) {
	errResp := NewV2ErrorResponse("NOT_FOUND", "Resource not found", nil)
	b, err := json.Marshal(errResp)
	assert.NoError(t, err)

	expected := `{"success":false,"error":{"code":"NOT_FOUND","message":"Resource not found"}}`
	assert.JSONEq(t, expected, string(b))
}

func TestV2PaginatedResponse(t *testing.T) {
	resp := NewV2PaginatedResponse([]string{"a"}, 10, 5, 0)
	b, err := json.Marshal(resp)
	assert.NoError(t, err)

	expected := `{"success":true,"data":["a"],"meta":{"total":10,"limit":5,"offset":0,"has_next":true}}`
	assert.JSONEq(t, expected, string(b))
}
