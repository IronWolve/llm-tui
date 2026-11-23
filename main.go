// Renderer and key handling adapted from the approved vLLM launcher TUI.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type appInfo struct {
	Brand    string `json:"brand"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Endpoint string `json:"endpoint"`
	State    string `json:"state"`
	Online   bool   `json:"online"`
	Loaded   string `json:"loaded"`
}

type profile struct {
	ID              int          `json:"id"`
	Name            string       `json:"name"`
	ShortName       string       `json:"short_name"`
	Variant         string       `json:"variant"`
	Badge           string       `json:"badge"`
	ServedID        string       `json:"served_id"`
	Recommended     bool         `json:"recommended"`
	Group           string       `json:"group"`
	GroupSuffix     *string      `json:"group_suffix"`
	Missing         []string     `json:"missing"`
	Details         [][]string   `json:"details"`
	Notes           []string     `json:"notes"`
	CompactSections [][][]string `json:"compact_sections"`
}

type catalog struct {
	App      appInfo   `json:"app"`
	Profiles []profile `json:"profiles"`
}

const (
	version         = "1.0.1"
	pickerColumns   = 34
	detailColumns   = 45
	minSplitPicker  = 24
	minSplitDetails = 34
	pickerRowGutter = 5 // Selection marker, ID chip, and gap.
)

type statusMsg struct {
	Online bool
	Model  string
}

type model struct {
	catalog     catalog
	catalogPath string
	cursor      int
	width       int
	height      int
	status      statusMsg
	refreshing  bool
	confirming  bool
	launch      bool
	digitInput  string
	lastDigit   time.Time
	detailPage  int
	proxyOnly   bool
	err         error
}

var (
	renderer  = lipgloss.NewRenderer(os.Stderr)
	ink       = lipgloss.AdaptiveColor{Light: "#17212B", Dark: "#E6EDF3"}
	muted     = lipgloss.AdaptiveColor{Light: "#62707E", Dark: "#8192A3"}
	cyan      = lipgloss.Color("#39D0D8")
	green     = lipgloss.Color("#6EE7A8")
	yellow    = lipgloss.Color("#F4C95D")
	magenta   = lipgloss.Color("#D68CFF")
	blue      = lipgloss.Color("#7AA2F7")
	red       = lipgloss.Color("#FF6B81")
	orange    = lipgloss.Color("#FF9E64")
	dark      = lipgloss.Color("#0B1118")
	selection = lipgloss.AdaptiveColor{Light: "#D7EAF0", Dark: "#24495A"}
	activeInk = lipgloss.AdaptiveColor{Light: "#17212B", Dark: "#F8FAFC"}
	border    = lipgloss.RoundedBorder()
	brand     = renderer.NewStyle().Bold(true).Foreground(dark).Background(cyan).Padding(0, 1)
	title     = renderer.NewStyle().Bold(true).Foreground(activeInk)
	subtitle  = renderer.NewStyle().Foreground(muted)
	panel     = renderer.NewStyle().Border(border).Padding(0, 1)
	keyStyle  = renderer.NewStyle().Bold(true).Foreground(cyan)
	valueText = renderer.NewStyle().Foreground(ink)
)

func loadCatalog(path string) (catalog, error) {
	var c catalog
	data, err := os.ReadFile(path)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")), &c); err != nil {
		return c, err
	}
	if len(c.Profiles) == 0 {
		return c, fmt.Errorf("profile catalog is empty")
	}
	seen := map[int]struct{}{}
	for _, p := range c.Profiles {
		if p.ID < 1 || p.ID > 99 {
			return c, fmt.Errorf("profile id must be in 1-99: %d", p.ID)
		}
		if _, ok := seen[p.ID]; ok {
			return c, fmt.Errorf("duplicate profile id: %d", p.ID)
		}
		seen[p.ID] = struct{}{}
	}
	return c, nil
}

type catalogMsg struct {
	catalog catalog
	err     error
}

func refreshCatalog(path string) tea.Cmd {
	return func() tea.Msg {
		c, err := loadCatalog(path)
		return catalogMsg{c, err}
	}
}

func initialModel(c catalog) model {
	cursor := 0
	for i, p := range c.Profiles {
		if p.Recommended {
			cursor = i
			break
		}
	}
	status := statusMsg{Online: c.App.Online, Model: c.App.Loaded}
	if status.Online && status.Model != "" {
		for i, p := range c.Profiles {
			if p.ServedID == status.Model {
				cursor = i
				break
			}
		}
	}
	return model{catalog: c, cursor: cursor, status: status}
}

func (m model) Init() tea.Cmd { return nil }

func indexOfID(profiles []profile, id int) int {
	for i, p := range profiles {
		if p.ID == id {
			return i
		}
	}
	return -1
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		if m.width > 0 && m.height > 0 && m.detailPage > 0 {
			m.detailPage = min(m.detailPage, m.detailLayout().pageCount())
		}
	case catalogMsg:
		m.refreshing = false
		if msg.err != nil {
			m.err = msg.err
			break
		}
		id := m.catalog.Profiles[m.cursor].ID
		m.catalog = msg.catalog
		m.status = statusMsg{Online: msg.catalog.App.Online, Model: msg.catalog.App.Loaded}
		m.cursor = max(0, indexOfID(m.catalog.Profiles, id))
		m.detailPage = 0
		if m.confirming {
			m.confirming, m.proxyOnly = false, false
		}
	case tea.KeyMsg:
		key := msg.String()
		if m.confirming {
			switch key {
			case "y", "Y", "enter":
				m.launch = true
				return m, tea.Quit
			case "ctrl+c":
				return m, tea.Quit
			case "n", "N", "esc", "q":
				m.confirming = false
				m.proxyOnly = false
			}
			return m, nil
		}

		isDigits := key != ""
		for _, digit := range key {
			if digit < '0' || digit > '9' {
				isDigits = false
				break
			}
		}
		if key == "0" && (m.digitInput == "" || time.Since(m.lastDigit) > 800*time.Millisecond) {
			m.digitInput = ""
			m.proxyOnly = true
			m.confirming = true
			return m, nil
		}
		if isDigits {
			now := time.Now()
			if len(key) > 1 {
				m.digitInput = key
			} else {
				if now.Sub(m.lastDigit) > 800*time.Millisecond {
					m.digitInput = ""
				}
				m.digitInput += key
			}
			m.lastDigit = now
			n, err := strconv.Atoi(m.digitInput)
			if err == nil {
				if idx := indexOfID(m.catalog.Profiles, n); idx >= 0 {
					m.cursor = idx
					m.detailPage = 0
				}
			}
			if n >= 10 || len(m.digitInput) >= 2 {
				m.digitInput = ""
			}
			return m, nil
		}
		m.digitInput = ""

		switch key {
		case "ctrl+c", "q", "Q", "esc":
			return m, tea.Quit
		case "up", "k":
			m.cursor = (m.cursor - 1 + len(m.catalog.Profiles)) % len(m.catalog.Profiles)
			m.detailPage = 0
		case "down", "j":
			m.cursor = (m.cursor + 1) % len(m.catalog.Profiles)
			m.detailPage = 0
		case "home", "g":
			m.cursor = 0
			m.detailPage = 0
		case "end", "G":
			m.cursor = len(m.catalog.Profiles) - 1
			m.detailPage = 0
		case "pgdown", "ctrl+d":
			if m.width > 0 {
				m.detailPage = min(m.detailLayout().pageCount(), m.detailPage+1)
			}
		case "pgup", "ctrl+u":
			if m.width > 0 && m.detailPage > 0 {
				m.detailPage = max(0, min(m.detailPage, m.detailLayout().pageCount())-1)
			}
		case "enter":
			if m.refreshing {
				return m, nil
			}
			if len(m.catalog.Profiles[m.cursor].Missing) > 0 {
				return m, nil
			}
			m.confirming = true
		case "r", "R":
			if m.refreshing {
				return m, nil
			}
			m.refreshing = true
			m.err = nil
			return m, refreshCatalog(m.catalogPath)
		}
	}
	return m, nil
}

func badgeColor(p profile) lipgloss.Color {
	switch p.Badge {
	case "MISSING", "BROKEN":
		return red
	case "DEFAULT", "RECOMMENDED", "OFFICIAL 5090":
		return green
	case "VISION", "VISION+MTP":
		return orange
	case "BASELINE", "LOCAL TEST", "OVERLAY":
		return magenta
	case "NOT YET BOOTED":
		return yellow
	default:
		return cyan
	}
}

func groupColor(p profile) lipgloss.Color {
	if strings.Contains(strings.ToUpper(p.Group), "CPU") {
		return green
	}
	colors := []lipgloss.Color{orange, green, blue, magenta, cyan, yellow}
	hash := 0
	for _, ch := range p.Group {
		hash = (hash*33 + int(ch)) & 0x7fffffff
	}
	return colors[hash%len(colors)]
}

// Style punctuation separately so a comma never resets the surrounding
// foreground or selected-row background.
func styledText(style lipgloss.Style, text string) string {
	parts := strings.Split(text, ",")
	for i, part := range parts {
		parts[i] = style.Render(part)
	}
	return strings.Join(parts, style.Foreground(muted).Render(","))
}

func wrapLines(text string, width, limit int) []string {
	lines := strings.Split(ansi.Wrap(text, width, ""), "\n")
	if limit > 0 && len(lines) > limit {
		lines = lines[:limit]
		lines[limit-1] = ansi.Truncate(lines[limit-1], width-1, "") + "…"
	}
	return lines
}

func renderHeader(m model) string {
	p := m.catalog.Profiles[m.cursor]
	state, color := "OFFLINE", yellow
	if m.refreshing {
		state, color = "CHECKING", cyan
	} else if m.status.Online {
		state, color = "ONLINE", green
	}
	engine := m.catalog.App.Brand
	if engine == "" {
		engine = "LLM"
	}
	stateChip := renderer.NewStyle().Bold(true).Foreground(dark).Background(color).Padding(0, 1).Render(state)
	engineWidth := max(1, min(16, m.width-lipgloss.Width(stateChip)-7))
	prefix := brand.Render(ansi.Truncate(engine, engineWidth, "…")) + " " + stateChip + " " + title.Render(fmt.Sprintf("#%d", p.ID))

	var badges []string
	if len(p.Missing) > 0 {
		badges = append(badges, renderer.NewStyle().Foreground(red).Bold(true).Render("MISSING"))
	}
	if p.Badge != "" && (len(p.Missing) == 0 || p.Badge != "MISSING") {
		badges = append(badges, renderer.NewStyle().Foreground(badgeColor(p)).Render(p.Badge))
	}
	if !strings.Contains(p.Badge, "NOT YET BOOTED") && strings.Contains(strings.Join(p.Notes, " "), "NOT YET BOOTED") {
		badges = append(badges, renderer.NewStyle().Foreground(yellow).Render("NOT YET BOOTED"))
	}

	effort := detailMap(p)["Effort"]
	if effort != "" {
		effort = ", " + effort
	}
	budget := m.width - lipgloss.Width(prefix) - 1 - lipgloss.Width(effort)
	for _, badge := range badges {
		budget -= lipgloss.Width(badge) + 1
	}
	// Preserve the ID, status and readiness before the display name.
	parts := []string{prefix}
	if budget > 0 {
		name := ansi.Truncate(displayName(p), budget, "…")
		parts = append(parts, styledText(title, name+effort))
	}
	parts = append(parts, badges...)
	if m.status.Online && m.status.Model != "" && m.status.Model != p.ServedID {
		loaded := m.status.Model
		for _, candidate := range m.catalog.Profiles {
			if candidate.ServedID == loaded {
				loaded = fmt.Sprintf("#%d %s", candidate.ID, displayName(candidate))
				break
			}
		}
		label := "loaded " + loaded
		if remaining := m.width - lipgloss.Width(strings.Join(parts, " ")) - 1; remaining > len("loaded ")+2 {
			parts = append(parts, subtitle.Render(ansi.Truncate(label, remaining, "…")))
		}
	}
	return ansi.Truncate(strings.Join(parts, " "), m.width, "…")
}

func groupLabel(p profile) string {
	suffix := detailMap(p)["Draft style"]
	if p.GroupSuffix != nil {
		suffix = *p.GroupSuffix
	}
	if suffix == "" {
		return p.Group
	}
	if p.Group == "" {
		return suffix
	}
	return p.Group + ", " + suffix
}

type pickerChoice struct {
	heading []string
	row     string
}

func pickerVariant(p profile, name string) string {
	if group, ok := strings.CutPrefix(p.Group, "MODEL: "); ok {
		family, _, _ := strings.Cut(group, ", ")
		for _, separator := range []string{", ", ": "} {
			if variant, ok := strings.CutPrefix(name, family+separator); ok && variant != "" {
				return variant
			}
		}
	}
	return name
}

func pickerLabelParts(p profile) (name, effort, vision string) {
	name = p.ShortName
	if name == "" {
		name = pickerVariant(p, p.Name)
	}
	values := detailMap(p)
	if strings.HasPrefix(values["Vision"], "on") {
		vision = " V"
	}
	return name, values["Effort"], vision
}

func pickerLabel(p profile, width int) string {
	if width <= 0 {
		return ""
	}
	name, effort, vision := pickerLabelParts(p)
	if width < 5 {
		return ansi.Truncate(name, width, "…")
	}
	suffix := vision
	if effort != "" {
		suffix = " " + effort + vision
	}
	if lipgloss.Width(name+suffix) > width && p.ShortName != "" {
		name = pickerVariant(p, name)
	}
	if effort != "" {
		budget := width - lipgloss.Width(name+vision) - 1
		effort = " " + ansi.Truncate(effort, max(budget, width/2), "…")
	}
	return ansi.Truncate(name, width-lipgloss.Width(effort+vision), "…") + effort + vision
}

func renderProfileList(m model, width, height int) string {
	style := panel.BorderForeground(lipgloss.Color("#36586B"))
	if height < 5 || width < 16 {
		style = renderer.NewStyle()
	}
	available := height - style.GetVerticalFrameSize()
	inner := width - style.GetHorizontalFrameSize()
	labelWidth := inner - pickerRowGutter
	headingLimit := min(2, available-2)
	profiles := m.catalog.Profiles
	choices := make([]pickerChoice, len(profiles))
	total := 0
	for i, p := range profiles {
		accent := groupColor(p)
		choice := &choices[i]
		if i > 0 && p.Group == profiles[i-1].Group {
			choice.heading = choices[i-1].heading
		} else if label := groupLabel(p); label != "" && headingLimit > 0 {
			style := renderer.NewStyle().Bold(true).Foreground(accent)
			for _, line := range wrapLines(label, inner, headingLimit) {
				choice.heading = append(choice.heading, styledText(style, line))
			}
			total += len(choice.heading)
		}
		chip := renderer.NewStyle().Bold(true).Foreground(dark).Background(accent).Width(3).Align(lipgloss.Center).Render(strconv.Itoa(p.ID))
		marker := " "
		style := valueText
		if i == m.cursor {
			marker = keyStyle.Render("›")
			style = title.Background(selection)
		}
		label := pickerLabel(p, labelWidth)
		if i == m.cursor {
			label += strings.Repeat(" ", max(0, labelWidth-lipgloss.Width(label)))
		}
		choice.row = ansi.Truncate(marker+chip+" "+styledText(style, label), inner, "")
		total++
	}

	start := 0
	overflow := total > available
	showRange := overflow && available >= 3
	if overflow {
		if showRange {
			available--
		}
		start = m.cursor
		used := len(choices[start].heading) + 1
		for start > 0 {
			extra := 1
			if profiles[start-1].Group != profiles[start].Group {
				extra += len(choices[start-1].heading)
			}
			if used+extra > available {
				break
			}
			used += extra
			start--
		}
	}

	var lines []string
	end := start
	for i := start; i < len(choices); i++ {
		choice := choices[i]
		var heading []string
		if i == start || profiles[i].Group != profiles[i-1].Group {
			heading = choice.heading
		}
		if len(lines)+len(heading)+1 > available {
			break
		}
		lines = append(lines, heading...)
		lines = append(lines, choice.row)
		end = i + 1
	}
	if showRange {
		lines = append(lines, subtitle.Render(ansi.Truncate(fmt.Sprintf("%d–%d of %d", start+1, end, len(choices)), inner, "…")))
	}
	return style.Width(inner + style.GetHorizontalPadding()).Render(strings.Join(lines, "\n"))
}

func detailMap(p profile) map[string]string {
	out := make(map[string]string, len(p.Details))
	for _, pair := range p.Details {
		if len(pair) == 2 {
			out[pair[0]] = pair[1]
		}
	}
	return out
}

func profileFacts(p profile) map[string]string {
	values := detailMap(p)
	for _, section := range p.CompactSections {
		for _, row := range section {
			if len(row) == 2 && values[row[0]] == "" {
				values[row[0]] = row[1]
			}
		}
	}
	if p.ServedID != "" {
		values["Model id"] = p.ServedID
	}
	return values
}

func renderKV(label, value string, width, labelWidth int, accent lipgloss.Color, limit int) string {
	valueWidth := width
	indent := ""
	if labelWidth > 0 {
		valueWidth -= labelWidth + 1
	} else if width >= 16 {
		indent = "  "
		valueWidth -= len(indent)
	}
	var link string
	if label == "HF link" && strings.HasPrefix(value, "https://huggingface.co/") && renderer.ColorProfile().Name() != "Ascii" {
		link = value
		if lipgloss.Width(value) > valueWidth {
			value = "huggingface.co"
		}
	}
	labelStyle := renderer.NewStyle().Bold(true).Foreground(accent).Width(labelWidth)
	// Preserve a copyable endpoint or served ID whenever it fits the box.
	if (label == "API URL" || label == "Model id") && lipgloss.Width(value) > valueWidth && lipgloss.Width(value) <= width {
		return labelStyle.Render(label) + "\n" + styledText(valueText, value)
	}
	lines := wrapLines(value, valueWidth, limit)
	for i, line := range lines {
		lines[i] = indent + styledText(valueText, line)
		if link != "" {
			lines[i] = ansi.SetHyperlink(link) + lines[i] + ansi.ResetHyperlink()
		}
	}
	if labelWidth == 0 {
		return labelStyle.Render(ansi.Wrap(label, width, "")) + "\n" + strings.Join(lines, "\n")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, labelStyle.Render(label), " ", strings.Join(lines, "\n"))
}

func stackedSummaryRow(label string, accent lipgloss.Color, limit int) bool {
	if accent != green || limit == 0 {
		return false
	}
	return label == "Memory settings" || label == "Bench experts" || label == "Performance"
}

func section(rows [][]string, width int, accent lipgloss.Color, limit int) string {
	style := panel.BorderForeground(accent)
	if width < 19 {
		style = renderer.NewStyle()
	}
	inner := width - style.GetHorizontalFrameSize()
	labelWidth := 8
	for _, row := range rows {
		if len(row) == 2 && row[1] != "" && row[0] != "Estimated Vram/Ram Usage" &&
			!stackedSummaryRow(row[0], accent, limit) {
			labelWidth = max(labelWidth, lipgloss.Width(row[0]))
		}
	}
	if inner < 23 {
		labelWidth = 0
	} else {
		labelWidth = min(labelWidth, inner-15)
	}
	var content []string
	for _, row := range rows {
		if len(row) != 2 || row[1] == "" {
			continue
		}
		if row[0] == "Estimated Vram/Ram Usage" {
			// Retain the legacy full-width row without narrowing other values.
			labelStyle := renderer.NewStyle().Bold(true).Foreground(accent)
			lines := wrapLines(row[0]+": "+row[1], inner, limit)
			lines[0] = strings.Replace(lines[0], row[0]+":", labelStyle.Render(row[0]+":"), 1)
			content = append(content, strings.Join(lines, "\n"))
		} else if stackedSummaryRow(row[0], accent, limit) {
			rowLimit := limit
			if limit == 2 && row[0] != "Bench experts" {
				rowLimit = 3
			}
			content = append(content, renderKV(row[0], row[1], inner, 0, accent, rowLimit))
		} else {
			content = append(content, renderKV(row[0], row[1], inner, labelWidth, accent, limit))
		}
	}
	if len(content) == 0 {
		return ""
	}
	return style.Width(inner + style.GetHorizontalPadding()).Render(strings.Join(content, "\n"))
}

func displayName(p profile) string {
	if p.ShortName != "" {
		return p.ShortName
	}
	return p.Name
}

func detailSections(sections [][][]string, width, limit int) []string {
	accents := []lipgloss.Color{orange, green, magenta}
	var parts []string
	for i, rows := range sections {
		if box := section(rows, width, accents[i%len(accents)], limit); box != "" {
			parts = append(parts, box)
		}
	}
	return parts
}

func summarySections(p profile) [][][]string {
	values := profileFacts(p)
	if values["Model"] == "" {
		values["Model"] = p.Name
	}
	if values["Performance"] == "" {
		values["Performance"] = values["PERF"]
	}
	return [][][]string{
		{{"Model", values["Model"]}, {"Quant", values["Quant"]}, {"HF link", values["HF link"]}},
		{{"Context", values["Context"]}, {"Memory settings", values["Memory settings"]},
			{"Bench experts", values["Bench experts"]}, {"Performance", values["Performance"]}},
		{{"API URL", values["API URL"]}, {"Model id", values["Model id"]}},
	}
}

func detailsBody(m model, width, limit int) string {
	p := m.catalog.Profiles[m.cursor]
	sections := p.CompactSections
	compact := limit > 0
	if compact {
		sections = summarySections(p)
	} else if len(sections) > 0 && width < 100 {
		// Keep the projector on the Vision row when the pane is narrow.
		overview := sections[0]
		vision, projector := -1, -1
		for i, row := range overview {
			if len(row) == 2 && row[1] != "" {
				switch row[0] {
				case "Vision":
					vision = i
				case "Projector":
					projector = i
				}
			}
		}
		if vision >= 0 && projector >= 0 {
			overview = append([][]string(nil), overview...)
			overview[vision] = []string{"Vision", overview[vision][1] + ", " + overview[projector][1]}
			overview[projector] = nil
			sections = append([][][]string(nil), sections...)
			sections[0] = overview
		}
	}
	parts := detailSections(sections, width, limit)
	if len(parts) == 0 {
		parts = detailSections(summarySections(p), width, limit)
	}
	noticeLimit := 0
	if compact {
		noticeLimit = 1
	}
	if len(p.Missing) > 0 {
		style := renderer.NewStyle().Foreground(red)
		for _, line := range wrapLines("Missing: "+strings.Join(p.Missing, ", "), width, noticeLimit) {
			parts = append(parts, styledText(style, line))
		}
	}
	if m.err != nil {
		style := renderer.NewStyle().Foreground(red)
		var notice []string
		for _, line := range wrapLines("Catalog refresh failed: "+m.err.Error(), width, noticeLimit) {
			notice = append(notice, style.Render(line))
		}
		parts = append(notice, parts...)
	}
	return strings.Join(parts, "\n")
}

func idRange(profiles []profile) string {
	if len(profiles) == 0 {
		return ""
	}
	profiles = append([]profile(nil), profiles...)
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })
	parts := []string{}
	start, previous := profiles[0].ID, profiles[0].ID
	flush := func(first, last int) {
		if first == last {
			parts = append(parts, strconv.Itoa(first))
		} else {
			parts = append(parts, fmt.Sprintf("%d-%d", first, last))
		}
	}
	for _, p := range profiles[1:] {
		if p.ID == previous+1 {
			previous = p.ID
			continue
		}
		flush(start, previous)
		start, previous = p.ID, p.ID
	}
	flush(start, previous)
	return strings.Join(parts, ", ")
}

func renderFooter(width int, profiles []profile, page, pages int) string {
	action := func(key, label string) string {
		return styledText(keyStyle, key) + " " + subtitle.Render(label)
	}
	parts := []string{
		action("↑/↓", "select"), action(idRange(profiles), "jump"), action("Enter", "review"),
		action("0", "status"), action("R", "refresh"), action("Q", "quit"),
	}
	if pages > 0 && page == 0 {
		parts = append(parts, action("PgDn", "details"))
	} else if pages > 0 {
		parts = append(parts, action("PgUp/PgDn", fmt.Sprintf("%d/%d", page, pages)))
	}
	line := strings.Join(parts, "  ")
	if lipgloss.Width(line) > width {
		parts[1] = action("digits", "jump")
		line = strings.Join(parts, "  ")
	}
	if lipgloss.Width(line) > width {
		line = strings.Join(parts, " ")
	}
	if lipgloss.Width(line) > width {
		short := ""
		if pages > 0 {
			short = parts[len(parts)-1]
		}
		for _, key := range []string{"Q", "↑/↓", "Enter", "0", "R"} {
			if lipgloss.Width(short)+1+lipgloss.Width(key) <= width {
				short = strings.TrimSpace(short + " " + keyStyle.Render(key))
			}
		}
		line = short
	}
	return ansi.Truncate(line, width, "…")
}

func renderConfirm(m model) string {
	if m.width < 42 {
		return renderCompactConfirm(m)
	}
	p := m.catalog.Profiles[m.cursor]
	width := min(72, m.width-8)
	inner := width - 6
	var rows []string
	if m.proxyOnly {
		rows = append(rows, title.Render("Review status request"), "")
		if !m.status.Online {
			rows = append(rows, subtitle.Render("No model reported online."))
		} else if m.status.Model == "" {
			rows = append(rows, subtitle.Render("Server online."))
		} else {
			rows = append(rows, renderKV("Loaded", m.status.Model, inner, 8, cyan, 3))
		}
		rows = append(rows, subtitle.Render("Return a status-only request to the launcher."))
	} else {
		rows = append(rows, title.Render(fmt.Sprintf("Review profile #%d", p.ID)))
		for _, line := range wrapLines(displayName(p), inner, 2) {
			rows = append(rows, styledText(title, line))
		}
		rows = append(rows, "")
		values := profileFacts(p)
		for _, key := range []string{"Model id", "Context", "Effort"} {
			if value := values[key]; value != "" {
				rows = append(rows, renderKV(key, value, inner, 8, cyan, 3))
			}
		}
		if m.status.Online {
			rows = append(rows, subtitle.Render("A model is online. The launcher handles launch checks."))
		}
	}
	rows = append(rows, "", keyStyle.Render("Y / Enter")+" confirm   "+keyStyle.Render("N / Esc")+" back   "+keyStyle.Render("Ctrl-C")+" quit")
	style := renderer.NewStyle().Border(border).BorderForeground(cyan).Padding(1, 2).Width(width - 2)
	card := style.Render(strings.Join(rows, "\n"))
	if lipgloss.Height(card) > m.height || lipgloss.Width(card) > m.width {
		return renderCompactConfirm(m)
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, card)
}

func renderCompactConfirm(m model) string {
	p := m.catalog.Profiles[m.cursor]
	heading := fmt.Sprintf("Review #%d", p.ID)
	facts := []string{displayName(p)}
	values := profileFacts(p)
	for _, key := range []string{"Model id", "Context", "Effort"} {
		if value := values[key]; value != "" {
			facts = append(facts, key+" "+value)
		}
	}
	if m.proxyOnly {
		heading = "Review status"
		loaded := "Offline"
		if m.status.Online {
			loaded = "Online"
			if m.status.Model != "" {
				loaded += " " + m.status.Model
			}
		}
		facts = []string{loaded}
	}
	lines := []string{title.Render(ansi.Truncate(heading, m.width, "…"))}
	room := max(0, m.height-2)
	for _, fact := range facts {
		if room == 0 {
			break
		}
		wrapped := wrapLines(fact, m.width, room)
		for _, line := range wrapped {
			lines = append(lines, styledText(valueText, line))
		}
		room -= len(wrapped)
	}
	if m.height > 1 {
		keys := "Y confirm  Esc back"
		if m.width < lipgloss.Width(keys) {
			keys = "Y / Esc"
		}
		lines = append(lines, keyStyle.Render(ansi.Truncate(keys, m.width, "…")))
	}
	return strings.Join(lines, "\n")
}

func (m model) splitPanes() bool {
	return m.width >= minSplitPicker+minSplitDetails+1 && m.height >= 8
}

func (m model) pickerWidth() int {
	if !m.splitPanes() {
		return m.width
	}
	right := min(detailColumns, max(minSplitDetails, m.width-pickerColumns-1))
	minimum := min(pickerColumns, m.width-right-1)
	frame := panel.GetHorizontalFrameSize()
	inner := minimum - frame
	for _, p := range m.catalog.Profiles {
		name, effort, vision := pickerLabelParts(p)
		if effort != "" {
			effort = " " + effort
		}
		inner = max(inner, lipgloss.Width(groupLabel(p)), pickerRowGutter+lipgloss.Width(name+effort+vision))
	}
	return min(inner+frame, m.width-right-1)
}

type detailLayout struct {
	lines     []string
	height    int
	split     bool
	listWidth int
	width     int
}

func (m model) detailLayout() detailLayout {
	d := detailLayout{height: max(1, m.height-2), split: m.splitPanes(), listWidth: m.pickerWidth(), width: m.width}
	if d.split {
		d.width -= d.listWidth + 1
	}
	d.lines = strings.Split(detailsBody(m, d.width, 0), "\n")
	return d
}

func (d detailLayout) pageCount() int {
	if len(d.lines) <= d.height {
		if d.split {
			return 0
		}
		return 1
	}
	// Repeat the previous page's last line to keep wrapped settings readable.
	step := max(1, d.height-1)
	return 1 + (len(d.lines)-d.height+step-1)/step
}

func compactDetails(m model, width, height int) string {
	for _, limit := range []int{2, 1} {
		body := detailsBody(m, width, limit)
		if lipgloss.Height(body) <= height {
			return body
		}
	}
	p := m.catalog.Profiles[m.cursor]
	values := profileFacts(p)
	rows := [][]string{{"API URL", values["API URL"]}, {"Model id", values["Model id"]}, {"Context", values["Context"]}}
	if len(p.Missing) > 0 {
		rows = append([][]string{{"Missing", p.Missing[0]}}, rows...)
	} else if m.err != nil {
		rows = append([][]string{{"Refresh", "Catalog refresh failed"}}, rows...)
	}
	var lines []string
	for _, row := range rows {
		if row[1] != "" {
			accent := magenta
			switch row[0] {
			case "Context":
				accent = green
			case "Missing", "Refresh":
				accent = red
			}
			line := renderKV(row[0], row[1], width, 8, accent, 2)
			lines = append(lines, strings.Split(line, "\n")...)
		}
	}
	return strings.Join(lines[:min(len(lines), height)], "\n")
}

func (m model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading model catalog..."
	}
	if m.confirming {
		return renderConfirm(m)
	}
	header := renderHeader(m)
	if m.height == 1 {
		return header
	}
	layout := m.detailLayout()
	pages := layout.pageCount()
	page := min(m.detailPage, pages)
	footer := renderFooter(m.width, m.catalog.Profiles, page, pages)
	if m.height == 2 {
		return header + "\n" + footer
	}
	details := ""
	switch {
	case pages == 0:
		details = strings.Join(layout.lines, "\n")
	case page > 0:
		start := (page - 1) * max(1, layout.height-1)
		details = strings.Join(layout.lines[start:min(start+layout.height, len(layout.lines))], "\n")
	case layout.split:
		details = compactDetails(m, layout.width, layout.height)
	}
	body := details
	if layout.split || page == 0 {
		body = renderProfileList(m, layout.listWidth, layout.height)
		if layout.split {
			body = lipgloss.JoinHorizontal(lipgloss.Top, body, " ", details)
		}
	}
	body += strings.Repeat("\n", max(0, layout.height-lipgloss.Height(body)))
	return strings.Join([]string{header, body, footer}, "\n")
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Println("llm-tui", version)
		return
	}

	// Use the declared background when available. Querying a terminal that
	// ignores escape-sequence requests can delay the first frame for seconds.
	background := os.Getenv("COLORFGBG")
	background = background[strings.LastIndexByte(background, ';')+1:]
	renderer.SetHasDarkBackground(background != "7" && background != "15")

	path := os.Getenv("MODEL_UI_CATALOG")
	if path == "" {
		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "llm-tui:", err)
			os.Exit(1)
		}
		path = filepath.Join(filepath.Dir(exe), "catalog.json")
	}
	resultPath := os.Getenv("MODEL_UI_RESULT")
	c, err := loadCatalog(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "llm-tui:", err)
		os.Exit(1)
	}
	m := initialModel(c)
	m.catalogPath = path
	finalModel, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "llm-tui:", err)
		os.Exit(1)
	}
	result := finalModel.(model)
	if !result.launch {
		return
	}
	id := result.catalog.Profiles[result.cursor].ID
	if result.proxyOnly {
		id = 0
	}
	if resultPath == "" {
		_, err = fmt.Fprintln(os.Stdout, id)
	} else {
		err = os.WriteFile(resultPath, []byte(strconv.Itoa(id)+"\n"), 0600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "llm-tui:", err)
		os.Exit(1)
	}
}
