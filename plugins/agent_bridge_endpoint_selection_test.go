// SPDX-FileCopyrightText: Copyright (c) 2025 Cisco and/or its affiliates.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/kelindar/search"
	"github.com/stretchr/testify/assert"
)

func initForTests() error {

	jsonFile, err := os.Open("./testdata/config_for_testing.json")
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Successfully Opened users.json")
	defer jsonFile.Close()
	pluginConfigForTest, _ := io.ReadAll(jsonFile)

	if err := json.Unmarshal(pluginConfigForTest, &pluginConfig); err != nil {
		return fmt.Errorf("conversion error for pluginConfig: %s", err)
	}

	for apiID, pluginDataConfig := range pluginConfig {
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

		if err := initSelectOperations(apiID, pluginDataConfig); err != nil {
			return fmt.Errorf("can't init operations for testing: %s", err)
		}
	}
	return nil
}

func TestEndpointSelection(t *testing.T) {
	err := initForTests()
	assert.Nil(t, err)

	tests := []struct {
		targetApiID       string
		query             string
		expectedOperation string
		reachThreshold    bool
	}{
		{
			"tyk-github-id",
			"Give me the list of pull requests for repository",
			"pulls/list",
			true,
		},
		{
			"tyk-github-id",
			"Give me the 5 last issues on repo tyk owned by TykTechnologies.",
			"issues/list-for-repo",
			true,
		},
		{
			"tyk-github-id",
			"Give me the 5 last issues on repo apiclarity owned by thelasttoto",
			"issues/list-for-repo",
			true,
		},
		{
			"tyk-github-id",
			"Give me the 5 last commits on repo tyk owned by TykTechnologies.",
			"repos/list-commits",
			true,
		},
		{
			"tyk-github-id",
			"Give me the 5 last commits on repo apiclarity owned by thelasttoto",
			"repos/list-commits",
			true,
		},
		{
			"tyk-github-id",
			"Create a bug ",
			"issues/create",
			true,
		},
		{
			"tyk-github-id",
			"Create a bug in the repo 'thelasttoto/apiclarity'",
			"issues/create",
			true,
		},
		{
			"tyk-github-id",
			"Create a bug in the repo 'thelasttoto/apiclarity' about apiclarity crashing when compiled for linux, and assign it to user thelasttoto",
			"issues/create",
			true,
		},
		{
			"tyk-github-id",
			"Donnes moi les 5 derniers commits du repo apiclarity de thelasttoto",
			"repos/list-commits",
			true,
		},
		{
			"tyk-github-id",
			"Donnes moi les 5 derniers problemes du repo apiclarity de thelasttoto",
			"issues/list-for-repo",
			true,
		},
		/*{
			"tyk-jira-id",
			"what is the last issues of project PUCCINI",
			"getRecent",
			true,
		},
		{
			"tyk-jira-id",
			"I want to create an issue on project PUCCINI with description 'this is a test issue'",
			"createIssue",
			true,
		},*/
		{
			"tyk-gmail-id",
			"I want to create a new draft with label 'DRAFT', subject 'test' and body 'this is a test'",
			"gmail.users.drafts.create",
			true,
		},
		{
			"tyk-gmail-id",
			"what is the details of draft with id '123456789'",
			"gmail.users.drafts.get",
			true,
		},
		{
			"tyk-gmail-id",
			"show me content of draft with id '123456789'",
			"gmail.users.drafts.get",
			true,
		},
		{
			"tyk-gmail-id",
			"delete draft with id '123456789'",
			"gmail.users.drafts.delete",
			true,
		},
		{
			"tyk-gmail-id",
			"delete draft with id '123456789' from my mailbox",
			"gmail.users.drafts.delete",
			true,
		},
		{
			"tyk-gmail-id",
			"give me the list of my drafts currently on lmy mailbox",
			"gmail.users.drafts.list",
			true,
		},
		{
			"tyk-gmail-id",
			"show me all my drafts",
			"gmail.users.drafts.list",
			true,
		},
		{
			"tyk-gmail-id",
			"take my draft '13456789' and send it to 'john.smith@unkdomain.com'",
			"gmail.users.drafts.send",
			true,
		},
		{
			"tyk-gmail-id",
			"send the draft with id '13456789' to 'john.smith@unkdomain.com'",
			"gmail.users.drafts.send",
			true,
		},
		{
			"tyk-gmail-id",
			"create a new label with name 'PERSONAL'",
			"gmail.users.labels.create",
			true,
		},
		{
			"tyk-gmail-id",
			"create a new label with name 'PERSONAL' on my mailbox",
			"gmail.users.labels.create",
			true,
		},
		{
			"tyk-gmail-id",
			"add a label 'TEST' on my mailbox",
			"gmail.users.labels.create",
			true,
		},
		{
			"tyk-gmail-id",
			"add a new user label 'TEST' on my mailbox",
			"gmail.users.labels.create",
			true,
		},
		{
			"tyk-gmail-id",
			"delete label 'TEST' from my mailbox",
			"gmail.users.labels.delete",
			true,
		},
		{
			"tyk-gmail-id",
			"give me the list of all labels presents on my mailbox",
			"gmail.users.labels.list",
			true,
		},
		{
			"tyk-gmail-id",
			"change the name of the label 'TEST' to 'DRAFT'",
			"gmail.users.labels.update",
			true,
		},
		{
			"tyk-gmail-id",
			"update name of the label with ID 'TEST' to 'DRAFT'",
			"gmail.users.labels.update",
			true,
		},
		{
			"tyk-gmail-id",
			"update visibility of label 'TEST' to 'labelHide'",
			"gmail.users.labels.update",
			true,
		},
		{
			"tyk-gmail-id",
			"delete the message about 'test' from my mailbox",
			"gmail.users.messages.delete",
			true,
		},
		{
			"tyk-gmail-id",
			"delete the message '123456789'' from my mailbox",
			"gmail.users.messages.delete",
			true,
		},
		{
			"tyk-gmail-id",
			"I want to send an email to 'john.smith@unkdomain.com' with subject 'test' and body 'Hello my friends. I receive your message, I'm agree with your opinion, we will fix that ASAP. Best Regards'",
			"gmail.users.messages.send",
			true,
		},
		{
			"tyk-gmail-id",
			"Send an email to 'john.smith@unkdomain.com' with subject 'test' and body 'Hello my friends. I receive your message, I'm agree with your opinion, we will fix that ASAP. Best Regards'",
			"gmail.users.messages.send",
			true,
		},
		{
			"tyk-gmail-id",
			"Please, send an email to 'john.smith@unkdomain.com' with subject 'test' and body 'Hello my friends. I receive your message, I'm agree with your opinion, we will fix that ASAP. Best Regards'",
			"gmail.users.messages.send",
			true,
		},
		{
			"tyk-gmail-id",
			"I want to send a message to 'john.smith@unkdomain.com' with subject 'test' and body 'Hello my friends. I receive your message, I'm agree with your opinion, we will fix that ASAP. Best Regards'",
			"gmail.users.messages.send",
			true,
		},
		{
			"tyk-gmail-id",
			"write an email to 'john.smith@unkdomain.com' with subject 'test' to agree with the schedule of the meeting.",
			"gmail.users.messages.send",
			true,
		},
		{
			"tyk-gmail-id",
			"send an email to 'john.smith@unkdomain.com' with subject 'test' to agree with the schedule of the meeting.",
			"gmail.users.messages.send",
			true,
		},
		{
			"tyk-gmail-id",
			"give me the 5 last messages on my mailbox",
			"gmail.users.messages.list",
			true,
		},
		{
			"tyk-gmail-id",
			"delete the message with id '123456789'",
			"gmail.users.messages.delete",
			true,
		},
		{
			"tyk-gmail-id",
			"delete the email with id '123456789' from my mailbox",
			"gmail.users.messages.delete",
			true,
		},
		{
			"tyk-gmail-id",
			"give me details of message '123456789' from my mailbox",
			"gmail.users.messages.get",
			true,
		},
		{
			"tyk-gmail-id",
			"move message '123456789' to the trash",
			"gmail.users.messages.trash",
			true,
		},
		{
			"tyk-gmail-id",
			"I want to move the message '123456789' to the trash",
			"gmail.users.messages.trash",
			true,
		},
		{
			"tyk-gmail-id",
			"remove the message '123456789' from the trash",
			"gmail.users.messages.untrash",
			true,
		},
		{
			"tyk-gmail-id",
			"I want to get the attachment from message '123456789'",
			"gmail.users.messages.attachments.get",
			true,
		},
		{
			"tyk-gmail-id",
			"show me my message '123456789'",
			"gmail.users.messages.get",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			matchingOperation, matchingScore, err := findSelectOperation(tt.targetApiID, tt.query)

			assert.Nil(t, err)
			assert.Equal(t, tt.expectedOperation, *matchingOperation)
			assert.Equal(t, tt.reachThreshold, (matchingScore >= RELEVANCE_THRESHOLD))
		})
	}

}
