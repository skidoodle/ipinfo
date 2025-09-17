package server

// bogonDataStruct represents the response structure for bogon IP queries.
type bogonDataStruct struct {
	IP    string `json:"ip"`
	Bogon bool   `json:"bogon"`
}
