package main

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

type Model struct {
	content string
	edit    bool // Flag to trigger edit on exit
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "e":
			// Signal that we should edit after exit.
			m.edit = true
			return m, tea.Quit

		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m Model) View() string {
	return fmt.Sprintf(
		"Content:\n%s\n\n[e] Edit [q] Quit\n",
		m.content,
	)
}

func main() {
	m := Model{}

	for {
		p := tea.NewProgram(m)
		mod, err := p.Run()
		if err != nil {
			fmt.Println("Error:", err)
			os.Exit(1)
		}

		m = mod.(Model)

		if !m.edit {
			break // We’re done.
		}

		// Run the editor outside the TUI.
		edited, err := runEditor(m.content)
		if err != nil {
			fmt.Println("Editor error:", err)
			break
		}
		m.content = edited
		m.edit = false
	}

	fmt.Println("Goodbye!")
}

func runEditor(initial string) (string, error) {
	tmpfile, err := os.CreateTemp("", "edit-*.txt")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.WriteString(initial)
	if err != nil {
		return "", err
	}
	tmpfile.Close()

	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}

	cmd := exec.Command(editor, tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Run()
	if err != nil {
		return "", err
	}

	edited, err := os.ReadFile(tmpfile.Name())
	if err != nil {
		return "", err
	}

	return string(edited), nil
}
