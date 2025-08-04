package types

type RpcStatus string

const (
	Ok             RpcStatus = "PASS"
	Error          RpcStatus = "FAIL"
	NotImplemented RpcStatus = "NOT_IMPL"
	Skipped        RpcStatus = "SKIP"
	Deprecated     RpcStatus = "DEPRECATED"
)

type RpcName string

type RpcResult struct {
	Method      RpcName
	Status      RpcStatus
	Value       interface{}
	ErrMsg      string
	Category    string // Main category (namespace)
	Subcategory string // Subcategory (functional grouping)
}

type TestSummary struct {
	Passed         int
	Failed         int
	NotImplemented int
	Skipped        int
	Deprecated     int
	Total          int
	Categories     map[string]*CategorySummary
	Subcategories  map[string]map[string]*CategorySummary // category -> subcategory -> summary
}

type CategorySummary struct {
	Name           string
	Passed         int
	Failed         int
	NotImplemented int
	Skipped        int
	Deprecated     int
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
	case Deprecated:
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
	if s.Categories == nil {
		s.Categories = make(map[string]*CategorySummary)
	}
	if s.Subcategories == nil {
		s.Subcategories = make(map[string]map[string]*CategorySummary)
	}
	
	category := result.Category
	if category == "" {
		category = "Uncategorized"
	}
	
	subcategory := result.Subcategory
	if subcategory == "" {
		subcategory = "Other"
	}
	
	// Initialize category if it doesn't exist
	if s.Categories[category] == nil {
		s.Categories[category] = &CategorySummary{Name: category}
	}
	
	// Initialize subcategory tracking
	if s.Subcategories[category] == nil {
		s.Subcategories[category] = make(map[string]*CategorySummary)
	}
	if s.Subcategories[category][subcategory] == nil {
		s.Subcategories[category][subcategory] = &CategorySummary{Name: subcategory}
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
	case Skipped:
		s.Skipped++
	case Deprecated:
		s.Deprecated++
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
	case Skipped:
		catSummary.Skipped++
	case Deprecated:
		catSummary.Deprecated++
	}
	
	// Update subcategory summary
	subSummary := s.Subcategories[category][subcategory]
	subSummary.Total++
	switch result.Status {
	case Ok:
		subSummary.Passed++
	case Error:
		subSummary.Failed++
	case NotImplemented:
		subSummary.NotImplemented++
	case Skipped:
		subSummary.Skipped++
	case Deprecated:
		subSummary.Deprecated++
	}
}
