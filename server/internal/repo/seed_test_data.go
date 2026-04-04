package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/oceanplexian/lwts/server/internal/db"
)

// SeedTestData creates a board with edge-case cards designed to stress-test
// UI layout: overflow titles, missing fields, max values, all metadata combos.
// ownerID must already exist.
func SeedTestData(ctx context.Context, ds db.Datasource, ownerID string) error {
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	comments := NewCommentRepository(ds)
	users := NewUserRepository(ds)

	// ── Create test users ──
	type testUser struct {
		Name  string
		Email string
		ID    string
	}
	testUsers := []testUser{
		{Name: "Alexandra Konstantinopoulou-Papadimitriou", Email: "alex.k@test.dev"},
		{Name: "B", Email: "b@test.dev"},
		{Name: "Test User", Email: "test@test.dev"},
	}
	userIDs := map[string]string{"owner": ownerID}
	for i, u := range testUsers {
		created, err := users.Create(ctx, u.Name, u.Email, "$2b$10$G1Gt3Zvd76mWYPu8TDXrn.hU5c33FaVkkwDfu8b0FWZmlTmCRUgZW")
		if err != nil {
			return fmt.Errorf("create test user %q: %w", u.Name, err)
		}
		testUsers[i].ID = created.ID
		userIDs[u.Email] = created.ID
	}
	longNameID := testUsers[0].ID
	shortNameID := testUsers[1].ID
	normalID := testUsers[2].ID

	board, err := boards.Create(ctx, "Edge Cases", "EDGE", ownerID)
	if err != nil {
		return fmt.Errorf("create test board: %w", err)
	}

	type testCard struct {
		Column   string
		Title    string
		Tag      string
		Priority string
		Points   int
		DueDate  string
		Desc     string
		Assignee string
		Reporter string
	}

	yesterday := "2026-04-03"
	today := "2026-04-04"
	tomorrow := "2026-04-05"
	farOut := "2026-12-31"

	testCards := []testCard{
		// ── OVERFLOW: Long titles ──
		{Column: "backlog", Title: "Investigate why the production database connection pool is exhausting all available connections during peak traffic hours causing widespread service degradation across the entire platform", Tag: "red", Priority: "highest", Points: 13, DueDate: today,
			Assignee: longNameID, Reporter: ownerID,
			Desc: "This is a card with maximum metadata: long title, high points, due today, long assignee name, all fields populated."},

		{Column: "backlog", Title: "Fix the extremely critical production outage affecting all users in the APAC region that started at 3am UTC and is causing revenue loss estimated at approximately fifty thousand dollars per hour", Tag: "red", Priority: "highest", Points: 99, DueDate: yesterday,
			Assignee: ownerID, Reporter: longNameID,
			Desc: "Max points (99), overdue, longest possible title."},

		{Column: "todo", Title: strings.Repeat("A", 200), Tag: "blue", Priority: "medium", Points: 8, DueDate: tomorrow,
			Assignee: normalID, Reporter: ownerID,
			Desc: "200 character title with no word breaks — tests CSS word-break/overflow behavior."},

		{Column: "todo", Title: "ABCDEFGHIJKLMNOPQRSTUVWXYZ-1234567890-ABCDEFGHIJKLMNOPQRSTUVWXYZ-1234567890-ABCDEFGHIJKLMNOPQRSTUVWXYZ", Tag: "orange", Priority: "high", Points: 21, DueDate: today,
			Assignee: shortNameID, Reporter: ownerID,
			Desc: "Long unbreakable string with hyphens — tests overflow-wrap behavior."},

		// ── OVERFLOW: Short titles ──
		{Column: "in-progress", Title: "X", Tag: "blue", Priority: "low", Points: 1,
			Assignee: ownerID, Reporter: ownerID,
			Desc: "Single character title. Card should be same height as all others."},

		{Column: "in-progress", Title: "Hi", Tag: "green", Priority: "lowest", Points: 0,
			Desc: "Two character title, zero points, no assignee, no due date."},

		// ── METADATA COMBOS: All fields vs no fields ──
		{Column: "backlog", Title: "Card with every field populated", Tag: "red", Priority: "highest", Points: 99, DueDate: farOut,
			Assignee: longNameID, Reporter: longNameID,
			Desc: "Every optional field filled with max values. Card must be same height as empty cards."},

		{Column: "backlog", Title: "Card with nothing — no points, no date, no assignee, no tag",
			Priority: "medium",
			Desc: "Absolutely bare minimum card. Same height as the card above."},

		{Column: "todo", Title: "Only has a due date", Priority: "medium", DueDate: yesterday,
			Desc: "Just a due date, nothing else. Overdue — should show red."},

		{Column: "todo", Title: "Only has points", Tag: "blue", Priority: "medium", Points: 42,
			Desc: "Just points, nothing else."},

		{Column: "todo", Title: "Only has an assignee", Priority: "medium",
			Assignee: longNameID,
			Desc: "Just an assignee with a very long name."},

		// ── PRIORITY ICON + KEY + DATE in one row ──
		{Column: "in-progress", Title: "All metadata icons in one row", Tag: "orange", Priority: "highest", Points: 55, DueDate: today,
			Assignee: normalID, Reporter: ownerID,
			Desc: "Priority icon + EDGE-XX key + due date + points + avatar — all must fit on one line."},

		{Column: "in-progress", Title: "Lowest priority with all metadata", Tag: "blue", Priority: "lowest", Points: 88, DueDate: tomorrow,
			Assignee: longNameID, Reporter: longNameID,
			Desc: "Same as above but lowest priority. Verify icon doesn't change row height."},

		// ── DONE column — verify cleared cards look right ──
		{Column: "done", Title: "Completed card with full metadata", Tag: "green", Priority: "high", Points: 5, DueDate: yesterday,
			Assignee: normalID, Reporter: ownerID,
			Desc: "Done card should look identical in height to backlog cards."},

		{Column: "done", Title: "Completed card with nothing",
			Priority: "medium",
			Desc: "Bare done card."},

		// ── VOLUME: Many cards in one column to test scroll ──
		{Column: "backlog", Title: "Backlog filler 1 — padding card for scroll testing", Tag: "blue", Priority: "low", Points: 1},
		{Column: "backlog", Title: "Backlog filler 2 — more padding for scroll", Tag: "green", Priority: "medium", Points: 2},
		{Column: "backlog", Title: "Backlog filler 3 — even more padding", Tag: "orange", Priority: "high", Points: 3},
		{Column: "backlog", Title: "Backlog filler 4 — keep scrolling", Tag: "red", Priority: "highest", Points: 5},
		{Column: "backlog", Title: "Backlog filler 5 — still going", Tag: "blue", Priority: "low", Points: 8},
		{Column: "backlog", Title: "Backlog filler 6 — last one", Tag: "green", Priority: "lowest", Points: 13},

		// ── DUE DATE STATES ──
		{Column: "todo", Title: "Due yesterday — should be red", Tag: "red", Priority: "high", DueDate: yesterday,
			Assignee: ownerID},
		{Column: "todo", Title: "Due today — should be orange", Tag: "orange", Priority: "medium", DueDate: today,
			Assignee: normalID},
		{Column: "todo", Title: "Due tomorrow — should be normal", Tag: "blue", Priority: "low", DueDate: tomorrow,
			Assignee: shortNameID},
		{Column: "todo", Title: "Due far out — Dec 31", Tag: "green", Priority: "lowest", DueDate: farOut},

		// ── SPECIAL CHARACTERS in title ──
		{Column: "in-progress", Title: "Fix <script>alert('xss')</script> in user input", Tag: "red", Priority: "high", Points: 3,
			Assignee: ownerID, Desc: "HTML in title must be escaped, not rendered."},
		{Column: "in-progress", Title: "Support émojis 🎉 and ünïcödé in títlés", Tag: "blue", Priority: "medium", Points: 2,
			Assignee: normalID, Desc: "Unicode and emoji must not break layout."},
		{Column: "in-progress", Title: "Path: /usr/local/bin/very/deeply/nested/directory/structure/config.yaml", Tag: "orange", Priority: "low", Points: 1,
			Desc: "Slash-heavy path — tests word-break on slashes."},
	}

	for _, tc := range testCards {
		points := tc.Points
		var dueDate *string
		if tc.DueDate != "" {
			dueDate = &tc.DueDate
		}
		var assignee *string
		if tc.Assignee != "" {
			assignee = &tc.Assignee
		}
		var reporter *string
		if tc.Reporter != "" {
			reporter = &tc.Reporter
		}

		card, err := cards.Create(ctx, board.ID, CardCreate{
			ColumnID:    tc.Column,
			Title:       tc.Title,
			Description: tc.Desc,
			Tag:         tc.Tag,
			Priority:    tc.Priority,
			AssigneeID:  assignee,
			ReporterID:  reporter,
			Points:      &points,
			DueDate:     dueDate,
		})
		if err != nil {
			return fmt.Errorf("create test card %q: %w", tc.Title[:min(len(tc.Title), 40)], err)
		}

		// Add a comment to every 3rd card to test comment badge
		if len(testCards) > 0 && tc.Points > 3 {
			for j := 0; j < tc.Points%5+1; j++ {
				body := fmt.Sprintf("Test comment %d on card %s", j+1, card.Key)
				if _, err := comments.Create(ctx, card.ID, ownerID, body); err != nil {
					return fmt.Errorf("create test comment on %q: %w", card.Key, err)
				}
			}
		}
	}

	fmt.Printf("created test board %q with %d edge-case cards\n", board.Name, len(testCards))
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
