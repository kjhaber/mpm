package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/kjhaber/mpm/internal/crypt"
	"github.com/kjhaber/mpm/internal/vault"
)

var (
	errBadPassword = crypt.ErrBadPassword
	readFile       = os.ReadFile

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	hintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	msgStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
)

func (m *Model) title() string {
	switch m.screen {
	case screenSecrets:
		return "mpm › " + m.repo
	case screenFields:
		return "mpm › " + m.repo + " › " + vault.SafeName(m.secret)
	case screenRepoFields:
		return "mpm › " + m.repo + " › fields"
	}
	return "mpm › repositories"
}

func (m *Model) hints() string {
	switch m.screen {
	case screenSecrets:
		return "enter open · n new · x delete · / filter · esc back · q quit"
	case screenFields:
		return "enter/s show · c copy · e edit · / filter · esc back · q quit"
	case screenRepoFields:
		return "n new · x delete · u/d move · r toggle required · p pattern · esc back · q quit"
	}
	return "enter open · n new · x delete · f fields · / filter · esc quit · q quit"
}

func (m *Model) row(name string) string {
	switch m.screen {
	case screenFields:
		if m.revealed && m.revealName == name {
			return name + ": " + m.handle.Repo.Secrets[m.secret][name]
		}
	case screenRepoFields:
		req, _ := m.handle.Repo.GetAttrib(name, "required")
		pat, _ := m.handle.Repo.GetAttrib(name, "pattern")
		return fmt.Sprintf("%s  (required: %s, pattern: %s)", name, req, pat)
	}
	return vault.SafeName(name)
}

func (m *Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.title()) + "\n\n")

	vis := m.visible()
	if len(vis) == 0 {
		b.WriteString(hintStyle.Render("  (nothing here)") + "\n")
	}
	for i, idx := range vis {
		if i == m.cursor {
			b.WriteString(cursorStyle.Render("> "+m.row(m.items[idx])) + "\n")
		} else {
			b.WriteString("  " + m.row(m.items[idx]) + "\n")
		}
	}
	b.WriteString("\n")

	switch m.mode {
	case modePrompt:
		b.WriteString(m.promptLabel + m.promptInput.View() + "\n")
	case modeConfirm:
		b.WriteString(m.confirmLabel + " (y/N)\n")
	case modeFilter:
		b.WriteString(m.filterInput.View() + "\n")
	default:
		if m.query != "" {
			b.WriteString(hintStyle.Render("filter: "+m.query) + "\n")
		}
	}
	if m.msg != "" {
		b.WriteString(msgStyle.Render(m.msg) + "\n")
	}
	b.WriteString(hintStyle.Render(m.hints()) + "\n")
	return b.String()
}
