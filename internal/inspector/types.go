package inspector

type TaskParameter struct {
	Name        string
	Description string
	IsRequired  bool
}

type TaskDefinition struct {
	Name        string
	Description string
	Usage       string
	Parameters  []TaskParameter
}

type MCPConfig struct {
	Tasks []TaskDefinition
}