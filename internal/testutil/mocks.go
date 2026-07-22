package testutil

// MockLsofExecutor simulates lsof command execution
type MockLsofExecutor struct {
	Output string
	Err    error
}

// Execute returns the mocked lsof output
func (m *MockLsofExecutor) Execute(port int) ([]byte, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return []byte(m.Output), nil
}
