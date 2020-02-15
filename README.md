# JIRA-SYNC
A pipeline of Open-Integration that is using Jira API.
The pipeline is running JQL to list all the issue that have been updated and inclduing the user in the description or in the comment and sends it to slack.
The pipeline is designed to be executed on daily basis to get the last day updates.

# Run
To run this pipeline locally:
* `git clone https://github.com/olegsu/jira-sync`
* `cd jira-sync`
* `export SLACK_WEBHOOK_URL=`
* `export JIRA_USER=`
* `export JIRA_ENDPOINT=`
* `export JIRA_API_TOKEN=`
* `export JIRA_START_DAY=-1`
* `export TRELLO_APP_ID=`
* `export TRELLO_BOARD_ID=`
* `export TRELLO_API_TOKEN=`
* `export TRELLO_LIST_ID=`
* `export TRELLO_LABEL_IDS=`
* `./build.run.sh`