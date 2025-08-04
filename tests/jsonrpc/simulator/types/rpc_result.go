package types

type RpcStatus string

const (
	Ok             RpcStatus = "PASS"
	Error          RpcStatus = "FAIL"
	Warning        RpcStatus = "WARNING"
	NotImplemented RpcStatus = "NOT_IMPL"
	Skipped        RpcStatus = "SKIP"
)

type RpcName string

type RpcResult struct {
	Method   RpcName
	Status   RpcStatus
	Value    interface{}
	Warnings []string
	ErrMsg   string
	Category string
}

type TestSummary struct {
	Passed         int
	Failed         int
	NotImplemented int
	Skipped        int
	Warnings       int
	Total          int
}

type TestCategory struct {
	Name        string
	Description string
	Methods     []TestMethod
}

type TestMethod struct {
	Name        RpcName
	Handler     interface{}
	Description string
	SkipReason  string
}

func GetStatusPriority(status RpcStatus) int {
	switch status {
	case Ok:
		return 1
	case Warning:
		return 2
	case NotImplemented:
		return 3
	case Skipped:
		return 4
	case Error:
		return 5
	default:
		return 6
	}
}

func (s *TestSummary) AddResult(result *RpcResult) {
	s.Total++
	switch result.Status {
	case Ok:
		s.Passed++
	case Error:
		s.Failed++
	case Warning:
		s.Warnings++
	case NotImplemented:
		s.NotImplemented++
	case Skipped:
		s.Skipped++
	}
}
