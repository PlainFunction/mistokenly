package models

import "time"

type TokenizeRequest struct {
	Data            string `json:"data"`
	DataType        string `json:"dataType"`
	RetentionPolicy string `json:"retentionPolicy"`
	ClientID        string `json:"clientId"`
	OrganizationID  string `json:"organizationId"`
	OrganizationKey string `json:"organizationKey"`
}

type TokenizeResponse struct {
	ReferenceHash string    `json:"referenceHash"`
	TokenType     string    `json:"tokenType"`
	ExpiresAt     time.Time `json:"expiresAt"`
	Status        string    `json:"status"`
}

type DetokenizeRequest struct {
	ReferenceHash     string `json:"referenceHash"`
	Purpose           string `json:"purpose"`
	RequestingService string `json:"requestingService"`
	RequestingUser    string `json:"requestingUser"`
	OrganizationID    string `json:"organizationId"`
	OrganizationKey   string `json:"organizationKey"`
}

type DetokenizeResponse struct {
	Data              string    `json:"data"`
	DataType          string    `json:"dataType"`
	OriginalTimestamp time.Time `json:"originalTimestamp"`
	AccessLogged      bool      `json:"accessLogged"`
	Status            string    `json:"status"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
