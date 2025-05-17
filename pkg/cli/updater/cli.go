// Copyright 2025 Outreach Corporation. All Rights Reserved.

// Description: CLI compatibility layer.

package updater

import (
	"context"

	cliV2 "github.com/urfave/cli/v2"
	cliV3 "github.com/urfave/cli/v3"
)

type Flag interface {
	// ToUrfaveV2 converts the flag to a urfave/cli/v2.Flag.
	ToUrfaveV2() cliV2.Flag
	// ToUrfaveV3 converts the flag to a urfave/cli/v3.Flag.
	ToUrfaveV3() cliV3.Flag
}

// BoolFlag is a CLI library-agnostic representation of a CLI
// boolean flag.
type BoolFlag struct {
	// The name of the flag, sans the leading double-hyphen.
	Name string
	// The short description of the flag.
	Usage string
}

func (f *BoolFlag) ToUrfaveV2() cliV2.Flag {
	return &cliV2.BoolFlag{
		Name:  f.Name,
		Usage: f.Usage,
	}
}

func (f *BoolFlag) ToUrfaveV3() cliV3.Flag {
	return &cliV3.BoolFlag{
		Name:  f.Name,
		Usage: f.Usage,
	}
}

// StringFlag is a CLI library-agnostic representation of a CLI
// string flag.
type StringFlag struct {
	// The name of the flag, sans the leading double-hyphen.
	Name string
	// The short description of the flag.
	Usage string
	// The default value of the flag.
	Value string
}

func (f *StringFlag) ToUrfaveV2() cliV2.Flag {
	return &cliV2.StringFlag{
		Name:  f.Name,
		Usage: f.Usage,
		Value: f.Value,
	}
}

func (f *StringFlag) ToUrfaveV3() cliV3.Flag {
	return &cliV3.StringFlag{
		Name:  f.Name,
		Usage: f.Usage,
		Value: f.Value,
	}
}

// CLIArgs is an interface for urfave/cli/v2.Args and urfave/cli/v3.Args.
type CLIArgs interface {
	First() string
	// Get returns the value of the argument at the given index.
	Get(index int) string
}

// CLICmd is an interface for urfave/cli/v2.Command and urfave/cli/v3.Command.
type CLICmd struct {
	V2 *cliV2.Context
	V3 *cliV3.Command
}

func CmdForV2(ctx *cliV2.Context) *CLICmd {
	return &CLICmd{
		V2: ctx,
	}
}
func CmdForV3(cmd *cliV3.Command) *CLICmd {
	return &CLICmd{
		V3: cmd,
	}
}

func (c *CLICmd) Args() CLIArgs {
	if c.V2 != nil {
		return c.V2.Args()
	}
	return c.V3.Args()
}

func (c *CLICmd) Bool(name string) bool {
	if c.V2 != nil {
		return c.V2.Bool(name)
	}
	return c.V3.Bool(name)
}

func (c *CLICmd) String(name string) string {
	if c.V2 != nil {
		return c.V2.String(name)
	}
	return c.V3.String(name)
}

type CommandAction func(ctx context.Context, c *CLICmd) error

// Command is a CLI library-agnostic representation of a CLI (sub)command.
type Command struct {
	Name     string
	Usage    string
	Commands []*Command
	Flags    []Flag
	Action   CommandAction
}

// ToUrfaveV2 converts the command to a urfave/cli/v2.Command.
func (c *Command) ToUrfaveV2() *cliV2.Command {
	cliCmd := cliV2.Command{
		Name:  c.Name,
		Usage: c.Usage,
	}
	for _, cmd := range c.Commands {
		cliCmd.Subcommands = append(cliCmd.Subcommands, cmd.ToUrfaveV2())
	}
	cliCmd.Action = func(ctx *cliV2.Context) error {
		return c.Action(ctx.Context, CmdForV2(ctx))
	}
	return &cliCmd
}

// ToUrfaveV3 converts the command to a urfave/cli/v3.Command.
func (c *Command) ToUrfaveV3() *cliV3.Command {
	cliCmd := cliV3.Command{
		Name:  c.Name,
		Usage: c.Usage,
	}
	for _, cmd := range c.Commands {
		cliCmd.Commands = append(cliCmd.Commands, cmd.ToUrfaveV3())
	}
	cliCmd.Action = func(ctx context.Context, _ *cliV3.Command) error {
		return c.Action(ctx, CmdForV3(&cliCmd))
	}
	return &cliCmd
}
