package inputbox

import (
	"os/exec"
	"strings"
)

// InputBox displays a dialog box, returning the entered value and a bool for success
func InputBox(title, message, defaultAnswer string) (string, bool) {
	for {
		cmd := exec.Command(
			"zenity", "--forms",
			"--title", title,
			"--text", message,
			"--add-entry=Your input",
			"--cancel-label=Snooze for 4hrs",
		)

		out, err := cmd.Output()
		if err != nil {
			// Check if the error is because the user pressed the cancel button
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() == 1 {
					// Cancel (or your custom cancel label) was clicked
					return "", false
				}
			}
			// Other error
			return "", false
		}
		if strings.TrimSpace(string(out)) != "" {
			return strings.TrimSpace(string(out)), true
		}

		message = "Input cannot be empty. Please enter a value:"
	}
}
