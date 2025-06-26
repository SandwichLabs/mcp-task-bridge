package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/sandwichlabs/mcp-task-bridge/internal/inspector"
)

type model struct {
	list         list.Model
	choice       string
	quitting     bool
	taskConfig   *inspector.MCPConfig
	selectedTask *inspector.TaskDefinition
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "enter":
			if item, ok := m.list.SelectedItem().(listItem); ok {
				m.selectedTask = &item.TaskDefinition
			}
			return m, nil
		case "esc":
			m.selectedTask = nil
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.list.SetWidth(msg.Width)
		return m, nil
	}

	var cmd tea.Cmd
	if m.selectedTask == nil {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	if m.quitting {
		return ""
	}
	if m.selectedTask != nil {
		return selectedTaskView(m.selectedTask)
	}
	return m.list.View()
}

func selectedTaskView(task *inspector.TaskDefinition) string {
	var s string
	s += fmt.Sprintf("Task: %s\n\n", task.Name)
	s += fmt.Sprintf("Description:\n%s\n\n", task.Description)
	s += fmt.Sprintf("Usage:\n%s\n\n", task.Usage)
	if len(task.Parameters) > 0 {
		s += "Parameters:\n"
		for _, p := range task.Parameters {
			s += fmt.Sprintf("  - %s\n", p.Name)
		}
	}
	s += "\n(Press 'esc' to go back, 'q' to quit)"
	return s
}

// listItem is a wrapper around inspector.TaskDefinition to satisfy the list.Item interface.
type listItem struct {
	inspector.TaskDefinition
}

// Implement list.Item for listItem
func (li listItem) Title() string       { return li.TaskDefinition.Name }
func (li listItem) Description() string { return li.TaskDefinition.Usage }
func (li listItem) FilterValue() string { return li.TaskDefinition.Name }

func NewModel(config *inspector.MCPConfig) model {
	items := make([]list.Item, len(config.Tasks))
	for i, task := range config.Tasks {
		items[i] = listItem{task} // Wrap TaskDefinition in listItem
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Available Tasks"

	return model{list: l, taskConfig: config}
}
