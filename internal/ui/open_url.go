package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/browseropen"
)

type BrowserOpenMsg struct {
	URL string
	Err error
}

func OpenURLCmd(rawURL string) tea.Cmd {
	rawURL = strings.TrimSpace(rawURL)
	return func() tea.Msg {
		return BrowserOpenMsg{URL: rawURL, Err: browseropen.Open(rawURL)}
	}
}
