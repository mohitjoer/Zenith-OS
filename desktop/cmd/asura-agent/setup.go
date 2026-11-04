package main

import (
	"fmt"
	"strings"
)

// setup.go is `setup`: it says where the agent engine is set and how, and changes
// nothing. The engine is a program the user supplies (engine.go), so there is
// nothing to install, no account to create and no network to reach.

func setupText(cfg Config) string {
	st := engineStatus(cfg)
	now := strings.Join(st.Command, " ")
	if st.Reason != "" {
		now = st.Reason
	}
	return displayName + " runs the agent engine you point it at: any program that speaks\n" +
		"the Agent Client Protocol over stdio.\n\n" +
		"Set its command (" + st.Key + ") in " + st.Config + ":\n\n" +
		"  { \"engine\": { \"command\": [\"your-agent\", \"acp\"] } }\n\n" +
		"now: " + now + "\n\n" +
		"Run `" + binName + " status` to check it. Nothing is installed and no network is used.\n"
}

func cmdSetup() error {
	fmt.Print(setupText(LoadConfig()))
	return nil
}
