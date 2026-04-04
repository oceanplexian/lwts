# Card Operations

## Card Rendering
- Log in and load a board with seed data. You should see 8 cards across the columns.
- Each card should show its key (e.g. "LWTS-1"), and these should match the server values.
- Cards with an assignee should show their avatar initials (e.g. "AC" for Alex Chen).

## Create Card
- Click the add card button. Fill in a title "New task", select a priority, and submit. The card should appear in the correct column.
- If the "keep open" toggle is on, the modal should stay open with an empty form after creating.

## Card Detail Modal
- Click any card. A detail modal should open showing the card's title, description, priority, assignee, points, and comments.
- Edit the title and click away. The change should save automatically.
- Edit the description in the rich editor, click Save. The change should persist.
- Change the priority dropdown from "Medium" to "High". It should save immediately.
- Change the points value, blur the field. It should save.

## Card Drag & Drop
- Drag a card from "To Do" to "In Progress". The card should land in the new column and stay there after release.
- Drag a card to position 0 in a column. Existing cards should shift down.

## Card Delete
- Open a card's detail, click delete. The card should disappear from the board.
- Click "Clear done" (if available). All cards in the Done column should be removed.

## Due Dates
- Open a card detail. You should see a date picker.
- Set a due date. The date should appear on the card in the board view.
- A card with a past due date should show the date in red.
- A card due today should show "Today" in orange.
- Clear the date. The date display should disappear from the card.

## Sort by Due Date
- Enable due date sorting. Cards should reorder by date within each column.
- Cards without dates should appear at the bottom.
