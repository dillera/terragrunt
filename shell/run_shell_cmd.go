package shell

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"reflect"
	"strings"

	"github.com/gruntwork-io/terragrunt/errors"
	"github.com/gruntwork-io/terragrunt/options"
)

// Run the specified shell command with the specified arguments. Connect the command's stdin, stdout, and stderr to
// the currently running app.
func RunShellCommand(terragruntOptions *options.TerragruntOptions, command string, args ...string) error {
	terragruntOptions.Logger.Printf("Running command: %s %s", command, strings.Join(args, " "))

	cmd := exec.Command(command, args...)

	// TODO: consider adding prefix from terragruntOptions logger to stdout and stderr
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Terragrunt can run some commands (such as terraform remote config) before running the actual terraform
	// command requested by the user. The output of these other commands should not end up on stdout as this
	// breaks scripts relying on terraform's output.
	if !reflect.DeepEqual(terragruntOptions.TerraformCliArgs, args) {
		cmd.Stdout = os.Stderr
	}

	cmd.Dir = terragruntOptions.WorkingDir

	signalChannel := NewSignalsForwarder(forwardSignals, cmd.Process, terragruntOptions.Logger)
	defer signalChannel.Close()

	return errors.WithStackTrace(cmd.Run())
}

type SignalsForwarder chan os.Signal

func NewSignalsForwarder(signals []os.Signal, p *os.Process, logger *log.Logger) SignalsForwarder {
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, signals...)

	go func() {
		for {
			select {
			case s := <-signalChannel:
				if s == nil {
					return
				}
				logger.Printf("Forward signal %s to terraform.", s.String())
				err := p.Signal(s)
				if err != nil {
					logger.Printf("Error forwarding signal: %v", err)
				}
			}
		}
	}()

	return signalChannel
}

func (signalChannel *SignalsForwarder) Close() error {
	signal.Stop(*signalChannel)
	*signalChannel <- nil
	close(*signalChannel)
	return nil
}
