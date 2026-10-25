package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/glitchfl/lowkey/internal/platform"
)

type options struct {
	command string
	record  bool
	mode    string
}

const usage = `usage: lowkey [doctor | me [--mode <mode>]] [--record]

  (no command)  follow the game and show lobby tables until the window closes
  doctor        check this PC in seconds - exits non-zero on any failure
  me            print your own row and session line once
  --record      also save every Riot response, scrubbed, into a folder next to the exe
`

// Run is lowkey's single entry point
func Run(ctx context.Context, env Env, args []string) int {
	env = env.withDefaults()
	opts, err := parseArgs(args)
	if err != nil {
		fmt.Fprintf(env.Console.Out, "%v\n\n%s", err, usage)
		return 2
	}
	setup := platform.Prepare()
	switch opts.command {
	case "help":
		fmt.Fprint(env.Console.Out, usage)
		return 0
	case "version":
		fmt.Fprintf(env.Console.Out, "lowkey %s\n", env.Version)
		return 0
	case "doctor":
		return runDoctor(ctx, env, opts, setup)
	case "me":
		return runMe(ctx, env, opts)
	}
	fmt.Fprintf(env.Console.Out, "%q isn't built yet\n", opts.command)
	return 2
}

func parseArgs(args []string) (options, error) {
	var o options
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--record" || a == "-record":
			o.record = true
		case a == "-h" || a == "--help" || a == "help":
			o.command = "help"
		case a == "--version" || a == "version":
			o.command = "version"
		case a == "doctor" || a == "me":
			if o.command != "" && o.command != a {
				return o, fmt.Errorf("pick one command, not %s and %s", o.command, a)
			}
			o.command = a
		case strings.HasPrefix(a, "--mode="):
			o.mode = strings.TrimPrefix(a, "--mode=")
		case a == "--mode":
			if i+1 >= len(args) {
				return o, fmt.Errorf("--mode needs a mode")
			}
			i++
			o.mode = args[i]
		default:
			return o, fmt.Errorf("unknown argument %q", a)
		}
	}
	return o, nil
}
