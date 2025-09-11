package inputbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bhendo/go-powershell/backend"
	"github.com/bhendo/go-powershell/utils"
	"github.com/juju/errors"
	"golang.org/x/sys/windows/registry"
)

const newline = "\r\n"

type Shell interface {
	Execute(cmd string) (string, string, error)
	Exit()
}

type shell struct {
	handle backend.Waiter
	stdin  io.Writer
	stdout io.Reader
	stderr io.Reader
}

func New(backend backend.Starter) (Shell, error) {
	psAbsPath, psErr := findPowerShell()
	if psErr != nil {
		return nil, psErr
	}
	handle, stdin, stdout, stderr, err := backend.StartProcess(psAbsPath, "-NoExit", "-Command", "-")
	if err != nil {
		return nil, err
	}

	return &shell{handle, stdin, stdout, stderr}, nil
}

func (s *shell) Execute(cmd string) (string, string, error) {
	if s.handle == nil {
		return "", "", errors.Annotate(errors.New(cmd), "Cannot execute commands on closed shells.")
	}

	outBoundary := createBoundary()
	errBoundary := createBoundary()

	// wrap the command in special markers so we know when to stop reading from the pipes
	full := fmt.Sprintf("%s; echo '%s'; [Console]::Error.WriteLine('%s')%s", cmd, outBoundary, errBoundary, newline)

	_, err := s.stdin.Write([]byte(full))
	if err != nil {
		return "", "", errors.Annotate(errors.Annotate(err, cmd), "Could not send PowerShell command")
	}

	// read stdout and stderr
	sout := ""
	serr := ""

	waiter := &sync.WaitGroup{}
	waiter.Add(2)

	go streamReader(s.stdout, outBoundary, &sout, waiter)
	go streamReader(s.stderr, errBoundary, &serr, waiter)

	waiter.Wait()

	if len(serr) > 0 {
		return sout, serr, errors.Annotate(errors.New(cmd), serr)
	}

	return sout, serr, nil
}

func (s *shell) Exit() {
	s.stdin.Write([]byte("exit" + newline))

	// if it's possible to close stdin, do so (some backends, like the local one,
	// do support it)
	closer, ok := s.stdin.(io.Closer)
	if ok {
		closer.Close()
	}

	s.handle.Wait()

	s.handle = nil
	s.stdin = nil
	s.stdout = nil
	s.stderr = nil
}

func streamReader(stream io.Reader, boundary string, buffer *string, signal *sync.WaitGroup) error {
	// read all output until we have found our boundary token
	output := ""
	bufsize := 64
	marker := boundary + newline

	for {
		buf := make([]byte, bufsize)
		read, err := stream.Read(buf)
		if err != nil {
			return err
		}

		output = output + string(buf[:read])

		if strings.HasSuffix(output, marker) {
			break
		}
	}

	*buffer = strings.TrimSuffix(output, marker)
	signal.Done()

	return nil
}

func createBoundary() string {
	return "$gorilla" + utils.CreateRandomString(12) + "$"
}

// verifyPowerShell checks that the given path launches successfully.
func verifyPowerShell(path string) error {
	maxAttempts := 10
	baseTimeout := 2 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		timeout := time.Duration(attempt) * baseTimeout
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		cmd := exec.CommandContext(ctx, path, "-Command", "$PSVersionTable.PSVersion")
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out

		err := cmd.Run()
		if err == nil && out.String() != "" {
			// successfully launched
			return nil
		}
	}

	return fmt.Errorf("failed to verify powershell at %s after %d attempts", path, maxAttempts)
}

// readPSInstallPath64 reads if powershell is available in the windows registry
func readPSInstallPath64() (string, error) {
	k, err := registry.OpenKey(
		registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\PowerShell\1`, registry.READ|registry.WOW64_64KEY)
	if err != nil {
		return "", fmt.Errorf("error opening registry key: %v", err)
	}
	defer k.Close()
	dir, _, err := k.GetStringValue("InstallPath")
	if err != nil || dir == "" {
		return "", fmt.Errorf("error getting InstallPath value: %v", err)
	}
	exe := filepath.Join(dir, "powershell.exe")
	if info, err := os.Stat(exe); err == nil && !info.IsDir() {
		return exe, nil
	}
	return "", os.ErrNotExist
}

// findPowerShell searches for powershell (5.1)
// verifying that the candidate is a valid executable.
func findPowerShell() (exe string, err error) {
	var p string
	var e error
	// 1) Try PATH first
	p, e = exec.LookPath("powershell.exe")
	if e == nil {
		if verifyPowerShell(p) == nil {
			return p, nil
		}
	}

	// 2) Try reading from registry
	p, e = readPSInstallPath64()
	if p != "" {
		if verifyPowerShell(p) == nil {
			return p, nil
		}
	}

	// 3) Try common install locations
	candidates := []string{
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
	}

	for _, c := range candidates {
		if _, statErr := os.Stat(c); statErr == nil {
			if verifyPowerShell(c) == nil {
				return c, nil
			}
		}
	}

	return "", errors.New("no valid PowerShell found")
}

// InputBox displays a dialog box, returning the entered value and a bool for success
func InputBox(title, message, defaultAnswer string) (string, bool) {
	for {
		message = message + " If you hit cancel button it will snooze for 4hrs."
		shell, err := New(&backend.Local{})
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
