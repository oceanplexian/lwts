# Settings

## Workspace
- Open Settings > General. Change the workspace name to "MyTeam" and blur the field. The header should update.
- Reload the page. "MyTeam" should still be there.

## Default Assignee
- Open the default assignee dropdown. You should see all team members + "Unassigned".
- Select "Alex Chen". Create a new card. The assignee should default to Alex.

## Theme
- Toggle dark mode off. The body should get the `light-theme` class and the background should change.
- Toggle it back on. Dark theme should restore.

## Density
- Select "Compact". The body should get `density-compact` class and card padding should shrink.
- Select "Large". Font size should increase.

## Board Display Toggles
- Toggle "Show card keys" off. Card keys (e.g. "LWTS-1") should disappear from all cards.
- Toggle "Show avatars" off. Assignee avatars should disappear.
- Toggle "Show priority" off. Priority indicators should disappear.

## Persistence
- Set compact density + large font. Reload the page. Both styles should be applied before content is visible (no flash).

## Members
- You should see all members listed with avatar, name, email, and role.
- The owner row should show a static "Owner" badge with no role dropdown.
- As admin, change another user's role from Admin to Member. The change should save.
- As a regular member, role dropdowns should be disabled.

## Remove Member
- Click remove on a member. A confirmation dialog should appear before the API call.
- Confirm removal. The member should disappear from the list.
- Your own row should not have a remove button.

## Invite
- Type "new@test.com" and click Invite. You should see a success toast.
- Type "notanemail" and click Invite. You should see a validation error.

## Danger Zone
- Click "Clear all cards", confirm. All columns should be empty.
- The clear button should be disabled until you type the workspace name to confirm.
