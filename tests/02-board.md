# Board Management

## Board Loading
- Log in. The board should load cards from the server (not localStorage).
- If the server has 3 boards, the board picker should show 3 options.

## Board Picker
- Click the board picker. You should see all boards listed, with the current board highlighted.
- The last option should be "+ New board".

## Create Board
- Click "+ New board" in the picker. A modal should appear with name and key fields.
- Create a board called "Sprint 3". The picker should now show "Sprint 3" as the active board.
- Try submitting with an empty name. You should see an error.

## Board Switching
- Switch from one board to another in the picker. The columns should re-render with the new board's cards.
- Select a board, reload the page. The same board should still be selected.

## Board Settings
- Open Settings > Boards. Change the board name. The header should update without reloading.
- Change the board key from "LWTS" to "KBN". New cards should use the "KBN-" prefix.
- Disable a column (e.g. "Backlog"). That column should disappear from the board view.

## Delete Board
- In Settings > Boards > Danger Zone, delete a board. It should disappear from the picker.
