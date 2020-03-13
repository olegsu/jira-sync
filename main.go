package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/open-integration/core"
	"github.com/open-integration/core/pkg/state"
	"github.com/open-integration/core/pkg/task"
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
		taskName string
		url      string
		message  string
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
					Reaction: func(ev state.Event, state state.State) []task.Task {
						return []task.Task{
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
					Condition: core.ConditionTaskFinishedWithStatus(taskGetAllIssuesWithMentions, state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						list := &list.ListReturns{}
						err := getTaskOutputTo(taskGetAllIssuesWithMentions, state, list)
						if err != nil {
							return []task.Task{}
						}
						if len(list.Issues) == 0 {
							return []task.Task{}
						}
						message := strings.Builder{}
						for _, issue := range list.Issues {
							if issue.Key != nil {
								message.WriteString(fmt.Sprintf("An issue i was mentioned in was updated %s/browse/%s \n", jiraEndpoint, *issue.Key))
							}
						}
						return []task.Task{
							buildSlackTask(&buildSlackTaskOptions{
								taskName: fmt.Sprintf("Send message as reaction to %s finished", taskGetAllIssuesWithMentions),
								url:      slackURL,
								message:  message.String(),
							}),
						}
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus(taskGetAllIssuesWithMentions, state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						tasks := []task.Task{}
						list := &list.ListReturns{}
						err := getTaskOutputTo(taskGetAllIssuesWithMentions, state, list)
						if err != nil {
							return tasks
						}
						for _, issue := range list.Issues {
							description, ok := issue.Fields["description"].(string)
							trelloCardDescriptionBuilder := strings.Builder{}
							trelloCardDescriptionBuilder.WriteString(fmt.Sprintf("Added by open-integration pipeline at %s\n", now()))
							trelloCardDescriptionBuilder.WriteString(fmt.Sprintf("Link: %s/browse/%s\n", jiraEndpoint, *issue.Key))
							trelloCardDescriptionBuilder.WriteString("Reason: I was mentioned.\n")
							if ok {
								trelloCardDescriptionBuilder.WriteString(fmt.Sprintf("Description: %s", description))
							}
							task := buildTrelloAddCardTask(&buildTrelloAddCardTaskOptions{
								taskName:              fmt.Sprintf("[Mentioned] Create card for issue %s", *issue.ID),
								trelloAPIToken:        trelloAPIToken,
								trelloAppID:           trelloAppID,
								trelloBoardID:         trelloBoardID,
								trelloCardDescription: trelloCardDescriptionBuilder.String(),
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
					Condition: core.ConditionTaskFinishedWithStatus(taskGetAllIssuesWhereIamWatcher, state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						list := &list.ListReturns{}
						err := getTaskOutputTo(taskGetAllIssuesWhereIamWatcher, state, list)
						if err != nil {
							return []task.Task{}
						}
						if len(list.Issues) == 0 {
							return []task.Task{}
						}
						tasks := []task.Task{}
						for i, issue := range list.Issues {
							if issue.Key != nil {
								message := strings.Builder{}
								message.WriteString(fmt.Sprintf("An issue I am watching was updated %s/browse/%s --- %d \n", jiraEndpoint, *issue.Key, i))
								tasks = append(tasks, buildSlackTask(&buildSlackTaskOptions{
									taskName: fmt.Sprintf("Send message as reaction to %s finished", taskGetAllIssuesWhereIamWatcher),
									url:      slackURL,
									message:  message.String(),
								}))
							}
						}
						return tasks
					},
				},
				core.EventReaction{
					Condition: core.ConditionTaskFinishedWithStatus(taskGetAllIssuesWhereIamWatcher, state.TaskStatusSuccess),
					Reaction: func(ev state.Event, state state.State) []task.Task {
						tasks := []task.Task{}
						list := &list.ListReturns{}
						err := getTaskOutputTo(taskGetAllIssuesWhereIamWatcher, state, list)
						if err != nil {
							return tasks
						}
						for _, issue := range list.Issues {
							description, ok := issue.Fields["description"].(string)
							trelloCardDescriptionBuilder := strings.Builder{}
							trelloCardDescriptionBuilder.WriteString(fmt.Sprintf("Added by open-integration pipeline at %s\n", now()))
							trelloCardDescriptionBuilder.WriteString(fmt.Sprintf("Link: %s/browse/%s\n", jiraEndpoint, *issue.Key))
							trelloCardDescriptionBuilder.WriteString("watching this issue.\n")
							if ok {
								trelloCardDescriptionBuilder.WriteString(fmt.Sprintf("Description: %s", description))
							}
							task := buildTrelloAddCardTask(&buildTrelloAddCardTaskOptions{
								taskName:              fmt.Sprintf("[Watching] Create card for issue %s", *issue.ID),
								trelloAPIToken:        trelloAPIToken,
								trelloAppID:           trelloAppID,
								trelloBoardID:         trelloBoardID,
								trelloCardDescription: trelloCardDescriptionBuilder.String(),
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

func buildJiraTask(options *buildJiraTaskOptions) task.Task {
	return task.Task{
		Metadata: task.Metadata{
			Name: options.taskName,
		},
		Spec: task.Spec{
			Service:  "jira",
			Endpoint: "list",
			Arguments: []task.Argument{
				task.Argument{
					Key:   "API_Token",
					Value: options.token,
				},
				task.Argument{
					Key:   "Endpoint",
					Value: options.endpoint,
				},
				task.Argument{
					Key:   "User",
					Value: options.user,
				},
				task.Argument{
					Key:   "JQL",
					Value: options.jql,
				},
				task.Argument{
					Key:   "QueryFields",
					Value: "*all",
				},
			},
		},
	}
}

func buildSlackTask(options *buildSlackTaskOptions) task.Task {
	return task.Task{
		Metadata: task.Metadata{
			Name: options.taskName,
		},
		Spec: task.Spec{
			Service:  "slack",
			Endpoint: "message",
			Arguments: []task.Argument{
				task.Argument{
					Key:   "Webhook_URL",
					Value: options.url,
				},
				task.Argument{
					Key:   "Message",
					Value: options.message,
				},
			},
		},
	}
}

func buildTrelloAddCardTask(options *buildTrelloAddCardTaskOptions) task.Task {
	task := task.Task{
		Metadata: task.Metadata{
			Name: options.taskName,
		},
		Spec: task.Spec{
			Endpoint: "addcard",
			Service:  "trello",
			Arguments: []task.Argument{
				task.Argument{
					Key:   "App",
					Value: options.trelloAppID,
				},
				task.Argument{
					Key:   "Token",
					Value: options.trelloAPIToken,
				},
				task.Argument{
					Key:   "Board",
					Value: options.trelloBoardID,
				},
				task.Argument{
					Key:   "List",
					Value: options.trelloListID,
				},
				task.Argument{
					Key:   "Name",
					Value: options.trelloCardName,
				},
				task.Argument{
					Key:   "Description",
					Value: options.trelloCardDescription,
				},
				task.Argument{
					Key:   "Labels",
					Value: options.trelloLebelIDs,
				},
			},
		},
	}
	return task
}

func getTaskOutputTo(task string, state state.State, target interface{}) error {
	output := ""
	for _, t := range state.Tasks() {
		if t.Task.Metadata.Name == task {
			output = t.Output
		}
	}
	return json.Unmarshal([]byte(output), target)
}
