package ui

import (
	"context"
	"log/slog"
	"slices"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/matthieugusmini/go-lolesports"

	"github.com/matthieugusmini/rift/internal/rift"
)

const (
	selectionListCount       = 3
	minListHeight            = 18
	minSelectionPromptHeight = 3

	standingsPageShortHelpHeight = 1
	standingsPageFullHelpHeight  = 6
)

const (
	errMessageFetchError = "Oups! Something went wrong...\nPress any key to try your luck again."
)

const (
	captionSelectSplit  = "SELECT A SPLIT"
	captionSelectLeague = "SELECT A LEAGUE"
	captionSelectStage  = "SELECT A STAGE"
)

type standingsPageState int

const (
	standingsPageStateLoadingSplits standingsPageState = iota
	standingsPageStateSplitSelection
	standingsPageStateLeagueSelection
	standingsPageStateLoadingStages
	standingsPageStateStageSelection
	standingsPageStateLoadingBracketTemplate
	standingsPageStateShowRankingPage
	standingsPageStateShowBracketPage
)

type standingsStyles struct {
	doc     lipgloss.Style
	prompt  lipgloss.Style
	spinner lipgloss.Style
	error   lipgloss.Style
	help    lipgloss.Style
}

func newDefaultStandingsStyles() (s standingsStyles) {
	s.doc = lipgloss.NewStyle().Padding(1, 2)

	s.prompt = lipgloss.NewStyle().
		Foreground(textPrimaryColor).
		Bold(true)

	s.help = lipgloss.NewStyle().Padding(1, 0, 0, 2)

	s.spinner = lipgloss.NewStyle().Foreground(spinnerColor)

	s.error = lipgloss.NewStyle().
		Align(lipgloss.Center).
		Foreground(textPrimaryColor).
		Italic(true)

	return s
}

type standingsPageKeyMap struct {
	baseKeyMap

	Select   key.Binding
	Previous key.Binding
	Up       key.Binding
	Down     key.Binding
}

func newDefaultStandingsPageKeyMap() standingsPageKeyMap {
	return standingsPageKeyMap{
		baseKeyMap: newBaseKeyMap(),
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter", "right"),
			key.WithHelp("enter/→", "select"),
		),
		Previous: key.NewBinding(
			key.WithKeys("esc", "left"),
			key.WithHelp("esc/←", "previous"),
		),
	}
}

type standingsPage struct {
	lolesportsClient      LoLEsportsLoader
	bracketTemplateLoader BracketTemplateLoader
	logger                *slog.Logger

	state standingsPageState

	splits  []lolesports.Split
	leagues []lolesports.League
	stages  []lolesports.Stage

	splitOptions  list.Model
	leagueOptions list.Model
	stageOptions  list.Model

	rankingView *rankingPage
	bracket     *bracketPage

	errMsg string

	spinner spinner.Model

	keyMap standingsPageKeyMap
	help   help.Model

	height, width int

	styles standingsStyles
}

func newStandingsPage(
	lolesportsClient LoLEsportsLoader,
	bracketLoader BracketTemplateLoader,
	logger *slog.Logger,
) *standingsPage {
	styles := newDefaultStandingsStyles()

	sp := spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(styles.spinner),
	)

	return &standingsPage{
		lolesportsClient:      lolesportsClient,
		bracketTemplateLoader: bracketLoader,
		logger:                logger,
		styles:                styles,
		spinner:               sp,
		keyMap:                newDefaultStandingsPageKeyMap(),
		help:                  help.New(),
	}
}

func (p *standingsPage) Init() tea.Cmd {
	if p.state != standingsPageStateLoadingSplits {
		return nil
	}
	return tea.Batch(p.spinner.Tick, p.fetchCurrentSeasonSplits())
}

