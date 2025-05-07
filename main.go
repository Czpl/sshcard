package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/activeterm"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/charmbracelet/wish/logging"
	"github.com/mitchellh/go-wordwrap"
)

const (
	host = "0.0.0.0"
	port = "23234"
)

func main() {
	go func() {
		fs := http.FileServer(http.Dir("/app/static"))
		http.Handle("/", fs)
		log.Info("Serving static HTML on :80")
		err := http.ListenAndServe(":80", nil)
		if err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()
	s, err := wish.NewServer(
		wish.WithAddress(net.JoinHostPort(host, port)),
		wish.WithHostKeyPath("/data/key"),
		wish.WithMiddleware(
			bubbletea.Middleware(teaHandler),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Error("Could not start server", "error", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	log.Info("Starting SSH server", "host", host, "port", port)
	go func() {
		if err = s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Error("Could not start server", "error", err)
			done <- nil
		}
	}()
	<-done
	log.Info("Stopping SSH server")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer func() { cancel() }()
	if err := s.Shutdown(ctx); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
		log.Error("Could not stop server", "error", err)
	}
}

func teaHandler(s ssh.Session) (tea.Model, []tea.ProgramOption) {
	pty, _, _ := s.Pty()

	renderer := bubbletea.MakeRenderer(s)
	txtStyle := renderer.NewStyle().Foreground(lipgloss.Color("10"))
	quitStyle := renderer.NewStyle().Foreground(lipgloss.Color("8"))
	quitStyleDark := renderer.NewStyle().Foreground(lipgloss.Color("238"))

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	m := model{
		width:         pty.Window.Width,
		height:        pty.Window.Height,
		txtStyle:      txtStyle,
		quitStyle:     quitStyle,
		quitStyleDark: quitStyleDark,
		spinner:       spin,
		options:       []string{"info", "contact"},
		selected:      make(map[int]struct{}),
	}
	return m, []tea.ProgramOption{tea.WithAltScreen()}
}

type model struct {
	spinner       spinner.Model
	width         int
	height        int
	txtStyle      lipgloss.Style
	quitStyle     lipgloss.Style
	quitStyleDark lipgloss.Style
	options       []string
	cursor        int
	selected      map[int]struct{}
}

func (m model) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.options)-1 {
				m.cursor++
			}
		case "enter", " ":
			_, ok := m.selected[m.cursor]
			if ok {
				delete(m.selected, m.cursor)
			} else {
				m.selected[m.cursor] = struct{}{}
			}
		}
	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m model) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(1, 2)

	var s string

	for i, choice := range m.options {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}

		checked := " "
		details := ""
		if _, ok := m.selected[i]; ok {
			checked = "x"
			if m.options[i] == "info" {
				details = `I'm a senior software engineer who loves to tinker with code across the board. Most of my time goes into building stuff with TypeScript, React, and Next.js, but I also dive into PHP for WordPress plugins when needed.On the side, I mess around with C++ and Go just for fun—keeps things interesting and keeps me learning.`
			}
			if m.options[i] == "contact" {
				details += `cmateusz@protonmail.com`
			}
		}
		s += fmt.Sprintf("%s [%s] %s\n", cursor, checked, choice)
		if details != "" {
			wrapped := wordwrap.WrapString(details, uint(m.width-10))
			s += fmt.Sprintf("\n%s\n\n", wrapped)
		}
	}
	helpMsg := m.quitStyle.Render("j") + m.quitStyleDark.Render(" down · ")
	helpMsg += m.quitStyle.Render("k") + m.quitStyleDark.Render(" up · ")
	helpMsg += m.quitStyle.Render("spc") + m.quitStyleDark.Render(" select · ")
	helpMsg += m.quitStyle.Render("q") + m.quitStyleDark.Render(" quit ")
	content := fmt.Sprintf("\n%s czpl.dev WIP \n\n", m.spinner.View()) + m.txtStyle.Render(s) + "\n\n" + helpMsg

	boxWidth := lipgloss.Width(content) + 4
	boxHeight := lipgloss.Height(content) + 2
	xOffset := (m.width - boxWidth) / 2
	yOffset := (m.height - boxHeight) / 2

	return lipgloss.NewStyle().
		Margin(yOffset, xOffset).
		Render(boxStyle.Render(content))
}
