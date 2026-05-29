package types

type RpcStatus string

const (
	Ok             RpcStatus = "PASS"
	Error          RpcStatus = "FAIL"
	NotImplemented RpcStatus = "NOT_IMPL"
	Legacy         RpcStatus = "LEGACY"
	Skipped        RpcStatus = "SKIP"
)

type RpcName string

type RpcResult struct {
	Method      RpcName
	Status      RpcStatus
	Value       interface{}
	ErrMsg      string
	Category    string // Main category (namespace)
	Description string // Test description to distinguish multiple tests with same method name
}

type TestSummary struct {
	Passed         int
	Failed         int
	NotImplemented int
	Legacy         int
	Skipped        int
	Total          int
	Categories     map[string]*CategorySummary
}

type CategorySummary struct {
	Name           string
	Passed         int
	Failed         int
	NotImplemented int
	Legacy         int
	Skipped        int
	Total          int
}

type TestCase struct {
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
	case NotImplemented:
		return 2
	case Skipped:
		return 3
	case Error:
		return 4
	default:
		return 5
	}
}

func (s *TestSummary) AddResult(result *RpcResult) {
	if s.Categories == nil {
		s.Categories = make(map[string]*CategorySummary)
	}

	category := result.Category
	if category == "" {
		category = "Uncategorized"
	}

	// Initialize category if it doesn't exist
	if s.Categories[category] == nil {
		s.Categories[category] = &CategorySummary{Name: category}
	}

	// Update overall summary
	s.Total++
	switch result.Status {
	case Ok:
		s.Passed++
	case Error:
		s.Failed++
	case NotImplemented:
		s.NotImplemented++
	case Legacy:
		s.Legacy++
	case Skipped:
		s.Skipped++
	}

	// Update category summary
	catSummary := s.Categories[category]
	catSummary.Total++
	switch result.Status {
	case Ok:
		catSummary.Passed++
	case Error:
		catSummary.Failed++
	case NotImplemented:
		catSummary.NotImplemented++
	case Legacy:
		catSummary.Legacy++
	case Skipped:
		catSummary.Skipped++
	}
}