func (p *standingsPage) Update(msg tea.Msg) (*standingsPage, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// When an error is displayed after failing to fetch the initial schedule data,
		// any keypress should trigger a refetch of the initial data again.
		if p.errMsg != "" {
			p.errMsg = ""
			if p.state == standingsPageStateLoadingSplits {
				return p, tea.Batch(p.fetchCurrentSeasonSplits())
			}
		}

		switch {
		case key.Matches(msg, p.keyMap.Quit):
			return p, tea.Quit

		case key.Matches(msg, p.keyMap.ShowFullHelp),
			key.Matches(msg, p.keyMap.CloseFullHelp):
			p.toggleFullHelp()

		case key.Matches(msg, p.keyMap.Previous):
			if !p.isShowingSubModel() || (p.isShowingSubModel() && p.isSubModelPreviousKey(msg)) {
				p.goToPreviousStep()
			}

		case key.Matches(msg, p.keyMap.Select):
			cmds = append(cmds, p.handleSelection())
		}

	case spinner.TickMsg:
		if p.isLoading() {
			var cmd tea.Cmd
			p.spinner, cmd = p.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case fetchedCurrentSeasonSplitsMessage:
		p.handleSplitsLoaded(msg)

	case loadedStandingsMessage:
		p.handleStandingsLoaded(msg)

	case loadedBracketStageTemplateMessage:
		p.handleBracketTemplateLoaded(msg)

	case fetchErrorMessage:
		p.handleErrorMessage(msg)
	}

	cmd := p.updateActiveModel(msg)
	cmds = append(cmds, cmd)

	return p, tea.Batch(cmds...)
}

func (p *standingsPage) updateActiveModel(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch p.state {
	case standingsPageStateSplitSelection:
		p.splitOptions, cmd = p.splitOptions.Update(msg)
	case standingsPageStateLeagueSelection:
		p.leagueOptions, cmd = p.leagueOptions.Update(msg)
	case standingsPageStateStageSelection:
		p.stageOptions, cmd = p.stageOptions.Update(msg)
	case standingsPageStateShowRankingPage:
		p.rankingView, cmd = p.rankingView.Update(msg)
	case standingsPageStateShowBracketPage:
		p.bracket, cmd = p.bracket.Update(msg)
	}

	return cmd
}

func (p *standingsPage) handleSplitsLoaded(msg fetchedCurrentSeasonSplitsMessage) {
	p.state = standingsPageStateSplitSelection

	p.splits = msg.splits
	p.splitOptions = newSplitOptionsList(p.splits, p.listWidth(), p.listHeight())
}

func (p *standingsPage) handleStandingsLoaded(msg loadedStandingsMessage) {
	p.state = standingsPageStateStageSelection

	p.stages = listStagesFromStandings(msg.standings)
	p.stageOptions = newStageOptionsList(p.stages, p.listWidth(), p.listHeight())
}

func (p *standingsPage) handleBracketTemplateLoaded(msg loadedBracketStageTemplateMessage) {
	p.state = standingsPageStateShowBracketPage

	// Bracket stages always have a single section.
	matches := p.selectedStage().Sections[0].Matches
	p.bracket = newBracketPage(msg.template, matches, p.width, p.height)
}

func (p *standingsPage) View() string {
	if p.width <= 0 {
		return ""
	}

	if p.errMsg != "" {
		return p.viewError()
	}

	var sections []string

	switch p.state {
	case standingsPageStateSplitSelection,
		standingsPageStateLeagueSelection,
		standingsPageStateStageSelection,
		standingsPageStateLoadingSplits,
		standingsPageStateLoadingStages:
		sections = append(sections, p.viewSelection())
		showPrompt := p.contentHeight() >= minListHeight+minSelectionPromptHeight
		if showPrompt {
			sections = append(sections, p.viewSelectionPrompt())
		}
		sections = append(sections, p.viewHelp())

	case standingsPageStateShowBracketPage:
		sections = append(sections, p.bracket.View())

	case standingsPageStateShowRankingPage:
		sections = append(sections, p.rankingView.View())
	}

	view := lipgloss.JoinVertical(lipgloss.Left, sections...)

	return p.styles.doc.Render(view)
}

