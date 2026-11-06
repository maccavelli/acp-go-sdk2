package acp_test

import (
	"fmt"
	"slices"

	acp "github.com/maccavelli/acp-go-sdk2"
)

// ExampleAuthenticateMethodIDs picks the id to authenticate with from the
// agent's initialize answer: a configured pin when the answer accepts it, and
// otherwise the first id it does accept. The SDK refuses any other id before
// sending it, a terminal method's included
// (0050-MADR-auth-method-ids-and-the-prompt-result-for-issue-14.md, D6).
func ExampleAuthenticateMethodIDs() {
	answer := acp.InitializeResponse{AuthMethods: []acp.AuthMethod{
		acp.NewAuthMethodAgent("openai", "OpenAI"),
		acp.NewAuthMethodAgent("grok", "Grok"),
		acp.NewAuthMethodTerminal("grok-login", "Sign in to Grok"),
	}}
	ids := acp.AuthenticateMethodIDs(answer.AuthMethods)
	fmt.Println(ids)

	pick := func(pin acp.AuthMethodId) acp.AuthMethodId {
		if slices.Contains(ids, pin) || len(ids) == 0 {
			return pin
		}
		return ids[0]
	}
	fmt.Println(pick("grok"))
	fmt.Println(pick("grok-login"))
	// Output:
	// [openai grok]
	// grok
	// openai
}
