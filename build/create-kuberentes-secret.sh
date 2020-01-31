echo "Creating secret in kuberentes cluster"

kubectl create secret generic jira-sync \
    --from-literal=slack-webhook-url=$SLACK_WEBHOOK_URL \ 
    --from-literal=jira-user=$JIRA_USER \ 
    --from-literal=jira-endpoint=$JIRA_ENDPOINT \ 
    --from-literal=jira-api-token=$JIRA_API_TOKEN

