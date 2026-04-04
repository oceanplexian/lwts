# Card Overflow & Layout Consistency

These tests apply to all three density modes: compact, default, and comfortable. Run each check in all three.

## Card Height Consistency
- Load a board with cards of varying content (short title, long title, with/without assignee, with/without due date). All cards in a column should be the same height.
- Switch to list view. All rows should be the same height.

## Title Overflow
- Create a card with a very long title (e.g. "Investigate why the production database connection pool is exhausting all available connections during peak hours"). The title should truncate at 2 lines max. No card should grow taller because of a long title.

## Metadata Row (key, priority, date, points)
- Find a card that has all metadata: key (LWTS-3), priority icon, due date, and points. The entire metadata row should stay on one line. Nothing should wrap to a second line.
- Create a card with a long key prefix (e.g. "SPRINT-123"), a due date, points, and priority. The metadata row should still be one line — items should truncate or hide before wrapping.

## Avatars & Icons
- Assignee avatars should not push the metadata row to a second line.
- Priority icons should stay inline with the key and never stack vertically.

## Edge Cases
- Card with no metadata (no date, no points, no assignee). It should be the same height as cards with full metadata.
- Card with a 1-word title. Same height as all other cards.
- Card with maximum points value (e.g. 99). Should not cause overflow.
