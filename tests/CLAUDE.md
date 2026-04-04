# UI Test Specs

These are interface tests meant to be run by an LLM agent with Chrome CDP access against a **disposable test server** — never against production.

## Test Server

The test server runs on **port 8044** with a SQLite database at `/tmp/lwts-test-ui.db`. It is seeded with edge-case data (overflow titles, missing fields, max values, all metadata combos, special characters).

- **URL**: `http://localhost:8044`
- **Login**: `testowner@test.dev` / `testpass123`
- The database is wiped and re-seeded each time the server starts.

A `SessionStart` hook in `.claude/settings.json` automatically starts the test server when you enter this directory. If the hook fails, fix the Go build error before proceeding — do not test against production.

## Running Tests

1. The test server should already be running (started by the hook). Verify: `curl -sf http://localhost:8044/healthz`
2. Launch Chrome with CDP: `~/.claude/skills/chrome-cdp/scripts/cdp.mjs launch`
3. Navigate to `http://localhost:8044`
4. Walk through the test spec files in order (01-login.md through 11-card-overflow.md)
5. All tests target `localhost:8044` — **never** `localhost:8099` or any deployed URL

## Login Recipe

```bash
# Navigate to login page
cdp nav <target> "http://localhost:8044/login.html"

# Enter credentials
cdp click <target> "#login-email"
cdp type <target> "testowner@test.dev"
cdp click <target> "#login-password"
cdp type <target> "testpass123"
cdp click <target> "button[type=submit]"

# Wait for redirect to board
sleep 2
```

## Welcome Modal

On first login, a "Welcome to LWTS" modal appears. Dismiss it before testing:

```bash
cdp eval <target> "var btns = document.querySelectorAll('button'); for(var i=0;i<btns.length;i++){if(btns[i].textContent.trim()==='Get started'){btns[i].click();break;}} 'dismissed'"
```

## CDP Tips for These Tests

- **Prefer `eval` over screenshots** for checking layout properties (heights, overflow, wrapping). Screenshots are for visual confirmation only.
- **Use ES5 syntax in `eval`** — arrow functions and spread syntax can fail in some CDP contexts. Use `var`, `function(){}`, `Array.from()`, `Array.prototype.filter.call()`.
- **Card height check pattern**:
  ```bash
  cdp eval <target> "var cards = document.querySelectorAll('.card'); var heights = Array.from(cards).map(function(c){ return c.getBoundingClientRect().height; }); var unique = heights.filter(function(v,i,a){ return a.indexOf(v)===i; }); JSON.stringify({count: cards.length, uniqueHeights: unique, allSame: unique.length === 1})"
  ```
- **Metadata wrapping check** — don't compare `.top` values between footer-left and footer-right (they differ by ~5px due to baseline alignment, not wrapping). Instead check that both children vertically overlap:
  ```bash
  cdp eval <target> "var f = document.querySelector('.card-footer'); var l = f.querySelector('.card-footer-left').getBoundingClientRect(); var r = f.querySelector('.card-footer-right').getBoundingClientRect(); JSON.stringify({sameRow: l.bottom > r.top && r.bottom > l.top, footerHeight: Math.round(f.getBoundingClientRect().height)})"
  ```
- **Title line clamp check** — titles use CSS line-clamp. `scrollHeight > clientHeight` means the text IS longer than 2 lines but is properly hidden. That's a PASS, not a fail.

## Test Data

The `seed-test` command creates:
- 4 users (Test Owner, Alexandra Konstantinopoulou-Papadimitriou, B, Test User)
- 1 board ("Edge Cases", key prefix "EDGE")
- ~28 cards across all columns with edge cases:
  - Titles: 200-char no-space strings, long natural text, 1-char, 2-char, HTML injection, emoji/unicode, slash-heavy paths
  - Metadata combos: all fields populated, no fields, only-date, only-points, only-assignee
  - Points: 0, 1, 2, 3, 5, 8, 13, 21, 42, 55, 88, 99
  - Due dates: yesterday (red), today (orange), tomorrow (normal), far future, none
  - Assignees: very long name, single-char name, normal name, none
  - Priorities: all 5 levels represented
  - Comments: varying counts for badge testing

## Stopping the Server

```bash
./tests/start-test-server.sh stop
```
