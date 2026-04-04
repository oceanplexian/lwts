# Realtime & SSE

## Live Card Updates
- Open the board in two browser sessions (or use the API to simulate a second user).
- Create a card in session 2. It should appear in session 1 without reloading.
- Update a card title via the API. The title should change in the connected browser.
- Move a card via the API. The card should move in real-time.
- Delete a card via the API. The card should disappear from the connected browser.
- Add a comment via the API. The comment count badge should update on the card.

## Conflict Handling
- Open the same card in two sessions. Edit the title in session 1 and save. Then edit the title in session 2 and save. Session 2 should see a "Card was modified" notification.

## User Presence
- Connect multiple users to the same board. You should see their avatars in the presence area.
- With 7 users connected, you should see 5 avatars + a "+2" overflow indicator.
- Hover an avatar. A tooltip with the username should appear.

## Board Switch
- Switch boards. The SSE connection should close and a new one should open for the new board.
