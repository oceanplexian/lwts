package repo

import (
	"context"
	"testing"

	"github.com/oceanplexian/lwts/server/internal/db"
	"github.com/oceanplexian/lwts/server/migrations"
)

func setupTestDB(t *testing.T) db.Datasource {
	t.Helper()
	ds, err := db.NewSQLiteDatasource("sqlite://:memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	ctx := context.Background()
	if err := db.Migrate(ctx, ds, migrations.FS); err != nil {
		ds.Close()
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { ds.Close() })
	return ds
}

// ── UserRepository ──

func TestUserCreate(t *testing.T) {
	ds := setupTestDB(t)
	repo := NewUserRepository(ds)
	ctx := context.Background()

	u, err := repo.Create(ctx, "Alice Smith", "Alice@LWTS.dev", "hash123")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.Initials != "AS" {
		t.Errorf("initials = %q, want AS", u.Initials)
	}
	if u.Email != "alice@lwts.dev" {
		t.Errorf("email = %q, want lowercase", u.Email)
	}
	if u.Role != "member" {
		t.Errorf("role = %q, want member", u.Role)
	}
	if u.AvatarColor == "" {
		t.Error("avatar_color should not be empty")
	}
}

func TestUserGetByID(t *testing.T) {
	ds := setupTestDB(t)
	repo := NewUserRepository(ds)
	ctx := context.Background()

	u, _ := repo.Create(ctx, "Bob Jones", "bob@test.com", "hash")
	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Bob Jones" {
		t.Errorf("name = %q", got.Name)
	}
}

func TestUserGetByEmail(t *testing.T) {
	ds := setupTestDB(t)
	repo := NewUserRepository(ds)
	ctx := context.Background()

	_, _ = repo.Create(ctx, "Alice Smith", "Alice@LWTS.dev", "hash")
	got, err := repo.GetByEmail(ctx, "alice@lwts.dev")
	if err != nil {
		t.Fatalf("get by email: %v", err)
	}
	if got.Name != "Alice Smith" {
		t.Errorf("name = %q", got.Name)
	}
}

func TestUserUpdate(t *testing.T) {
	ds := setupTestDB(t)
	repo := NewUserRepository(ds)
	ctx := context.Background()

	u, _ := repo.Create(ctx, "Alice Smith", "alice@test.com", "hash")
	newName := "Alice Jones"
	updated, err := repo.Update(ctx, u.ID, UserUpdate{Name: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Alice Jones" {
		t.Errorf("name = %q", updated.Name)
	}
	if updated.Initials != "AJ" {
		t.Errorf("initials = %q, want AJ", updated.Initials)
	}
	if !updated.UpdatedAt.After(u.CreatedAt) {
		t.Error("updated_at should be after created_at")
	}
}

func TestUserList(t *testing.T) {
	ds := setupTestDB(t)
	repo := NewUserRepository(ds)
	ctx := context.Background()

	_, _ = repo.Create(ctx, "A", "a@test.com", "h")
	_, _ = repo.Create(ctx, "B", "b@test.com", "h")
	_, _ = repo.Create(ctx, "C", "c@test.com", "h")

	users, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(users) != 3 {
		t.Errorf("len = %d, want 3", len(users))
	}
	// password_hash should not be populated (List doesn't select it)
	for _, u := range users {
		if u.PasswordHash != "" {
			t.Error("password_hash should be empty in list results")
		}
	}
}

func TestUserNotFound(t *testing.T) {
	ds := setupTestDB(t)
	repo := NewUserRepository(ds)
	ctx := context.Background()

	_, err := repo.GetByID(ctx, "nonexistent")
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ── BoardRepository ──

func TestBoardCreate(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	b, err := boards.Create(ctx, "My Board", "MB", owner.ID)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if b.ProjectKey != "MB" {
		t.Errorf("project_key = %q", b.ProjectKey)
	}
	if b.Columns == "" || b.Columns == "{}" {
		t.Error("columns should have default value")
	}
}

func TestBoardCRUD(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	_, _ = boards.Create(ctx, "Board A", "BA", owner.ID)
	_, _ = boards.Create(ctx, "Board B", "BB", owner.ID)

	list, err := boards.List(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("len = %d, want 2", len(list))
	}

	// Update
	newName := "Renamed"
	updated, err := boards.Update(ctx, list[0].ID, BoardUpdate{Name: &newName})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "Renamed" {
		t.Errorf("name = %q", updated.Name)
	}

	// Delete
	if err := boards.Delete(ctx, list[0].ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, _ = boards.List(ctx)
	if len(list) != 1 {
		t.Errorf("len after delete = %d", len(list))
	}
}

// ── CardRepository ──

func TestCardCreate(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	board, _ := boards.Create(ctx, "Board", "LWTS", owner.ID)

	c, err := cards.Create(ctx, board.ID, CardCreate{
		ColumnID: "todo",
		Title:    "First card",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c.Key != "LWTS-1" {
		t.Errorf("key = %q, want LWTS-1", c.Key)
	}
	if c.Version != 1 {
		t.Errorf("version = %d, want 1", c.Version)
	}

	// Second card
	c2, _ := cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "Second card"})
	if c2.Key != "LWTS-2" {
		t.Errorf("key = %q, want LWTS-2", c2.Key)
	}
}

func TestCardListByBoard(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	board, _ := boards.Create(ctx, "Board", "LWTS", owner.ID)

	_, _ = cards.Create(ctx, board.ID, CardCreate{ColumnID: "backlog", Title: "C1"})
	_, _ = cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "C2"})
	_, _ = cards.Create(ctx, board.ID, CardCreate{ColumnID: "backlog", Title: "C3"})

	list, err := cards.ListByBoard(ctx, board.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("len = %d, want 3", len(list))
	}
	// Should be ordered by column_id then position
	if list[0].ColumnID != "backlog" || list[1].ColumnID != "backlog" {
		t.Error("first two cards should be backlog")
	}
	if list[2].ColumnID != "todo" {
		t.Error("third card should be todo")
	}
}

func TestCardOptimisticLock(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	board, _ := boards.Create(ctx, "Board", "LWTS", owner.ID)
	card, _ := cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "Test"})

	// Update with correct version
	newTitle := "Updated"
	updated, err := cards.Update(ctx, card.ID, 1, CardUpdate{Title: &newTitle})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("version = %d, want 2", updated.Version)
	}

	// Update with stale version
	staleTitle := "Stale"
	_, err = cards.Update(ctx, card.ID, 1, CardUpdate{Title: &staleTitle})
	if err != ErrConflict {
		t.Errorf("err = %v, want ErrConflict", err)
	}
}

