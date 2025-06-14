package inputbox

import (
	"strings"

	ps "github.com/bhendo/go-powershell"
	"github.com/bhendo/go-powershell/backend"
)

// InputBox displays a dialog box, returning the entered value and a bool for success
func InputBox(title, message, defaultAnswer string) (string, bool) {
	for {
		message = message + " If you hit cancel button it will snooz for 4hrs."
		shell, err := ps.New(&backend.Local{})
		if err != nil {
			panic(err)
		}
		defer shell.Exit()

		out, _, err := shell.Execute(`
			[void][Reflection.Assembly]::LoadWithPartialName('Microsoft.VisualBasic')
			$title = '` + title + `'
			$msg = '` + message + `'
			$default = '` + defaultAnswer + `'
			$answer = [Microsoft.VisualBasic.Interaction]::InputBox($msg, $title, $default)
			Write-Output $answer
			`)
		if err != nil {
			return "", false
		}
		if len(string(out)) == 2 {
			return "", false
		}
		result := out
		if len(string(out)) > 2 {
			result = strings.TrimSpace(string(out))
		}
		if result != "" {
			return result, true
		}
		message = "Input cannot be empty. Please enter a value."
	}
}
