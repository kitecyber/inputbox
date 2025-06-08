package inputbox

import (
	"fmt"
	"os/exec"
	"strings"
)

// InputBox displays a dialog box, returning the entered value and a bool for success
func InputBox(title, message, defaultAnswer string) (string, bool) {
	for {
		script := fmt.Sprintf(`set T to display dialog "%s" buttons {"Snooze for 4hrs", "OK"} default button "OK" with title "%s" default answer "%s"
		if button returned of T is "OK" then
			return "OK:" & text returned of T
		else
			return "SNOOZE:"
		end if`, message, title, defaultAnswer)

		out, err := exec.Command("osascript", "-e", script).Output()
		if err != nil {
			return "", false
		}

		result := strings.TrimSpace(string(out))
		if strings.HasPrefix(result, "OK:") {
			code := strings.TrimPrefix(result, "OK:")
			if code != "" {
				return code, true
			}
		} else if strings.HasPrefix(result, "SNOOZE:") {
			return "", false
		}

		message = "Input cannot be empty. Please enter a value."
	}
}
