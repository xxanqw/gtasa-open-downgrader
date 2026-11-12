# Troubleshooting and diagnostic logs

[Back to the project](../README.md)

Open **Log** after a failed action. Use search or **Warnings & errors** to find the relevant entry, and export the original text when reporting a problem.

**Log** uses the read-only settings-editor renderer with syntax highlighting, compact timestamps, wrapped fields, event filters, search, selection/copy and live follow. Diagnostics include operation/parent IDs, durations, build and desktop details, HTTP status/redirects/request IDs and connection details, file hashes, archive filtering, backups, configuration preservation, rollback results and error causes. Request URLs omit credentials, query parameters and fragments; request headers, cookies and full environment dumps are excluded.

The viewer and its Copy all/Export actions retain up to 1 MiB of recent session entries. Logs also survive app exit in `logs/current.log` under **Tools → Downgrader folder** (the app cache directory). Disk logging rotates at startup and at 4 MiB, retaining three segments (`current.log`, `.1`, `.2`) for a maximum of 12 MiB. Include those files too when the relevant operation has scrolled out of the session log. Local paths are included for troubleshooting.

The log reader offers **I'm creating an issue on GitHub**. It asks before uploading the current raw session log to `files.xxanqw.me`, explains that local paths are included and anyone with the link can read it, and offers a path without uploading. Successful uploads provide a copyable, 30-day log-reader link and an **Open GitHub issue** action with that link already in the draft; the user reviews and submits the issue on GitHub.

For file recovery, see [Using the downgrader](usage.md). Include the app version, OS, game edition, reproduction steps, and the log when [opening an issue](https://github.com/xxanqw/gtasa-open-downgrader/issues). Do not upload copyrighted game files.