func (p *standingsPage) viewError() string {
	errMsg := p.styles.error.Render(p.errMsg)
	return p.styles.doc.
		Width(p.width).
		Height(p.contentHeight()).
		Align(lipgloss.Center, lipgloss.Center).
		Render(errMsg)
}

func (p *standingsPage) viewSelection() string {
	listStyle := lipgloss.NewStyle().
		Width(p.listWidth()).
		Height(p.listHeight()).
		Align(lipgloss.Center)

	var (
		splitOptionsView  string
		leagueOptionsView string
		stageOptionsView  string
	)
	switch p.state {
	case standingsPageStateLoadingSplits:
		splitOptionsView = listStyle.Render(p.spinner.View())

	case standingsPageStateSplitSelection:
		splitOptionsView = listStyle.Render(p.splitOptions.View())

	case standingsPageStateLeagueSelection:
		splitOptionsView = listStyle.Render(p.splitOptions.View())
		leagueOptionsView = listStyle.Render(p.leagueOptions.View())

	case standingsPageStateLoadingStages:
		splitOptionsView = listStyle.Render(p.splitOptions.View())
		leagueOptionsView = listStyle.Render(p.leagueOptions.View())
		stageOptionsView = listStyle.Render(p.spinner.View())

	case standingsPageStateStageSelection:
		splitOptionsView = listStyle.Render(p.splitOptions.View())
		leagueOptionsView = listStyle.Render(p.leagueOptions.View())
		stageOptionsView = listStyle.Render(p.stageOptions.View())
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		splitOptionsView,
		leagueOptionsView,
		stageOptionsView,
	)
}

func (p *standingsPage) viewSelectionPrompt() string {
	promptHeight := p.contentHeight() - p.listHeight()

	var prompt string

	switch p.state {
	case standingsPageStateSplitSelection:
		prompt = p.styles.prompt.Render(captionSelectSplit)
	case standingsPageStateLeagueSelection:
		prompt = p.styles.prompt.Render(captionSelectLeague)
	case standingsPageStateStageSelection:
		prompt = p.styles.prompt.Render(captionSelectStage)
	}

	return lipgloss.Place(
		p.width,
		promptHeight,
		lipgloss.Center,
		lipgloss.Center,
		prompt,
	)
}

func (p *standingsPage) viewHelp() string {
	return p.styles.help.Render(p.help.View(p))
}

func (p *standingsPage) setSize(width, height int) {
	h, v := p.styles.doc.GetFrameSize()
	p.width, p.height = width-h, height-v

	p.help.Width = p.width

	switch p.state {
	case standingsPageStateSplitSelection:
		p.splitOptions.SetSize(p.listSize())

	case standingsPageStateLeagueSelection:
		listWidth, listHeight := p.listSize()
		p.splitOptions.SetSize(listWidth, listHeight)
		p.leagueOptions.SetSize(listWidth, listHeight)

	case standingsPageStateStageSelection:
		listWidth, listHeight := p.listSize()
		p.splitOptions.SetSize(listWidth, listHeight)
		p.leagueOptions.SetSize(listWidth, listHeight)
		p.stageOptions.SetSize(listWidth, listHeight)

		// Give full height to Sub-models.
	case standingsPageStateShowRankingPage:
		p.rankingView.setSize(p.width, p.height)

	case standingsPageStateShowBracketPage:
		p.bracket.setSize(p.width, p.height)
	}
}

func (p *standingsPage) isLoading() bool {
	return p.state == standingsPageStateLoadingSplits ||
		p.state == standingsPageStateLoadingStages ||
		p.state == standingsPageStateLoadingBracketTemplate
}

func (p *standingsPage) handleErrorMessage(msg fetchErrorMessage) {
	p.errMsg = errMessageFetchError

	switch p.state {
	case standingsPageStateLoadingStages:
		p.state = standingsPageStateLeagueSelection

	case standingsPageStateLoadingBracketTemplate:
		p.state = standingsPageStateStageSelection
	}

	p.logger.Error("Failed to fetch standings", slog.Any("error", msg.err))
}

