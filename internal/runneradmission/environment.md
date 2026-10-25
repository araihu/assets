# Environment Variables

## Config

 - `GITHUB_REPOSITORY` (**required**, non-empty) - Repository selected by the GitHub runner.
 - `GITHUB_ACTOR` (**required**, non-empty) - Actor selected by the GitHub runner.
 - `GITHUB_EVENT_NAME` (**required**, non-empty) - GitHub event name. Only explicitly supported events are admitted.
 - `GITHUB_REF` (**required**, non-empty) - Git ref selected by the GitHub runner.
 - `GITHUB_EVENT_PATH` (**required**, non-empty) - Runner-provided event JSON file.

