# Decisions

Choices made while building the app, one or two lines each.

## Product

1. **A host for many WhatsApp tasks, named "Sabeel WhatsApp Tools".** "Course groups" is its first workflow; later ones plug into the same app (see DEVELOPING.md). The data folder is therefore `sabeel-whatsapp-tools`, not `course-groups`.
2. **Sabeel brand instead of the suggested green/teal.** Ivory canvas, raspberry for primary actions, sage-green for success, gold as hairlines, light theme only. These are the institute's brand rules. Every colour is a token in `web/src/app.css`.
3. **Invite messages open with "Assalamu alaikum".** The message stays fully editable before anyone copies or sends it.
4. **Plain language throughout.** The UI never shows JIDs, E.164, HTTP codes or "API". Server errors carry a user-facing message, written for people, that the UI shows as-is.
5. **Shared Import → Match → Review steps.** They live in `web/src/people` and `internal/people`, so future workflows that start from a spreadsheet reuse them.

## WhatsApp

6. **whatsmeow is pinned to commit `5d4657a` (2026-10-31).** whatsmeow publishes no releases; the latest commit was the most recent API.
7. **New groups are created empty, then everyone is added in batches.** Every person gets an individual result, and one bad number can't fail the creation itself.
8. **Pacing:** 5 people per batch with random 4–9 s pauses; numbers looked up 20 at a time with 2–4 s pauses; invites sent 8–15 s apart; retries after 5, 15 and 45 s; "slow down" answers wait 60 s, doubling, up to 4 times. All of this is in `internal/whatsapp/pacing.go`. Demo mode runs 4× faster.
9. **The group size limit is 1,024 members**, including the admin. whatsmeow has no constant for it; WhatsApp documents 1,024. Group names are limited to 25 characters, counted in UTF-16 units as WhatsApp counts them, so an emoji counts as 2.
10. **How WhatsApp's answer for each person is read:** 403 means privacy settings (an invite is needed); 408 means they left recently (an invite is needed); 409 means already a member; 404 means not on WhatsApp; anything else is a failure.
11. **Numbers are checked with WhatsApp automatically on the Review step.** Checks are paced and cached for 6 hours. A run re-checks any number that wasn't checked, so the outcome is right whichever path the user took.
12. **Pause and Stop take effect between steps.** A request already sent always finishes, so the app always knows what happened. Stopping keeps everything done so far, and the results page offers "Add the remaining N people".
13. **When the network drops mid-run, the job waits** (shown as "Waiting…") until whatsmeow reconnects, then continues without using up a retry. Being logged out, a temporary ban, an outdated app or the link being opened elsewhere ends the run, and its results are saved.
14. **When WhatsApp says the client is outdated,** the app fetches the current WhatsApp Web version once (`whatsmeow.GetLatestVersion`) and reconnects. Only if WhatsApp still refuses does it show "Please update the app".
15. **Passkey-protected accounts can't link by QR code.** Linking them needs a WebAuthn step on whatsapp.com that a local app can't perform, so the app explains this and suggests the pairing code.
16. **Personal invites** use WhatsApp's invite card when the 403 answer includes an invite code, and a text message with the link otherwise. They are opt-in, confirmed in a dialog, and sent one at a time.
17. **The linked device shows on the phone as "Sabeel WhatsApp Tools" (Desktop).** Pairing codes report "Chrome (Linux/Windows/Mac OS)", because WhatsApp only accepts common browser names there.
18. **Community admin is detected from the Community's own group or its announcement group.** Groups outside any Community where the account is an admin are listed too, so late registrants can be added to them.
19. **If WhatsApp can't be reached when the user disconnects,** the app still removes the link locally and says so.

## Data, privacy, security