func (p *standingsPage) handleSelection() tea.Cmd {
	var cmd tea.Cmd

	switch p.state {
	case standingsPageStateSplitSelection:
		p.selectSplit()
	case standingsPageStateLeagueSelection:
		cmd = p.selectLeague()
	case standingsPageStateStageSelection:
		cmd = p.selectStage()
	}

	return cmd
}

func (p *standingsPage) selectSplit() {
	p.state = standingsPageStateLeagueSelection

	p.leagues = listLeaguesFromTournaments(p.selectedSplit().Tournaments)
	p.leagueOptions = newLeagueOptionsList(p.leagues, p.listWidth(), p.listHeight())
}

func (p *standingsPage) selectLeague() tea.Cmd {
	p.state = standingsPageStateLoadingStages

	tournamentIDs := listTournamentIDsForLeague(
		p.selectedSplit().Tournaments,
		p.selectedLeague().ID,
	)

	return tea.Batch(p.spinner.Tick, p.loadStandings(tournamentIDs))
}

func (p *standingsPage) selectStage() tea.Cmd {
	stageType := getStageType(p.selectedStage())
	switch stageType {
	case stageTypeGroups:
		p.rankingView = newRankingPage(
			p.selectedSplit(),
			p.selectedLeague(),
			p.selectedStage(),
			p.width,
			p.height,
		)
		p.state = standingsPageStateShowRankingPage

	case stageTypeBracket:
		p.state = standingsPageStateLoadingBracketTemplate
		return p.loadBracketStageTemplate(p.selectedStage().ID)
	}

	return nil
}

func (p *standingsPage) goToPreviousStep() {
	switch p.state {
	case standingsPageStateLeagueSelection:
		p.state = standingsPageStateSplitSelection
		p.leagueOptions = list.Model{}

	case standingsPageStateStageSelection:
		p.state = standingsPageStateLeagueSelection
		p.stageOptions = list.Model{}

	case standingsPageStateShowRankingPage, standingsPageStateShowBracketPage:
		p.state = standingsPageStateStageSelection
	}
}

func (p *standingsPage) isShowingSubModel() bool {
	return p.state == standingsPageStateShowRankingPage ||
		p.state == standingsPageStateShowBracketPage
}

func (p *standingsPage) isSubModelPreviousKey(k tea.KeyMsg) bool {
	switch p.state {
	case standingsPageStateShowRankingPage:
		return key.Matches(k, p.rankingView.keyMap.Previous)
	case standingsPageStateShowBracketPage:
		return key.Matches(k, p.bracket.keyMap.Previous)
	}
	return false
}

func (p *standingsPage) updateContentViewHeight() {
	listHeight := p.listHeight()

	switch p.state {
	case standingsPageStateSplitSelection:
		p.splitOptions.SetHeight(listHeight)

	case standingsPageStateLeagueSelection:
		p.splitOptions.SetHeight(listHeight)
		p.leagueOptions.SetHeight(listHeight)

	case standingsPageStateStageSelection:
		p.splitOptions.SetHeight(listHeight)
		p.leagueOptions.SetHeight(listHeight)
		p.stageOptions.SetHeight(listHeight)
	}
}

func (p *standingsPage) contentHeight() int {
	return p.height - p.helpHeight()
}

func (p *standingsPage) listSize() (width, height int) {
	return p.listWidth(), p.listHeight()
}

func (p *standingsPage) listWidth() int {
	return p.width / selectionListCount
}

func (p *standingsPage) listHeight() int {
	showPrompt := p.contentHeight() >= minListHeight+minSelectionPromptHeight
	if showPrompt {
		return max(p.contentHeight()/2, minListHeight)
	} else {
		return p.contentHeight()
	}
}

func (p *standingsPage) ShortHelp() []key.Binding {
	return []key.Binding{
		p.keyMap.Select,
		p.keyMap.NextPage,
		p.keyMap.Quit,
		p.keyMap.ShowFullHelp,
	}
}

