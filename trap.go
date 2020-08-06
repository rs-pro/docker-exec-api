package dea

import (
	"gopkg.in/alessio/shellescape.v1"
)

// BashTrapShellScript is used to wrap a shell script in a trap that makes sure the script always exits
// with exit code of 0 this can be useful in container environments where exiting with an exit code different from 0
// would kill the container.
// At the same time it writes to a file the actual exit code of the script as well as the filename
// of the script as json.
const bashTrapShellScript = `runner_script_trap() {
	exit_code=$?
	printf "dea-command-done||$exit_code"
	exit 0
}

trap runner_script_trap EXIT
`

const bashPre = "set -eo pipefail\n"

const deaPrefix = "docker-exec-api-command-done"

func PrepareCommand(cmd string) string {
	return "echo " + shellescape.Quote(cmd) + "\n" + cmd + "\nprintf \"" + deaPrefix + "||$?\\n\"\n"
}