20. **Session database:** `session.db` through the pure-Go `modernc.org/sqlite` (driver name `sqlite`). The connection string is a plain file path with `_pragma=foreign_keys(1)`, WAL and a busy timeout. The file is created with mode 0600 before SQLite opens it, and the data folder is 0700.
21. **Run history is one JSON file per run** (0600), written atomically. Runs left "running" by a crash become "Interrupted" on the next start.
22. **Logs are redacted when written:** every run of 7 or more digits and every `+`-international number becomes `[redacted]`. whatsmeow logs only at Info and above. Logs rotate at 5 MB and three old files are kept.
23. **The session token is passed in the URL fragment (`#token=`), not the query string,** so it never reaches server logs or Referer headers. The page moves it to `sessionStorage`, and every API call sends it as the `X-App-Token` header, which cross-site pages can't send without a CORS preflight the server never grants. The server also checks the Host header (against DNS rebinding), `Origin` and `Sec-Fetch-Site`, and serves a strict Content Security Policy that blocks every outside request.
24. **Live updates use server-sent events read with `fetch`,** so the token can travel as a header. An open event stream counts as activity, and the page also sends a heartbeat every minute. With no tab open and no job running for 15 minutes, the app quits by itself.
25. **Single instance:** an OS file lock (`flock` on Linux and macOS, `LockFileEx` on Windows) plus `instance.json` with the running copy's address. Demo mode keeps its own data in a `demo` subfolder, so it can run alongside the real app.
26. **Sleep is prevented only while a job runs:** `systemd-inhibit` on Linux, `caffeinate` on macOS, `SetThreadExecutionState` on Windows.

## Spreadsheets and phone numbers

27. **"Fixed automatically" means a real repair:** an Excel number format, a leading 00, an extension, a label like "Cell:", or a country code without its "+". Adding the default country code doesn't count, otherwise every US list would be all "fixed".
28. **Excel scientific notation** (e.g. `1.2125550123E+10`) is recovered only when every digit survived. Otherwise the row is marked "damaged by Excel". A cell with two numbers needs attention; the app doesn't guess which one to use.
29. **Duplicates keep the first row,** which borrows a name from its duplicate if it has none. The connected account's own number is left out.
30. **Delimiters are detected** by how consistently each candidate splits the first rows. A header row is recognised by familiar words, or by plain text sitting above data-like values. Columns empty everywhere are dropped. Excel's `sep=;` line is honoured.
31. **`.xlsx` files are refused** with instructions to save as CSV, rather than adding a workbook parser.
32. **Column choices are remembered per set of headers** (order, case and punctuation ignored), and stored by header text so reordered columns still match.

## Front end and packaging

33. **Svelte 5 + Vite + TypeScript.** Inter is bundled (`@fontsource-variable/inter`) and so are the Lucide icons. Routing uses the URL hash. Wizard drafts stay in memory for the session, so Back, or leaving the wizard and returning, never loses work.
34. **The built UI is embedded from `internal/webui/dist`** with `go:embed`. A placeholder file there keeps Go builds working before the first UI build.
35. **Windows:** built with `-H=windowsgui`. Icon, manifest and version info come from go-winres. Startup errors show in a message box, because there is no console.
36. **macOS:** a universal binary made with `lipo` on a Mac and `makefat` elsewhere. The bundle sets `LSUIElement` (no Dock icon, since a Go binary has no Cocoa event loop to answer "Quit" from the Dock); the app quits from its own Quit button. CI signs the bundle ad hoc, and `scripts/package-macos.sh` has a placeholder for Developer ID signing and notarization. The minimum is macOS 12, as Go requires.
37. **Linux:** a `.tar.gz` with `install.sh` into `~/.local`, plus a `.desktop` file with a "Try it in demo mode" action. No AppImage: the tarball already gives an apps-menu entry without needing FUSE.
38. **One GitHub Actions workflow** lints, runs the tests and the end-to-end suite, builds every package, self-tests each package on its own OS, and publishes a release for `v*` tags.
39. **App icon:** the Sabeel app family's icon (the calligraphy with a raspberry badge), with a new "group" badge, generated by `scripts/make-icons.py` on top of the brand repository's icon tool. The glyph should be moved into that tool upstream.
40. **The demo client is deterministic** (outcomes come from a hash of the number), and a demo-only `/api/demo/simulate` endpoint produces offline, logout, ban and outdated states for testing and screenshots.
