package main

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestFixtureLayout(t *testing.T) {
	c, err := loadCatalog("examples/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, example := range []struct {
		id   int
		perf string
	}{{1, "unmeasured"}, {12, "68 tok/s decode"}} {
		m := initialModel(c)
		m.cursor, m.width, m.height = indexOfID(c.Profiles, example.id), 110, 35
		screen := ansi.Strip(m.View())
		for _, want := range []string{"Example Text", "Example Vision", "API URL", "Model id", "Performance", example.perf} {
			if !strings.Contains(screen, want) {
				t.Errorf("profile %d missing %q in:\n%s", example.id, want, screen)
			}
		}
		assertFits(t, m, screen)
	}
	m := initialModel(c)
	m.width, m.height = 80, 25
	for i, example := range c.Profiles {
		m.cursor = i
		screen := ansi.Strip(m.View())
		if !strings.Contains(screen, example.ServedID) {
			t.Errorf("profile %d lacks a copyable served ID at 80×25:\n%s", example.ID, screen)
		}
		assertFits(t, m, screen)
	}
	m.cursor = indexOfID(c.Profiles, 12)
	profile := &c.Profiles[m.cursor]
	profile.CompactSections[1] = append(profile.CompactSections[1],
		[]string{"Extra", strings.Repeat("example ", 200)})
	screen := ansi.Strip(m.View())
	for _, want := range []string{"Example Text", "Example Vision", "PgDn details", "API URL", "example-vision-49152-mtp"} {
		if !strings.Contains(screen, want) {
			t.Errorf("compact screen missing %q in:\n%s", want, screen)
		}
	}
	assertFits(t, m, screen)

	pages := m.detailLayout().pageCount()
	if pages < 2 {
		t.Fatalf("expected overflow in test catalog, got %d detail pages", pages)
	}
	for page := 1; page <= pages; page++ {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgDown})
		m = next.(model)
		screen = ansi.Strip(m.View())
		if m.detailPage != page || !strings.Contains(screen, fmt.Sprintf("PgUp/PgDn %d/%d", page, pages)) {
			t.Errorf("page %d missing its navigation hint:\n%s", page, screen)
		}
		if !strings.Contains(screen, "›12") {
			t.Errorf("page %d lost the selected profile:\n%s", page, screen)
		}
		assertFits(t, m, screen)
	}
	if !strings.Contains(screen, "Sampling") || !strings.Contains(screen, "Reasoning") {
		t.Errorf("could not reach the end of the API details:\n%s", screen)
	}
	for range pages {
		next, _ := m.Update(tea.KeyMsg{Type: tea.KeyPgUp})
		m = next.(model)
	}
	if m.detailPage != 0 || !strings.Contains(ansi.Strip(m.View()), "PgDn details") {
		t.Fatal("Page Up must return from the first page to the compact overview")
	}
}

