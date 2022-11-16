package core

const (
	UUIDHeader  = "x-runner-uuid"
	TokenHeader = "x-runner-token"
)

// Runner struct
type Runner struct {
	ID      int64  `json:"id"`
	UUID    string `json:"uuid"`
	Name    string `json:"name"`
	Token   string `json:"token"`
	Address string `json:"address"`
}
