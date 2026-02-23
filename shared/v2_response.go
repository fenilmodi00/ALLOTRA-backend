package shared

type V2APIError struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

type V2ErrorResponse struct {
	Success bool       `json:"success"`
	Error   V2APIError `json:"error"`
}

type V2PageMeta struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasNext bool `json:"has_next"`
}

type V2PaginatedResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Meta    V2PageMeta  `json:"meta"`
}

type V2Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
}

func NewV2ErrorResponse(code, message string, details interface{}) V2ErrorResponse {
	return V2ErrorResponse{
		Success: false,
		Error:   V2APIError{Code: code, Message: message, Details: details},
	}
}

func NewV2PaginatedResponse(data interface{}, total, limit, offset int) V2PaginatedResponse {
	return V2PaginatedResponse{
		Success: true,
		Data:    data,
		Meta: V2PageMeta{
			Total:   total,
			Limit:   limit,
			Offset:  offset,
			HasNext: (offset + limit) < total,
		},
	}
}

func NewV2Response(data interface{}) V2Response {
	return V2Response{Success: true, Data: data}
}
