echo "Creating secret in kuberentes cluster"

kubectl create secret generic jira-sync \
    --from-literal=slack-webhook-url=$SLACK_WEBHOOK_URL \
    --from-literal=jira-user=$JIRA_USER \
    --from-literal=jira-endpoint=$JIRA_ENDPOINT \
    --from-literal=jira-api-token=$JIRA_API_TOKEN \
    --from-literal=trello-app-id=$TRELLO_APP_ID \
    --from-literal=trello-board-id=$TRELLO_BOARD_ID \
    --from-literal=trello-api-token=$TRELLO_API_TOKEN \
    --from-literal=trello-list-id=$TRELLO_LIST_ID \
    --from-literal=trello-label-ids=$TRELLO_LABEL_IDS

