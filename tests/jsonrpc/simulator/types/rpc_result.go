package types

type RpcStatus string

const (
	Ok      RpcStatus = "ok"
	Error   RpcStatus = "error"
	Warning RpcStatus = "warning"
)

type RpcName string

type RpcResult struct {
	Method   RpcName
	Status   RpcStatus
	Value    interface{}
	Warnings []string
	ErrMsg   string
}

func GetStatusPriority(status RpcStatus) int {
	switch status {
	case Ok:
		return 1
	case Warning:
		return 2
	case Error:
		return 3
	default:
		return 4
	}
}
