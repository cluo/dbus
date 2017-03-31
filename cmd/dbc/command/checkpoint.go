package command

import (
	"flag"
	"fmt"
	"strings"

	"github.com/funkygao/columnize"
	czk "github.com/funkygao/dbus/pkg/checkpoint/store/zk"
	"github.com/funkygao/gafka/ctx"
	"github.com/funkygao/gafka/zk"
	"github.com/funkygao/gocli"
)

type Checkpoint struct {
	Ui  cli.Ui
	Cmd string
}

func (this *Checkpoint) Run(args []string) (exitCode int) {
	var zone string
	cmdFlags := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	cmdFlags.Usage = func() { this.Ui.Output(this.Help()) }
	cmdFlags.StringVar(&zone, "z", ctx.ZkDefaultZone(), "")
	if err := cmdFlags.Parse(args); err != nil {
		return 1
	}

	zkzone := zk.NewZkZone(zk.DefaultConfig(zone, ctx.ZoneZkAddrs(zone)))
	mgr := czk.NewManager(zkzone)
	states, err := mgr.AllStates()
	if err != nil {
		this.Ui.Error(err.Error())
		return 2
	}

	lines := []string{"Scheme|DSN|Position"}
	for _, state := range states {
		lines = append(lines, fmt.Sprintf("%s|%s|%s", state.Scheme(), state.DSN(), state.String()))
	}

	if len(lines) > 1 {
		this.Ui.Output(columnize.SimpleFormat(lines))
	}

	return
}

func (*Checkpoint) Synopsis() string {
	return "Manages cluster checkpoint"
}

func (this *Checkpoint) Help() string {
	help := fmt.Sprintf(`
Usage: %s checkpoint [options]

    %s

Options:

    -z zone

`, this.Cmd, this.Synopsis())
	return strings.TrimSpace(help)
}