func TestLargeCatalogPicker(t *testing.T) {
	c, err := loadCatalog("examples/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	template := c.Profiles[0]
	c.Profiles = nil
	for id := 1; id <= 99; id += 2 {
		p := template
		p.ID = id
		p.Group = fmt.Sprintf("MODEL: Group %d, 98304", id/15)
		p.ShortName = fmt.Sprintf("Choice %d with a longer label", id)
		c.Profiles = append(c.Profiles, p)
	}
	last := &c.Profiles[len(c.Profiles)-1]
	last.Group = strings.Repeat("Unusually long group heading ", 40)
	last.ShortName = strings.Repeat("Long selected choice ", 40)
	last.Badge = "NOT YET BOOTED"

	m := initialModel(c)
	m.width, m.height = 80, 25
	for _, action := range []struct {
		key tea.KeyType
		id  int
	}{
		{tea.KeyHome, 1},
		{tea.KeyEnd, 99},
		{tea.KeyUp, 97},
	} {
		next, _ := m.Update(tea.KeyMsg{Type: action.key})
		m = next.(model)
		screen := ansi.Strip(m.View())
		if m.catalog.Profiles[m.cursor].ID != action.id ||
			!strings.Contains(screen, fmt.Sprintf("›%2d", action.id)) {
			t.Errorf("could not see selected profile %d:\n%s", action.id, screen)
		}
		for _, want := range []string{"API URL", "digits jump", "Enter review", "Q quit"} {
			if !strings.Contains(screen, want) {
				t.Errorf("profile %d missing %q in:\n%s", action.id, want, screen)
			}
		}
		if action.id == 99 && !strings.Contains(screen, "NOT YET BOOTED") {
			t.Errorf("long name pushed readiness out of the header:\n%s", screen)
		}
		assertFits(t, m, screen)
	}
}

func TestPickerChoicesStayOnOneRow(t *testing.T) {
	m := initialModel(catalog{Profiles: []profile{
		{ID: 1, Group: "MODEL: Inkling 2 Mini, 262144", ShortName: "Inkling 2 Mini, draft off", Details: [][]string{{"Effort", "high (0.9)"}}},
		{ID: 2, Group: "MODEL: Inkling 2 Mini, 262144", ShortName: "Inkling 2 Mini, DSpark", Details: [][]string{{"Effort", "high (0.9)"}}},
	}})
	m.cursor, m.width, m.height = 1, 80, 25
	picker := ansi.Strip(renderProfileList(m, m.pickerWidth(), m.height-2))
	for _, want := range []string{"MODEL: Inkling 2 Mini, 262144", " 1  draft off high (0.9)", "› 2  DSpark high (0.9)"} {
		if !strings.Contains(picker, want) {
			t.Errorf("choice %q must fit on one line:\n%s", want, picker)
		}
	}

	label := "Inkling 2 Mini, baseline with no speculative draft"
	m.catalog.Profiles[0].ShortName = label
	m.width = 120
	picker = ansi.Strip(renderProfileList(m, m.pickerWidth(), m.height-2))
	if !strings.Contains(picker, " 1  "+label+" high (0.9)") {
		t.Errorf("wide picker should fit the full choice on one line:\n%s", picker)
	}
	assertFits(t, m, ansi.Strip(m.View()))

	m.width = 80
	picker = ansi.Strip(renderProfileList(m, m.pickerWidth(), m.height-2))
	if !strings.Contains(picker, "… high (0.9)") {
		t.Errorf("narrow picker should shorten the label and preserve effort on one line:\n%s", picker)
	}
	assertFits(t, m, ansi.Strip(m.View()))
}

func TestSmallerTerminals(t *testing.T) {
	c, err := loadCatalog("examples/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []tea.WindowSizeMsg{
		{Width: 72, Height: 20},
		{Width: 52, Height: 14},
		{Width: 24, Height: 7},
	} {
		t.Run(fmt.Sprintf("%dx%d", size.Width, size.Height), func(t *testing.T) {
			m := initialModel(c)
			update := func(msg tea.Msg) {
				next, _ := m.Update(msg)
				m = next.(model)
			}
			update(size)
			update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("12")})
			screen := ansi.Strip(m.View())
			for _, want := range []string{"#12", "›12", "PgDn details"} {
				if !strings.Contains(screen, want) {
					t.Errorf("picker missing %q:\n%s", want, screen)
				}
			}
			assertFits(t, m, screen)

			pages := m.detailLayout().pageCount()
			var details []string
			for i := 0; i < pages; i++ {
				update(tea.KeyMsg{Type: tea.KeyPgDown})
				screen := ansi.Strip(m.View())
				assertFits(t, m, screen)
				details = append(details, screen)
			}
			if !strings.Contains(strings.Join(details, "\n"), "Model id") {
				t.Error("could not reach the API details")
			}
			for range pages {
				update(tea.KeyMsg{Type: tea.KeyPgUp})
			}
			if m.detailPage != 0 || !strings.Contains(ansi.Strip(m.View()), "›12") {
				t.Fatal("Page Up did not return to the selected choice")
			}

			update(tea.KeyMsg{Type: tea.KeyEnter})
			review := ansi.Strip(m.View())
			if !m.confirming || m.launch || !strings.Contains(review, "#12") {
				t.Errorf("profile was not available for review:\n%s", review)
			}
			assertFits(t, m, review)
			update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
			if !m.launch || m.proxyOnly || m.catalog.Profiles[m.cursor].ID != 12 {
				t.Fatal("could not confirm the selected profile")
			}
		})
	}
}

func assertFits(t *testing.T, m model, screen string) {
	t.Helper()
	if lipgloss.Width(screen) > m.width || lipgloss.Height(screen) > m.height {
		t.Errorf("screen exceeds %d×%d:\n%s", m.width, m.height, screen)
	}
}

func TestReviewWorkflow(t *testing.T) {
	c, err := loadCatalog("examples/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	m := initialModel(c)
	m.width, m.height = 80, 25
	key := func(msg tea.KeyMsg) {
		next, _ := m.Update(msg)
		m = next.(model)
	}
	runeKey := func(r rune) { key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}}) }
	runeKey('1')
	runeKey('2')
	key(tea.KeyMsg{Type: tea.KeyEnter})
	if m.catalog.Profiles[m.cursor].ID != 12 || !m.confirming || m.launch {
		t.Fatal("digits and Enter must select and review without confirming")
	}
	review := ansi.Strip(m.View())
	for _, want := range []string{"Review profile #12", "example-vision-49152-mtp", "49152", "high"} {
		if !strings.Contains(review, want) {
			t.Errorf("review missing %q:\n%s", want, review)
		}
	}
	assertFits(t, m, review)
	key(tea.KeyMsg{Type: tea.KeyEsc})
	if m.confirming || m.launch {
		t.Fatal("cancel must not confirm")
	}
	runeKey('0')
	if !m.confirming || !m.proxyOnly || m.launch {
		t.Fatal("0 must review a status-only request")
	}
	runeKey('y')
	if !m.launch || !m.proxyOnly {
		t.Fatal("confirm must preserve the status-only result")
	}
}
