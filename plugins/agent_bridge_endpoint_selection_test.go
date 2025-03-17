// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/kelindar/search"
	"github.com/stretchr/testify/assert"
)

const APIID_TO_TEST = "tyk-github-id"

func initForTests() error {
	pluginConfigForTest := []byte(`
{
  "tyk-github-id": {
    "azureConfig": {
      "openAIKey": "xxx",
      "openAIEndpoint": "",
      "modelDeployment": "gpt-4o-mini"
    },
    "selectOperations": {
      "gists/create-comment": {
        "x-nl-input-examples": [
          "Add a comment to my gist titled 'Python Utilities' with my feedback: 'The second line must be fixed'",
          "Create a comment on the gist with ID 20384fe099243335bc81c608ea89e1de: 'That is wonderfull'"
        ]
      },
      "gists/list-comments": {
        "x-nl-input-examples": [
          "List comments of GIST 20384fe099243335bc81c608ea89e1de",
          "What are the feedback I've received on my gist 20384fe099243335bc81c608ea89e1de ?",
          "Is there negative comment on my most recent gist ?"
        ]
      },
      "gists/list-public": {
        "x-nl-input-examples": [
          "Fetch the latest public gists and filter them by creation date",
          "I only want the first 10 entries of the 4th page. Only give me the descriptions in markdown, and translate them in french",
          "I'm looking to display only the file names of public gists from the second page",
          "Can you download the content of all public gists that include Python files ?",
          "I intend to read and review the titles of all public gists created this week",
          "I aim to access public gists and read the tags applied to each one"
        ]
      },
      "gists/list-starred": {
        "x-nl-input-examples": [
          "I want to see which gists I've starred recently and read their descriptions",
          "What are the gists that I prefer ?",
          "I need to access my starred gists on GitHub to share a specific one with a colleague",
          "Give me my starred gists about zig programming language"
        ]
      },
      "issues/add-assignees": {
        "x-nl-input-examples": [
          "Add to assignees the user #NAME# to the issue #NB#",
          "Add #NAME# to the assignees of the issue #NB#",
          "Add the user #NAME# to the assignees of the issue #NB#"
        ]
      },
      "issues/add-labels": {
        "x-nl-input-examples": [
          "Add label 'bug' to issue #NB of repository"
        ]
      },
      "issues/create": {
        "x-nl-input-examples": [
          "Create a bug in repo",
          "I want to say that there is a vulnerability issue",
          "Add an issue about",
          "Create an issue about"
        ]
      },
      "issues/get": {
        "x-nl-input-examples": [
          "Give me details for issue #NB",
          "show me details of issue #NB"
        ]
      },
      "issues/list-comments": {
        "x-nl-input-examples": [
          "List comments for issue #NB of repository",
          "List comments for issue #NB of repository with 50 results per page"
        ]
      },
      "issues/list-comments-for-repo": {
        "x-nl-input-examples": [
          "List comments for issue",
          "List comments for issue with 50 results per page"
        ]
      },
      "issues/list-for-repo": {
        "x-nl-input-examples": [
          "List the issues for the repository",
          "Give issues in the repository to address any pending items",
          "Issues that are already closed",
          "Give me the last issues on repo for",
          "Give me last issues on repo.",
          "Give me issues assigned to",
          "Give me issues list",
          "last 3 issues in the repo"
        ]
      },
      "issues/remove-assignees": {
        "x-nl-input-examples": [
          "delete the user #NAME# from assignee on the issue #NB#",
          "remove #NAME# from the assignees of the issue #NB#"
        ]
      },
      "issues/update": {
        "x-nl-input-examples": [
          "Update title of issue #ID to",
          "Assign issue #ID to",
          "Set title of issue #ID to ",
          "Add label to issue #ID",
          "Set state of issue #ID to open",
          "Set state of issue #ID to closed"
        ]
      },
      "pulls/create": {
        "x-nl-input-examples": [
          "Create a pull request for repository",
          "From branch, create a pull request to with title"
        ]
      },
      "pulls/create-review-comment": {
        "x-nl-input-examples": [
          "Create a comment for pull request #ID on repository NAME/OWNER",
          "Add a comment for pull request"
        ]
      },
      "pulls/get": {
        "x-nl-input-examples": [
          "Get the status of the pull request #ID on repository",
          "Give me details on pull request #ID",
          "return the id of the pull request #ID"
        ]
      },
      "pulls/list": {
        "x-nl-input-examples": [
          "Give me the list of pull requests for repository",
          "show me pull requests for repository"
        ]
      },
      "pulls/list-review-comments": {
        "x-nl-input-examples": [
          "Give me comments for pull requests #ID on repository NAME/OWNER",
          "Show me list of comments for the pull request #ID"
        ]
      },
      "pulls/update": {
        "x-nl-input-examples": [
          "Update the state of the pull request #ID on repository",
          "change the state of the pull request #ID to",
          "close the pull request #ID",
          "change the base of the pull request #ID to"
        ]
      },
      "repos/delete-release": {
        "x-nl-input-examples": [
          "Delete release #nb",
          "Remove last release from repository"
        ]
      },
      "repos/get-readme": {
        "x-nl-input-examples": [
          "Show me the README for repository owned by",
          "Give readme.md content in the repository"
        ]
      },
      "repos/get-release": {
        "x-nl-input-examples": [
          "Give me details for release #nb",
          "return details for release",
          "what about release #nb?"
        ]
      },
      "repos/list-commits": {
        "x-nl-input-examples": [
          "List the commits for repository",
          "Give commits in",
          "Commits created by author",
          "last 3 commits in",
          "last 3 commits in the repo created by author.",
          "Give commits in the repo created by author on branch",
          "last 3 commits in the repo",
          "Give me the last commits on repo for"
        ]
      },
      "repos/list-releases": {
        "x-nl-input-examples": [
          "List the releases for repository owned by",
          "Give releases in repository",
          "What are the list of releases for repository"
        ]
      },
      "repos/list-tags": {
        "x-nl-input-examples": [
          "List the tags for repository",
          "Give tags in the repository by"
        ]
      }
    },
    "selectModelEmbedding": "MiniLM-L6-v2.Q8_0.gguf",
    "selectModelsPath": "../tyk-release-v5.8.0-alpha8/models",
    "llmConfig": {
      "AzureConfig": {
        "openAIKey": "xxx",
        "modelDeployment": "gpt-4o-mini"
      }
    },
    "APIID": "tyk-github-id"
  }
  "tyk-jira-id": {
    "azureConfig": {
      "openAIKey": "62eccad2fad545ccb9e75db791089355",
      "openAIEndpoint": "https://smith-project-agents.openai.azure.com",
      "modelDeployment": "gpt-4o-mini"
    },
    "selectOperations": {
      "addUserToGroup": {
        "x-nl-input-examples": [
          "add user #ID# to group #ID#",
          "user #ID# is now part of group #ID#"
        ]
      },
      "assignIssue": {
        "x-nl-input-examples": [
          "assign issue #ID# to user #ID#",
          "add assignee #ID# to issue #ID#"
        ]
      },
      "copyDashboard": {
        "x-nl-input-examples": [
          "create a copy of dashboard #ID# with name #NAME#",
          "copy the dashboard #ID# and set name=#NAME# and description=#DESCRIPTION# to the new one",
          "duplicate dashboard #ID# and change name to #NAME"
        ]
      },
      "createComponent": {
        "x-nl-input-examples": [
          "create a component named #NAME# on project #ID# with description '#DESCRIPTION#'",
          "add a new component '#NAME#' on project #ID#"
        ]
      },
      "createCustomField": {
        "x-nl-input-examples": [
          "Create a custom field named #NAME# of type #TYPE# and description '#DESCRIPTION#'",
          "Create a custom field named 'Approvers' of type 'com.atlassian.jira.plugin.system.customfieldtypes:multiuserpicker' and description 'Contains users needed for approval. This custom field was created by Jira Service Desk.'",
          "Create a custom field named 'Change reason' of type 'com.atlassian.jira.plugin.system.customfieldtypes:select' and description 'Choose the reason for the change request'"
        ]
      },
      "createDashboard": {
        "x-nl-input-examples": [
          "create a new dashboard named '#NAMED#' with permissions '#PERMISSIONS#'",
          "create a global dashboard named '#NAMED#' "
        ]
      },
      "createFilter": {
        "x-nl-input-examples": [
          "Create a filter that lists all open bugs",
          "Create a filter that lists all my tasks, names '#NAME#'"
        ]
      },
      "createGroup": {
        "x-nl-input-examples": [
          "create a group of users named #NAMED#",
          "create users group #NAMED#"
        ]
      },
      "createIssue": {
        "x-nl-input-examples": [
          "create an issue with summary '#DESCRIPTION#' for project #ID# and assign it to #USER#",
          "add an issue on project #ID# with summary '#DESCRIPTION#'. It concerns component #COMPONENT#. Assign it to #USER#",
          "create a subtask for issue #ID# with summary '#DESCRIPTION#' and label #LABEL#. Assign it to #USER#",
          "add a subtask #DESCRIPTION# on task #ID# and assign it to #USER#",
          "add a subtask on task #ID# with summary '#DESCRIPTION#' and label #LABEL#"
        ]
      },
      "createIssueType": {
        "x-nl-input-examples": [
          "create a new issue type named #NAME# with description #DESCRIPTION#",
          "add issue type #NAME#"
        ]
      },
      "createPriority": {
        "x-nl-input-examples": [
          "Create a new priority"
        ]
      },
      "createProject": {
        "x-nl-input-examples": [
          "create a project named #NAME# and assign it to me",
          "add a new project #NAME with name #NAME# and description '#DESCRIPTION#'"
        ]
      },
      "createVersion": {
        "x-nl-input-examples": [
          "create a new version named #NAME# with description '#DESCRIPTION#' for project #ID#",
          "Add a new version for project #ID#, named #NAME# and with the descrition '#DESCRIPTION#'",
          "I would like to create a new version for project #ID# with the name #NAME# and the description '#DESCRIPTION#'",
          "add a new version named #NAME# for project #ID# with description '#DESCRIPTION'"
        ]
      },
      "deleteComponent": {
        "x-nl-input-examples": [
          "delete component #ID#",
          "remove component with id #ID#"
        ]
      },
      "deleteCustomField": {
        "x-nl-input-examples": [
          "delete the field #ID#",
          "delete the custom field #ID#",
          "remove the field #ID#",
          "remove the custom field #ID#"
        ]
      },
      "deleteDashboard": {
        "x-nl-input-examples": [
          "delete dashboard #ID#",
          "remove dashboard with id #ID#"
        ]
      },
      "deleteIssue": {
        "x-nl-input-examples": [
          "delete the issue #ID#",
          "remove issue #ID#",
          "I want to delete the issue #ID# from project"
        ]
      },
      "deleteIssueType": {
        "x-nl-input-examples": [
          "delete issue type #ID#",
          "remove issue type #ID# from project"
        ]
      },
      "deletePriority": {
        "x-nl-input-examples": [
          "delete the priority #ID#",
          "remove priority #ID#"
        ]
      },
      "deleteProject": {
        "x-nl-input-examples": [
          "delete project with id #ID#",
          "remove project #ID#"
        ]
      },
      "deleteProjectAsynchronously": {
        "x-nl-input-examples": [
          "delete project #ID#",
          "remove the project #ID#"
        ]
      },
      "deleteVersion": {
        "x-nl-input-examples": [
          "delete version #ID# of the project #ID#",
          "remove a version with #ID# of the project #ID#",
          "delete the version of the project #ID# with id #ID#"
        ]
      },
      "editIssue": {
        "x-nl-input-examples": [
          "update issue #ID# summary to",
          "update issue #ID# customfield_10000 to ",
          "change the field of issue #ID# to ",
          "edit issue #ID#. Change summary to ",
          "Add label #LABEL# on issue #ID#",
          "Set components of issue #ID# to ",
          "Components of issue #ID# are",
          "update the issue #ID#. component is",
          "assign the issue #ID# to user #USER#"
        ]
      },
      "findComponentsForProjects": {
        "x-nl-input-examples": [
          "give me components on project #ID#",
          "show me the list of components on project #ID#",
          "show me components from project #ID# in page #NB#, #NB# items per page",
          "show me components from project #ID# with name containing #NAME#",
          "give me conponents on page #NB# from project #ID#, order by name, #NB# items per page"
        ]
      },
      "getAllDashboards": {
        "x-nl-input-examples": [
          "give me the list of all my dashboards",
          "show me list of my dashboards"
        ]
      },
      "getAllLabels": {
        "x-nl-input-examples": [
          "show me the list of labels on project",
          "give me the list of labels, content of page #NB#, #NB items per page",
          "what is the list of current labels"
        ]
      },
      "getAllProjects": {
        "x-nl-input-examples": [
          "show me the list of available projects",
          "give me my list of projects",
          "give me my projects"
        ]
      },
      "getCommentsByIds": {
        "x-nl-input-examples": [
          "get comments with ids [1, 2, 5, 10]"
        ]
      },
      "getComponent": {
        "x-nl-input-examples": [
          "get component #ID#",
          "show me details of component #ID#"
        ]
      },
      "getCurrentUser": {
        "x-nl-input-examples": [
          "get me my details",
          "show details about me",
          "who am I"
        ]
      },
      "getDashboard": {
        "x-nl-input-examples": [
          "give me details of dashboard #ID#",
          "get dashboard with id #ID#",
          "show me dashboard #ID#"
        ]
      },
      "getDashboardsPaginated": {
        "x-nl-input-examples": [
          "give me the list of dashboards for group #ID#",
          "show me list of dashboards for project #ID#",
          "return the list of dashboards for account id #ID#",
          "search for the list of dashboard that match name #FILTER#"
        ]
      },
      "getEvents": {
        "x-nl-input-examples": [
          "give me the list of all issue events",
          "show me the list of all events",
          "get all issue events"
        ]
      },
      "getFields": {
        "x-nl-input-examples": [
          "get all my custom fields",
          "return the list of custom issue fields",
          "show me all custom fields",
          "give me the list of known custom fields"
        ]
      },
      "getIssue": {
        "x-nl-input-examples": [
          "give me issue #ID# details",
          "show me details for issue #ID#",
          "I want to see issue #ID#. Show me only name and description",
          "can you give me details for issue #ID#"
        ]
      },
      "getIssueAllTypes": {
        "x-nl-input-examples": [
          "get list of issue types for project",
          "give me the list of all issue types",
          "what is the list of current issue types created for the project"
        ]
      },
      "getIssueType": {
        "x-nl-input-examples": [
          "show me details of issue type #ID#",
          "give me details of issue type #ID#",
          "show me issue type #ID#"
        ]
      },
      "getMyPermissions": {
        "x-nl-input-examples": [
          "get all my permissions",
          "get all my permissions I have",
          "show me all my permissions",
          "give me the list of permissions I have on this project"
        ]
      },
      "getPermittedProjects": {
        "x-nl-input-examples": [
          "give me the list of projects reacheable",
          "give me the list of projects on which I work on",
          "show me the list of project with the permission"
        ]
      },
      "getPriorities": {
        "x-nl-input-examples": [
          "give me the list of priorities defined",
          "show me list of priorities actually defined",
          "list me the list of priorities for project"
        ]
      },
      "getProject": {
        "x-nl-input-examples": [
          "get details of project with id #ID#",
          "show me project #ID#",
          "give me details of project #ID#"
        ]
      },
      "getProjectComponents": {
        "x-nl-input-examples": [
          "give me the list of components of project #ID#",
          "what is the list of components on project #ID#?",
          "which components are included on project #ID#?",
          "show me the list of all components of the project #ID#",
          "show me all components of the project #ID#"
        ]
      },
      "getProjectVersions": {
        "x-nl-input-examples": [
          "give me the list of version for project #ID#",
          "what are the versions of the project #ID#",
          "show me all versions of project #ID#"
        ]
      },
      "getRecent": {
        "x-nl-input-examples": [
          "show me my last projects",
          "give me my list of my last projects",
          "show me the list of last projects viewed"
        ]
      },
      "getUsersFromGroup": {
        "x-nl-input-examples": [
          "give me the list of users of group #ID#",
          "what are the users of group #ID#",
          "give the users of group #ID#. return page #NB#, #NB# users per page, please",
          "list the 5 first users of group #ID#",
          "return users on page #NB# for group #ID#. Put #NB# items per page"
        ]
      },
      "getVersion": {
        "x-nl-input-examples": [
          "show me details of the version #ID# of the project #ID#",
          "give me the description of the version #ID# of the project #ID#",
          "on the project #ID#, give me details of the version #ID#",
          "give me the status of the version #ID# on the project #ID#",
          "what is the release date of the version #ID# of the project #ID#"
        ]
      },
      "movePriorities": {
        "x-nl-input-examples": [
          "move priority #ID# after #ID#",
          "move priority #ID# before #ID#"
        ]
      },
      "removeGroup": {
        "x-nl-input-examples": [
          "delete the group of users named #NAMED#",
          "delete users group #NAMED#",
          "remove the group #NAMED#"
        ]
      },
      "removeUserFromGroup": {
        "x-nl-input-examples": [
          "remove user #ID# from group #ID#",
          "user #ID# is not any more part of group #ID#"
        ]
      },
      "searchForIssuesUsingJql": {
        "x-nl-input-examples": [
          "give me issues for project #NAME#",
          "show me the list of open issues for project #NAME#",
          "I want to see issues for project #NAME#, with only name and description fields"
        ]
      },
      "searchProjects": {
        "x-nl-input-examples": [
          "show me projects with status #status#",
          "give me list of projects with ",
          "return project from page #NB#, with #NB# items per page"
        ]
      },
      "setDefaultPriority": {
        "x-nl-input-examples": [
          "set the default priority to #ID#"
        ]
      },
      "updateComponent": {
        "x-nl-input-examples": [
          "update name of component #ID# to '#NAME#'",
          "update description of component #ID# to '#DESCRIPTION#'",
          "update component #ID# and set description to '#DESCRIPTION#'",
          "update component #ID# with assignee type #TYPE#, description '#DESCRIPTION#', lead account id #ID# and name '#NAME#'"
        ]
      },
      "updateDashboard": {
        "x-nl-input-examples": [
          "update name of dashboard #ID# to #NAME#",
          "update dashboard #ID# with name=#NAME#, description=#DESCRIPTION# and type=global",
          "change type of dashboard #ID# to 'global'",
          "change name of dashboard #ID# to #NAME#"
        ]
      },
      "updateIssueType": {
        "x-nl-input-examples": [
          "update issue type #ID# with new name #NAME# and description #DESCRIPTION#",
          "update name of issue type #ID# to #NAME#",
          "update description of issue type #ID# to #description#"
        ]
      },
      "updateProject": {
        "x-nl-input-examples": [
          "update description of project #ID# to #description#",
          "update name of project #ID# to #NAME#",
          "change name and description of project #ID# to #NAME# and '#DESCRIPTION'"
        ]
      },
      "updateVersion": {
        "x-nl-input-examples": [
          "update name of version #ID# of the project #ID# to #NAME#",
          "change the description of the version #ID# of the project #ID# to '#DESCRIPTION#'",
          "update the release date of the version #ID# of the project #ID# to #DATE#",
          "change the status of the version #ID# of the project #ID# to #STATUS#"
        ]
      }
    },
    "selectModelEmbedding": "paraphrase-multilingual-mpnet-base-v2-q8_0.gguf",
    "selectModelsPath": "models",
    "llmConfig": {
      "AzureConfig": {
        "openAIKey": "62eccad2fad545ccb9e75db791089355",
        "openAIEndpoint": "https://smith-project-agents.openai.azure.com",
        "modelDeployment": "gpt-4o-mini"
      }
    },
    "APIID": "tyk-jira-id"
  }

}
`)
	if err := json.Unmarshal(pluginConfigForTest, &pluginConfig); err != nil {
		return fmt.Errorf("conversion error for pluginConfig: %s", err)
	}
	pluginDataConfig := pluginConfig[APIID_TO_TEST]
	pluginDataConfig.SelectModelEmbedding = DEFAULT_MODEL_EMBEDDINGS_MODEL

	if pluginDataConfig.AzureConfig.OpenAIKey == "" {
		return fmt.Errorf("Missing required config for azureConfig.openAIKey")
	}

	modelPath := filepath.Join(pluginDataConfig.SelectModelsPath, pluginDataConfig.SelectModelEmbedding)
	modelEmbedder, err := search.NewVectorizer(modelPath, 1)
	if err != nil {
		return fmt.Errorf("Unable to find embedding model %s: %s", pluginDataConfig.SelectModelEmbedding, err)
	}
	embeddingModels[pluginDataConfig.SelectModelEmbedding] = modelEmbedder

	if err := initSelectOperations(APIID_TO_TEST, pluginDataConfig); err != nil {
		return fmt.Errorf("can't init operations for testing: %s", err)
	}

	return nil
}

