RULE: When implementing new logic, you must also create unit tests for it.
RULE 2: If the new feature is a CRUD operation, make sure to make a corresponding CLI command for managing the state.
RULE 3: You must update the TODO file once a task is complete.

✅ Recently added is empty on the home page if media failed to find a match during indexing: No media have been indexed yet.
✅ Add a CLI command to enable / disable maintenance mode.
✅ Add CLI commands to manage libraries and series.
✅ The activity loading indicator shows all logs every time, it should only show new logs. So something in the backend sends all logs every time, not just new ones.
✅ Update the log outputs on the scraper and config page to always autoscroll down to the latest log.
✅ Update the unread notifications to only load on first page load, not on a clock.
✅ When a new scraper has been created, the tabs at the top should be updated to reflect this.
✅ The save and create button on the scraper page looks super weird and doesnt have a icon.
✅ Make the scraper disable button use destructive style.
✅ When a new library has been created, the form should be cleared, the same for editing.
✅ Add the NEW badge from the home page the series page.
✅ If i press a series from the home page in the LatestUpdates grid, the content is injected to fill the entire page instead of just the content segment of my layout. The same also happens if i press a chapter from the series page. I think this occured after adding hs-boost, which i need, but fix the target.
✅ Make the NEW badge duration configurable through the config view.
✅ Let's give the tabs on the scraper page a active state.