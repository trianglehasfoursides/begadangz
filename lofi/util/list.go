package util

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var style = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7571F9")).
	Padding(1).
	MarginLeft(2).
	Width(40).
	MaxHeight(20).
	Height(18)

type Item struct {
	Name    string
	Content string
}

func (i Item) Title() string       { return i.Name }
func (i Item) Description() string { return i.Content }
func (i Item) FilterValue() string { return i.Name }

type model struct {
	list list.Model
}

func newModel(initialItems []Item) model {
	items := make([]list.Item, len(initialItems))
	for i, it := range initialItems {
		items[i] = it
	}

	delegate := list.NewDefaultDelegate()
	groceryList := list.New(items, delegate, 0, 0)
	groceryList.Title = "Items"
	groceryList.Styles.Title = titleStyle

	return model{
		list: groceryList,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := appStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	newListModel, cmd := m.list.Update(msg)
	m.list = newListModel

	return m, cmd
}

func (m model) View() string {
	left := appStyle.Render(m.list.View())

	var selected Item
	if i, ok := m.list.SelectedItem().(Item); ok {
		selected = i
	}

	right := style.Render(fmt.Sprintf(
		"%s\n\n%s",
		selected.Name,
		selected.Content,
	))

	row := lipgloss.JoinHorizontal(lipgloss.Center, left, right)

	return lipgloss.Place(
		m.list.Width(), m.list.Height(),
		lipgloss.Center,
		lipgloss.Center,
		row,
		lipgloss.WithWhitespaceChars(" "),
	)
}

func List(items []Item) {
	if _, err := tea.NewProgram(newModel(items), tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

var (
	appStyle = lipgloss.NewStyle().Margin(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	styleList = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Padding(1, 2)
)