func TestEndpointSelection(t *testing.T) {
	err := initForTests()
	assert.Nil(t, err)

	tests := []struct {
		query             string
		expectedOperation string
		reachThreshold    bool
	}{
		{
			"Give me the list of pull requests for repository",
			"pulls/list",
			true,
		},
		{
			"Give me the 5 last issues on repo tyk owned by TykTechnologies.",
			"issues/list-for-repo",
			true,
		},
		{
			"Give me the 5 last issues on repo apiclarity owned by thelasttoto",
			"issues/list-for-repo",
			true,
		},
		{
			"Give me the 5 last commits on repo tyk owned by TykTechnologies.",
			"repos/list-commits",
			true,
		},
		{
			"Give me the 5 last commits on repo apiclarity owned by thelasttoto",
			"repos/list-commits",
			true,
		},
		{
			"Create a bug ",
			"issues/create",
			true,
		},
		{
			"Create a bug in the repo 'thelasttoto/apiclarity'",
			"issues/create",
			true,
		},
		{
			"Create a bug in the repo 'thelasttoto/apiclarity' about apiclarity crashing when compiled for linux, and assign it to user thelasttoto",
			"issues/create",
			true,
		},
		{
			"Donnes moi les 5 derniers commits du repo apiclarity de thelasttoto",
			"repos/list-commits",
			true,
		},
		{
			"Donnes moi les 5 derniers problemes du repo apiclarity de thelasttoto",
			"issues/list-for-repo",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			matchingOperation, matchingScore, err := findSelectOperation(APIID_TO_TEST, tt.query)

			assert.Nil(t, err)
			assert.Equal(t, tt.expectedOperation, *matchingOperation)
			assert.Equal(t, tt.reachThreshold, (matchingScore >= RELEVANCE_THRESHOLD))
		})
	}
}
