Changelog
=========

All notable changes to Sabeel WhatsApp Tools. Versions follow `Semantic Versioning <https://semver.org>`_.

0.1.0 (unreleased)
------------------

First version.

Added
~~~~~

- Desktop app for Linux, Windows and macOS that runs a private local web page and works through a WhatsApp account
  linked as a device (QR code or pairing code).
- **Course groups** workflow: import a registration spreadsheet (.csv, .tsv, .txt; any delimiter; UTF-8, UTF-16 or
  Windows-1252; with or without a header row), match the phone and name columns with suggestions that are remembered
  per layout, review every number (automatic repairs, problems explained, duplicates merged, WhatsApp check), then create
  a group inside a Community or add people to an existing group.
- Paced adding (5 at a time with random pauses), automatic retries, waiting through network drops, pause and stop.
- Results with the invite link, a ready-to-send message, opt-in personal invites, CSV export, and "add the remaining
  people" after a stopped run. Every run is kept under Recent activity.
- Demo mode (``--demo``) with a realistic pretend account.
- Single instance, Quit button, idle shutdown, sleep prevention during a run, logs with phone numbers removed.
- Linux tarball with installer and menu entry, Windows executable with icon and version info, universal macOS app.
- GitHub Actions: tests, end-to-end suite, packages self-tested on each OS, releases on version tags.
