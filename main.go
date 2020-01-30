package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/open-integration/core"
	list "github.com/open-integration/service-catalog/jira/pkg/endpoints/list"
)

func main() {
	jiraToken := os.Getenv("JIRA_API_TOKEN")
	jiraEndpoint := os.Getenv("JIRA_ENDPOINT")
	jiraUser := os.Getenv("JIRA_USER")
	slackURL := os.Getenv("SLACK_WEBHOOK_URL")
	pipe := core.Pipeline{
		Metadata: core.PipelineMetadata{
			Name: "sync-jira",
		},
		Spec: core.PipelineSpec{
			Services: []core.Service{
				core.Service{
					Name:    "slack",
					Version: "0.1.0",
					As:      "slack",
				},
				core.Service{
					Name:    "jira",
					Version: "0.1.0",
					As:      "jira",
				},
			},
			Tasks: []core.Task{
				core.Task{
					Metadata: core.TaskMetadata{
						Name: "Get Latest Issues",
					},
					Condition: &core.Condition{
						Name: "On Start",
						Func: core.ConditionEngineStarted,
					},
					Spec: core.TaskSpec{
						Service:  "jira",
						Endpoint: "list",
						Arguments: []core.Argument{
							core.Argument{
								Key:   "API_Token",
								Value: jiraToken,
							},
							core.Argument{
								Key:   "Endpoint",
								Value: jiraEndpoint,
							},
							core.Argument{
								Key:   "User",
								Value: jiraUser,
							},
							core.Argument{
								Key:   "JQL",
								Value: "status != Done AND (comment ~ currentUser() OR comment ~ currentUser()) AND updatedDate > startOfDay(-1)",
							},
							core.Argument{
								Key:   "QueryFields",
								Value: "comment",
							},
						},
					},
				},
				core.Task{
					Metadata: core.TaskMetadata{
						Name: "Send Slack Message",
					},
					Condition: &core.Condition{
						Name: "On Start",
						Func: core.ConditionTaskFinishedWithStatus("Get Latest Issues", core.TaskStatusSuccess),
					},
					SpecFunc: func(state *core.State) (*core.TaskSpec, error) {
						output := ""
						for _, t := range state.Tasks {
							if t.Task == "Get Latest Issues" {
								output = t.Output
							}
						}
						list := &list.ListReturns{}
						json.Unmarshal([]byte(output), list)
						message := strings.Builder{}
						for _, issue := range list.Issues {
							if issue.Self != nil {
								message.WriteString(fmt.Sprintf("I was mentioned in %s/browse/%s \n", jiraEndpoint, *issue.Key))
							}
						}
						task := core.TaskSpec{
							Service:  "slack",
							Endpoint: "message",
							Arguments: []core.Argument{
								core.Argument{
									Key:   "Webhook_URL",
									Value: slackURL,
								},
								core.Argument{
									Key:   "Message",
									Value: message.String(),
								},
							},
						}
						return &task, nil
					},
				},
			},
		},
	}
	opt := &core.EngineOptions{
		Pipeline: pipe,
	}
	e := core.NewEngine(opt)
	err := e.Run()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}