func (p *standingsPage) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Motions
		{
			p.keyMap.Up,
			p.keyMap.Down,
			p.keyMap.Select,
			p.keyMap.Previous,
		},
		// App navigation
		{
			p.keyMap.NextPage,
			p.keyMap.PrevPage,
		},
		// Others
		{
			p.keyMap.Quit,
			p.keyMap.CloseFullHelp,
		},
	}
}

func (p *standingsPage) toggleFullHelp() {
	// Sub-models displays their own help.
	if p.isShowingSubModel() {
		return
	}

	p.help.ShowAll = !p.help.ShowAll
	p.updateContentViewHeight()
}

func (p *standingsPage) helpHeight() int {
	padding := p.styles.help.GetVerticalPadding()
	if p.help.ShowAll {
		return standingsPageFullHelpHeight + padding
	}
	return standingsPageShortHelpHeight + padding
}

func (p *standingsPage) selectedSplit() lolesports.Split { return p.splits[p.splitOptions.Index()] }

func (p *standingsPage) selectedLeague() lolesports.League { return p.leagues[p.leagueOptions.Index()] }

func (p *standingsPage) selectedStage() lolesports.Stage { return p.stages[p.stageOptions.Index()] }

// Msgs

type (
	fetchedCurrentSeasonSplitsMessage struct{ splits []lolesports.Split }
	loadedBracketStageTemplateMessage struct{ template rift.BracketTemplate }
	loadedStandingsMessage            struct{ standings []lolesports.Standings }
	fetchErrorMessage                 struct{ err error }
)

// Cmds

func (p *standingsPage) loadStandings(tournamentIDs []string) tea.Cmd {
	return func() tea.Msg {
		standings, err := p.lolesportsClient.LoadStandingsByTournamentIDs(
			context.Background(),
			tournamentIDs,
		)
		if err != nil {
			return fetchErrorMessage{err: err}
		}
		return loadedStandingsMessage{standings}
	}
}

func (p *standingsPage) fetchCurrentSeasonSplits() tea.Cmd {
	return func() tea.Msg {
		splits, err := p.lolesportsClient.GetCurrentSeasonSplits(context.Background())
		if err != nil {
			return fetchErrorMessage{err: err}
		}
		return fetchedCurrentSeasonSplitsMessage{splits}
	}
}

func (p *standingsPage) isBracketStageTemplateAvailable(stageID string) bool {
	availableStageIDs, err := p.bracketTemplateLoader.ListAvailableStageIDs(context.Background())
	if err != nil {
		// Ignore err for now
		return false
	}

	return slices.Contains(availableStageIDs, stageID)
}

func (p *standingsPage) loadBracketStageTemplate(stageID string) tea.Cmd {
	return func() tea.Msg {
		tmpl, err := p.bracketTemplateLoader.Load(context.Background(), stageID)
		if err != nil {
			return fetchErrorMessage{err: err}
		}
		return loadedBracketStageTemplateMessage{tmpl}
	}
}

func listLeaguesFromTournaments(tournaments []lolesports.Tournament) []lolesports.League {
	var (
		leagues     []lolesports.League
		seenLeagues = map[string]bool{}
	)
	for _, tournament := range tournaments {
		if _, ok := seenLeagues[tournament.League.ID]; !ok {
			leagues = append(leagues, tournament.League)
			seenLeagues[tournament.League.ID] = true
		}
	}
	return leagues
}

func listTournamentIDsForLeague(tournaments []lolesports.Tournament, leagueID string) []string {
	var tournamentIDs []string
	for _, tournament := range tournaments {
		if tournament.League.ID == leagueID {
			tournamentIDs = append(tournamentIDs, tournament.ID)
		}
	}
	return tournamentIDs
}

func listStagesFromStandings(standings []lolesports.Standings) []lolesports.Stage {
	var stages []lolesports.Stage
	for _, standing := range standings {
		stages = append(stages, standing.Stages...)
	}
	return stages
}
