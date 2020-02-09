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

const (
	taskGetAllIssuesWithMentions    = "Get latest mentions issues"
	taskGetAllIssuesWhereIamWatcher = "Get Latest watched issues"
)

var now = func() string { return time.Now().Format("2006-01-02") }

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
		taskName string
		token    string
		endpoint string
		user     string
		jql      string
	}
)

func main() {
	jiraToken := getEnvOrDie("JIRA_API_TOKEN")
	jiraEndpoint := getEnvOrDie("JIRA_ENDPOINT")
	jiraUser := getEnvOrDie("JIRA_USER")
	jiraStartDay := getEnvOrDie("JIRA_START_DAY")
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
								taskName: taskGetAllIssuesWithMentions,
								token:    jiraToken,
								endpoint: jiraEndpoint,
								user:     jiraUser,
								jql:      fmt.Sprintf("status != Done AND (comment ~ currentUser() OR description ~ currentUser()) AND updatedDate > startOfDay(%s)", jiraStartDay),
							}),
							buildJiraTask(&buildJiraTaskOptions{
								taskName: taskGetAllIssuesWhereIamWatcher,
								token:    jiraToken,
								endpoint: jiraEndpoint,
								user:     jiraUser,
								jql:      fmt.Sprintf("status != Done AND watcher = currentUser() AND updatedDate > startOfDay(%s)", jiraStartDay),
							}),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus(taskGetAllIssuesWithMentions, core.TaskStatusSuccess),
					Reaction: func(ev core.Event, state core.State) []core.Task {
						list := &list.ListReturns{}
						err := getTaskOutputTo(taskGetAllIssuesWithMentions, state, list)
						if err != nil {
							return []core.Task{}
						}
						message := strings.Builder{}
						if len(list.Issues) == 0 {
							return []core.Task{}
						}
						for _, issue := range list.Issues {
							if issue.Key != nil {
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
					Condition: core.ConditionTaskFinishedWithStatus(taskGetAllIssuesWithMentions, core.TaskStatusSuccess),
					Reaction: func(ev core.Event, state core.State) []core.Task {
						tasks := []core.Task{}
						list := &list.ListReturns{}
						err := getTaskOutputTo(taskGetAllIssuesWithMentions, state, list)
						if err != nil {
							return tasks
						}
						for _, issue := range list.Issues {
							task := buildTrelloAddCardTask(&buildTrelloAddCardTaskOptions{
								taskName:              fmt.Sprintf("Create card for issue %s", *issue.ID),
								trelloAPIToken:        trelloAPIToken,
								trelloAppID:           trelloAppID,
								trelloBoardID:         trelloBoardID,
								trelloCardDescription: fmt.Sprintf("Added by open-integration pipeline at %s\nLink: %s/browse/%s\nReason: I was mentioned.\nDescription: %s", now(), jiraEndpoint, *issue.Key, issue.Fields["description"].(string)),
								trelloCardName:        fmt.Sprintf("Follow up with issue %s", *issue.Key),
								trelloLebelIDs:        []string{trelloLebelIDs},
								trelloListID:          trelloListID,
							})
							tasks = append(tasks, task)
						}
						return tasks
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus(taskGetAllIssuesWhereIamWatcher, core.TaskStatusSuccess),
					Reaction: func(ev core.Event, state core.State) []core.Task {
						list := &list.ListReturns{}
						err := getTaskOutputTo(taskGetAllIssuesWhereIamWatcher, state, list)
						if err != nil {
							return []core.Task{}
						}
						message := strings.Builder{}
						if len(list.Issues) == 0 {
							return []core.Task{}
						}
						for _, issue := range list.Issues {
							if issue.Key != nil {
								message.WriteString(fmt.Sprintf("An issue I am watching was updated %s/browse/%s \n", jiraEndpoint, *issue.Key))
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
					Condition: core.ConditionTaskFinishedWithStatus(taskGetAllIssuesWhereIamWatcher, core.TaskStatusSuccess),
					Reaction: func(ev core.Event, state core.State) []core.Task {
						tasks := []core.Task{}
						list := &list.ListReturns{}
						err := getTaskOutputTo(taskGetAllIssuesWhereIamWatcher, state, list)
						if err != nil {
							return tasks
						}
						for _, issue := range list.Issues {
							task := buildTrelloAddCardTask(&buildTrelloAddCardTaskOptions{
								taskName:              fmt.Sprintf("Create card for issue %s", *issue.ID),
								trelloAPIToken:        trelloAPIToken,
								trelloAppID:           trelloAppID,
								trelloBoardID:         trelloBoardID,
								trelloCardDescription: fmt.Sprintf("Added by open-integration pipeline at %s\nLink: %s/browse/%s\nReason: watching this issue.\nDescription: %s", now(), jiraEndpoint, *issue.Key, issue.Fields["description"].(string)),
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
		fmt.Printf("%s is required and not set, exiting", name)
		os.Exit(1)
	}
	return val
}

func buildJiraTask(options *buildJiraTaskOptions) core.Task {
	return core.Task{
		Metadata: core.TaskMetadata{
			Name: options.taskName,
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
					Value: options.jql,
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

func getTaskOutputTo(task string, state core.State, target interface{}) error {
	output := ""
	for _, t := range state.Tasks {
		if t.Task == task {
			output = t.Output
		}
	}
	return json.Unmarshal([]byte(output), target)
}
