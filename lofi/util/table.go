package util

import (
	"fmt"
	"log"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	green = lipgloss.AdaptiveColor{Light: "#02BA84", Dark: "#02BF87"}
)

// Styling
var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("#7571F9")).
	Width(58).
	Height(18).
	MaxHeight(20)

// Msg type (kalau nanti mau dipakai Cmd)
type haloMsg string

// Komponen utama
type Table struct {
	table  table.Model
	width  int
	height int
	halo   string
}

func (t Table) Init() tea.Cmd { return nil }

func (t Table) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	var render = func() {
		styleLabel := lipgloss.NewStyle().Foreground(lipgloss.Color(green.Dark))

		formatEnter := func(label, value string) string {
			return fmt.Sprintf("\n%s\n%s\n", styleLabel.Render(label), value)
		}

		formatSpace := func(label, value string) string {
			return fmt.Sprintf("%s : %s\n", styleLabel.Render(label), value)
		}

		row := t.table.SelectedRow()

		content := row[4]
		out, err := glamour.Render(content, "dark")
		if err != nil {
			out = row[4]
		}

		id := formatSpace("Id", row[0])
		from := formatSpace("From", row[1])
		to := formatSpace("To", row[2])
		subject := formatSpace("Subject", row[3])
		attachment := formatSpace("Attachment", row[5])
		mail := formatEnter("Mail", out)

		t.halo = id + from + to + subject + attachment + mail
	}
	render()

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if t.table.Focused() {
				t.table.Blur()
			} else {
				t.table.Focus()
			}
		case "q", "ctrl+c", "ctrl+d":
			return t, tea.Quit
			// case "enter":
			// 	t.halo = string(t.table.SelectedRow()[4])
		case "up", "down":
			// Update posisi row table
			t.table, cmd = t.table.Update(msg)
			render()
		}
	case tea.WindowSizeMsg:
		t.width = msg.Width
		t.height = msg.Height
	}

	t.table, cmd = t.table.Update(msg)
	return t, cmd
}

func (t Table) View() string {
	left := baseStyle.Render(t.table.View())
	right := style.Render(t.halo)

	row := lipgloss.JoinHorizontal(lipgloss.Center, left, right)

	return lipgloss.Place(
		t.width, t.height,
		lipgloss.Center,
		lipgloss.Center,
		row,
		lipgloss.WithWhitespaceChars(" "),
	)
}

// Fungsi yang bisa dipanggil untuk menjalankan TUI
func RenderTable(rows ...table.Row) {
	columns := []table.Column{
		{Title: "Id", Width: 4},
		{Title: "From", Width: 6},
		{Title: "To", Width: 6},
		{Title: "Subject", Width: 10},
		{Title: "Mail", Width: 10},
		{Title: "Attachment", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(10),
	)

	// Styling table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#7571F9")).
		BorderBottom(true).
		Bold(false)

	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)

	t.SetStyles(s)

	// Jalankan Bubble Tea dengan alt screen (fullscreen)
	m := Table{table: t}
	if _, err := tea.NewProgram(m, tea.WithAltScreen()).Run(); err != nil {
		log.Fatal(err)
	}
}
