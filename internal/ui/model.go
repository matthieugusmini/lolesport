package ui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matthieugusmini/go-lolesports"

	"github.com/matthieugusmini/rift/internal/rift"
)

const (
	logo = "Rift"

	navItemLabelSchedule  = "Schedule"
	navItemLabelStandings = "Standings"

	navigationBarHeight = 2

	maxWidth = 120
)

var navItemLabels = []string{
	navItemLabelSchedule,
	navItemLabelStandings,
}

type state int

const (
	stateShowSchedule state = iota
	stateShowStandings
)

var stateByNavItemLabel = map[string]state{
	navItemLabelSchedule:  stateShowSchedule,
	navItemLabelStandings: stateShowStandings,
}

type LoLEsportsLoader interface {
	GetSchedule(
		ctx context.Context,
		opts *lolesports.GetScheduleOptions,
	) (lolesports.Schedule, error)
	LoadStandingsByTournamentIDs(
		ctx context.Context,
		tournamentIDs []string,
	) ([]lolesports.Standings, error)
	GetCurrentSeasonSplits(ctx context.Context) ([]lolesports.Split, error)
}

type BracketTemplateLoader interface {
	ListAvailableStageIDs(ctx context.Context) ([]string, error)
	Load(ctx context.Context, stageID string) (rift.BracketTemplate, error)
}

type modelStyles struct {
	logo            lipgloss.Style
	normalNavItem   lipgloss.Style
	selectedNavItem lipgloss.Style
	separator       lipgloss.Style
}

func newDefaultModelStyles() (s modelStyles) {
	s.logo = lipgloss.NewStyle().
		Padding(0, 1).
		Foreground(textTitleColor).
		Bold(true)

	s.normalNavItem = lipgloss.NewStyle().
		Foreground(textPrimaryColor).
		Faint(true)

	s.selectedNavItem = lipgloss.NewStyle().
		Foreground(selectedColor).
		Bold(true)

	s.separator = lipgloss.NewStyle().
		Foreground(textSecondaryColor).
		Bold(true)

	return s
}

type Model struct {
	selectedNavIndex int

	schedulePage  *schedulePage
	standingsPage *standingsPage

	state state

	width, height int
	pageWidth     int

	styles modelStyles
}

func NewModel(
	lolesportsClient LoLEsportsLoader,
	bracketLoader BracketTemplateLoader,
	logger *slog.Logger,
) Model {
	return Model{
		schedulePage:  newSchedulePage(lolesportsClient, logger),
		standingsPage: newStandingsPage(lolesportsClient, bracketLoader, logger),
		styles:        newDefaultModelStyles(),
	}
}

func (m Model) Init() tea.Cmd {
	return m.schedulePage.Init()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "tab":
			m.selectedNavIndex = moveNavigationBarCursorRight(m.selectedNavIndex)
			return m.navigate()

		case "shift+tab":
			m.selectedNavIndex = moveNavigationBarCursorLeft(m.selectedNavIndex)
			return m.navigate()
		}

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.pageWidth = min(msg.Width, maxWidth)
		m.schedulePage.setSize(m.pageWidth, msg.Height-navigationBarHeight)
		m.standingsPage.setSize(m.pageWidth, msg.Height-navigationBarHeight)
	}

	return m.updateCurrentPage(msg)
}

func (m Model) View() string {
	navBar := m.viewNavigationBar(navItemLabels, m.selectedNavIndex, m.pageWidth)

	content := m.currentPageView()

	view := lipgloss.JoinVertical(lipgloss.Left, navBar, content)

	return lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center).
		Render(view)
}

func (m Model) viewNavigationBar(
	navLabels []string,
	selectedNavIndex int,
	width int,
) string {
	logo := m.styles.logo.Render(logo)

	styledNavItems := make([]string, len(navLabels))
	for i, label := range navLabels {
		isSelected := selectedNavIndex == i
		if isSelected {
			styledNavItems[i] = m.styles.selectedNavItem.Render(label)
		} else {
			styledNavItems[i] = m.styles.normalNavItem.Render(label)
		}
	}

	padding := strings.Repeat(" ", lipgloss.Width(logo))

	navItemsStyle := lipgloss.NewStyle().
		Width(width - lipgloss.Width(logo)*2).
		Align(lipgloss.Center)
	navItems := navItemsStyle.Render(strings.Join(styledNavItems, separatorBullet))

	navbar := logo + navItems + padding

	separator := m.styles.separator.Render(strings.Repeat(separatorLine, width))

	return fmt.Sprintf("%s\n%s", navbar, separator)
}

func (m Model) currentPageView() string {
	switch m.state {
	case stateShowSchedule:
		return m.schedulePage.View()
	case stateShowStandings:
		return m.standingsPage.View()
	default:
		return ""
	}
}

func (m Model) updateCurrentPage(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case stateShowSchedule:
		m.schedulePage, cmd = m.schedulePage.Update(msg)
	case stateShowStandings:
		m.standingsPage, cmd = m.standingsPage.Update(msg)
	}
	return m, cmd
}

func (m Model) navigate() (Model, tea.Cmd) {
	m.state = stateByNavItemLabel[navItemLabels[m.selectedNavIndex]]
	switch m.state {
	case stateShowSchedule:
		return m, m.schedulePage.Init()
	case stateShowStandings:
		return m, m.standingsPage.Init()
	default:
		return m, nil
	}
}

func moveNavigationBarCursorLeft(current int) int {
	return moveCursor(current, -1, len(navItemLabels))
}

func moveNavigationBarCursorRight(current int) int {
	return moveCursor(current, 1, len(navItemLabels))
}

func moveCursor(current, delta, upperBound int) int {
	if upperBound == 0 {
		return 0
	}
	return (current + delta + upperBound) % upperBound
}
