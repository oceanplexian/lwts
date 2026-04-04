# Filters & Search

## Filter Bar
- You should see a filter bar with controls for assignee, priority, tag, and text search.

## Assignee Filter
- Select "Alex Chen" in the assignee filter. Only Alex's cards should be visible. Column counts should update.
- Select both "Alex" and "Sam". Cards for both users should show.

## Priority Filter
- Select "Highest". Only highest-priority cards should be visible.
- Select "Highest" + "High". Both should show.

## Tag Filter
- Select "bug". Only bug-tagged cards should be visible.

## Text Search
- Type "shard" in the search box. Only cards with "shard" in the title should be visible.
- Type quickly. The filter should only apply after you stop typing (debounce).
- Click the X to clear search. All cards should reappear.

## Combined Filters
- Apply an assignee filter + a priority filter at the same time. Only cards matching both should show.

## Active Filter Indicator
- Apply 3 filters. You should see a badge showing "3 filters".
- Click "Clear all". All cards should reappear and the badge should disappear.
- With a filter active, column headers should show filtered counts (e.g. "1" instead of "3").