func TestCardMove(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	board, _ := boards.Create(ctx, "Board", "LWTS", owner.ID)
	card, _ := cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "Mover"})

	moved, err := cards.Move(ctx, card.ID, 1, "done", 0)
	if err != nil {
		t.Fatalf("move: %v", err)
	}
	if moved.ColumnID != "done" {
		t.Errorf("column = %q, want done", moved.ColumnID)
	}
	if moved.Position != 0 {
		t.Errorf("position = %d, want 0", moved.Position)
	}
	if moved.Version != 2 {
		t.Errorf("version = %d, want 2", moved.Version)
	}
}

func TestCardDelete(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	board, _ := boards.Create(ctx, "Board", "LWTS", owner.ID)
	card, _ := cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "Delete me"})

	if err := cards.Delete(ctx, card.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	_, err := cards.GetByID(ctx, card.ID)
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// ── CommentRepository ──

func TestCommentCRUD(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	comments := NewCommentRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	board, _ := boards.Create(ctx, "Board", "LWTS", owner.ID)
	card, _ := cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "Card"})

	c1, err := comments.Create(ctx, card.ID, owner.ID, "First comment")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if c1.Body != "First comment" {
		t.Errorf("body = %q", c1.Body)
	}

	_, _ = comments.Create(ctx, card.ID, owner.ID, "Second")
	_, _ = comments.Create(ctx, card.ID, owner.ID, "Third")

	list, err := comments.ListByCard(ctx, card.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Errorf("len = %d, want 3", len(list))
	}
	// Should be chronological
	if list[0].Body != "First comment" {
		t.Errorf("first = %q", list[0].Body)
	}

	// Delete
	if err := comments.Delete(ctx, c1.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	list, _ = comments.ListByCard(ctx, card.ID)
	if len(list) != 2 {
		t.Errorf("len after delete = %d", len(list))
	}
}

// ── Seed ──

func TestSeedDemo(t *testing.T) {
	ds := setupTestDB(t)
	ctx := context.Background()

	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)

	// Create an owner user first (SeedDemo requires an existing user)
	owner, err := users.Create(ctx, "Owner", "owner@test.com", "hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}

	if err := SeedDemo(ctx, ds, owner.ID); err != nil {
		t.Fatalf("seed demo: %v", err)
	}

	bl, _ := boards.List(ctx)
	if len(bl) != 1 {
		t.Fatalf("boards = %d, want 1", len(bl))
	}

	cl, _ := cards.ListByBoard(ctx, bl[0].ID)
	if len(cl) != 29 {
		t.Errorf("cards = %d, want 29", len(cl))
	}

	// SeedDemo now creates 3 additional team members + the owner = 4 total
	ul, _ := users.List(ctx)
	if len(ul) != 4 {
		t.Errorf("users = %d, want 4", len(ul))
	}
}

// ── Cascade ──

func TestBoardDeleteCascadesCards(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	board, _ := boards.Create(ctx, "Board", "LWTS", owner.ID)
	_, _ = cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "C1"})
	_, _ = cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "C2"})

	_ = boards.Delete(ctx, board.ID)

	list, _ := cards.ListByBoard(ctx, board.ID)
	if len(list) != 0 {
		t.Errorf("cards after board delete = %d, want 0", len(list))
	}
}

func TestCardDeleteCascadesComments(t *testing.T) {
	ds := setupTestDB(t)
	users := NewUserRepository(ds)
	boards := NewBoardRepository(ds)
	cards := NewCardRepository(ds)
	comments := NewCommentRepository(ds)
	ctx := context.Background()

	owner, _ := users.Create(ctx, "Owner", "owner@test.com", "hash")
	board, _ := boards.Create(ctx, "Board", "LWTS", owner.ID)
	card, _ := cards.Create(ctx, board.ID, CardCreate{ColumnID: "todo", Title: "C"})
	_, _ = comments.Create(ctx, card.ID, owner.ID, "comment")

	_ = cards.Delete(ctx, card.ID)

	list, _ := comments.ListByCard(ctx, card.ID)
	if len(list) != 0 {
		t.Errorf("comments after card delete = %d, want 0", len(list))
	}
}
