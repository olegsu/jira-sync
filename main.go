package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/open-integration/core"
	"github.com/open-integration/service-catalog/jira/pkg/endpoints/list"
)

type (
	buildTrelloAddCardTaskOptions struct {
		taskName              string
		trelloAppID           string
		trelloAPIToken        string
		trelloBoardID         string
		trelloListID          string
		trelloCardName        string
		trelloCardDescription string
		trelloLebelIDs        []string
	}

	buildSlackTaskOptions struct {
		url     string
		message string
	}

	buildJiraTaskOptions struct {
		token    string
		endpoint string
		user     string
	}
)

func main() {
	jiraToken := getEnvOrDie("JIRA_API_TOKEN")
	jiraEndpoint := getEnvOrDie("JIRA_ENDPOINT")
	jiraUser := getEnvOrDie("JIRA_USER")
	slackURL := getEnvOrDie("SLACK_WEBHOOK_URL")
	trelloAppID := getEnvOrDie("TRELLO_APP_ID")
	trelloBoardID := getEnvOrDie("TRELLO_BOARD_ID")
	trelloAPIToken := getEnvOrDie("TRELLO_API_TOKEN")
	trelloListID := getEnvOrDie("TRELLO_LIST_ID")
	trelloLebelIDs := getEnvOrDie("TRELLO_LABEL_IDS")
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
				core.Service{
					Name:    "trello",
					Version: "0.10.0",
					As:      "trello",
				},
			},
			Reactions: []core.EventReaction{
				core.EventReaction{
					Condition: core.ConditionEngineStarted,
					Reaction: func(ev core.Event, state core.State) []core.Task {
						return []core.Task{
							buildJiraTask(&buildJiraTaskOptions{
								token:    jiraToken,
								endpoint: jiraEndpoint,
								user:     jiraUser,
							}),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus("Get Latest Issues", core.TaskStatusSuccess),
					Reaction: func(ev core.Event, state core.State) []core.Task {
						output := ""
						for _, t := range state.Tasks {
							if t.Task == "Get Latest Issues" {
								output = t.Output
							}
						}
						list := &list.ListReturns{}
						json.Unmarshal([]byte(output), list)
						message := strings.Builder{}
						if len(list.Issues) == 0 {
							message.WriteString("No new mentions in the last day")
						}
						for _, issue := range list.Issues {
							if issue.Self != nil {
								message.WriteString(fmt.Sprintf("An issue i was mentioned in was updated %s/browse/%s \n", jiraEndpoint, *issue.Key))
							}
						}
						return []core.Task{
							buildSlackTask(&buildSlackTaskOptions{
								url:     slackURL,
								message: message.String(),
							}),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus("Get Latest Issues", core.TaskStatusSuccess),
					Reaction: func(ev core.Event, state core.State) []core.Task {
						output := ""
						for _, t := range state.Tasks {
							if t.Task == "Get Latest Issues" {
								output = t.Output
							}
						}
						list := &list.ListReturns{}
						json.Unmarshal([]byte(output), list)
						tasks := []core.Task{}
						for _, issue := range list.Issues {
							task := buildTrelloAddCardTask(&buildTrelloAddCardTaskOptions{
								taskName:              fmt.Sprintf("Create card for issue %s", *issue.ID),
								trelloAPIToken:        trelloAPIToken,
								trelloAppID:           trelloAppID,
								trelloBoardID:         trelloBoardID,
								trelloCardDescription: fmt.Sprintf("Added by open-integration pipeline at %s\nLink: %s/browse/%s", time.Now(), jiraEndpoint, *issue.Key),
								trelloCardName:        fmt.Sprintf("Follow up with issue %s", *issue.Key),
								trelloLebelIDs:        []string{trelloLebelIDs},
								trelloListID:          trelloListID,
							})
							tasks = append(tasks, task)
						}
						return tasks
					},
				},
			},
		},
	}
	opt := &core.EngineOptions{
		Pipeline: pipe,
	}
	runInKubeCluster := os.Getenv("RUN_IN_CLUSTER")
	kubeNamespace := os.Getenv("KUBE_NAMESPACE")
	if runInKubeCluster != "" {
		if kubeNamespace == "" {
			kubeNamespace = "default"
		}
		opt.Kubeconfig = &core.EngineKubernetesOptions{
			InCluster: true,
			Namespace: kubeNamespace,
		}
	}
	e := core.NewEngine(opt)
	err := e.Run()
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(1)
	}
}

func getEnvOrDie(name string) string {
	val := os.Getenv(name)
	if val == "" {
		fmt.Printf("%s is required and not set, exiting")
		os.Exit(1)
	}
	return val
}

func buildJiraTask(options *buildJiraTaskOptions) core.Task {
	return core.Task{
		Metadata: core.TaskMetadata{
			Name: "Get Latest Issues",
		},
		Spec: core.TaskSpec{
			Service:  "jira",
			Endpoint: "list",
			Arguments: []core.Argument{
				core.Argument{
					Key:   "API_Token",
					Value: options.token,
				},
				core.Argument{
					Key:   "Endpoint",
					Value: options.endpoint,
				},
				core.Argument{
					Key:   "User",
					Value: options.user,
				},
				core.Argument{
					Key:   "JQL",
					Value: "status != Done AND (comment ~ currentUser() OR description ~ currentUser()) AND updatedDate > startOfDay(-1)",
				},
				core.Argument{
					Key:   "QueryFields",
					Value: "*all",
				},
			},
		},
	}
}

func buildSlackTask(options *buildSlackTaskOptions) core.Task {
	return core.Task{
		Metadata: core.TaskMetadata{
			Name: "Send Slack Message",
		},
		Spec: core.TaskSpec{
			Service:  "slack",
			Endpoint: "message",
			Arguments: []core.Argument{
				core.Argument{
					Key:   "Webhook_URL",
					Value: options.url,
				},
				core.Argument{
					Key:   "Message",
					Value: options.message,
				},
			},
		},
	}
}

func buildTrelloAddCardTask(options *buildTrelloAddCardTaskOptions) core.Task {
	task := core.Task{
		Metadata: core.TaskMetadata{
			Name: options.taskName,
		},
		Spec: core.TaskSpec{
			Endpoint: "addcard",
			Service:  "trello",
			Arguments: []core.Argument{
				core.Argument{
					Key:   "App",
					Value: options.trelloAppID,
				},
				core.Argument{
					Key:   "Token",
					Value: options.trelloAPIToken,
				},
				core.Argument{
					Key:   "Board",
					Value: options.trelloBoardID,
				},
				core.Argument{
					Key:   "List",
					Value: options.trelloListID,
				},
				core.Argument{
					Key:   "Name",
					Value: options.trelloCardName,
				},
				core.Argument{
					Key:   "Description",
					Value: options.trelloCardDescription,
				},
				core.Argument{
					Key:   "Labels",
					Value: options.trelloLebelIDs,
				},
			},
		},
	}
	return task
}
