package flags

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func manQuote(s string) string {
	return strings.Replace(s, "\\", "\\\\", -1)
}

func formatForMan(wr io.Writer, s string) {
	for {
		idx := strings.IndexRune(s, '`')

		if idx < 0 {
			fmt.Fprintf(wr, "%s", manQuote(s))
			break
		}

		fmt.Fprintf(wr, "%s", manQuote(s[:idx]))

		s = s[idx+1:]
		idx = strings.IndexRune(s, '\'')

		if idx < 0 {
			fmt.Fprintf(wr, "%s", manQuote(s))
			break
		}

		fmt.Fprintf(wr, "\\fB%s\\fP", manQuote(s[:idx]))
		s = s[idx+1:]
	}
}

func writeManPageOptions(wr io.Writer, grp *Group) {
	grp.eachGroup(func(group *Group) {
		if !group.showInHelp() {
			return
		}

		// If the parent (grp) has any subgroups, display their descriptions as
		// subsection headers similar to the output of --help.
		if group.ShortDescription != "" && len(grp.groups) > 0 {
			fmt.Fprintf(wr, ".SS %s\n", group.ShortDescription)

			if group.LongDescription != "" {
				formatForMan(wr, group.LongDescription)
				fmt.Fprintln(wr, "")
			}
		}

		for _, opt := range group.options {
			if !opt.showInHelp() {
				continue
			}

			fmt.Fprintln(wr, ".TP")
			fmt.Fprintf(wr, "\\fB")

			if opt.ShortName != 0 {
				fmt.Fprintf(wr, "\\fB\\-%c\\fR", opt.ShortName)
			}

			if len(opt.LongName) != 0 {
				if opt.ShortName != 0 {
					fmt.Fprintf(wr, ", ")
				}

				fmt.Fprintf(wr, "\\fB\\-\\-%s\\fR", manQuote(opt.LongNameWithNamespace()))
			}

			if len(opt.ValueName) != 0 || opt.OptionalArgument {
				if opt.OptionalArgument {
					fmt.Fprintf(wr, " [\\fI%s=%s\\fR]", manQuote(opt.ValueName), manQuote(strings.Join(quoteV(opt.OptionalValue), ", ")))
				} else {
					fmt.Fprintf(wr, " \\fI%s\\fR", manQuote(opt.ValueName))
				}
			}

			if len(opt.Default) != 0 {
				fmt.Fprintf(wr, " <default: \\fI%s\\fR>", manQuote(strings.Join(quoteV(opt.Default), ", ")))
			} else if len(opt.EnvKeyWithNamespace()) != 0 {
				if runtime.GOOS == "windows" {
					fmt.Fprintf(wr, " <default: \\fI%%%s%%\\fR>", manQuote(opt.EnvKeyWithNamespace()))
				} else {
					fmt.Fprintf(wr, " <default: \\fI$%s\\fR>", manQuote(opt.EnvKeyWithNamespace()))
				}
			}

			if opt.Required {
				fmt.Fprintf(wr, " (\\fIrequired\\fR)")
			}

			fmt.Fprintln(wr, "\\fP")

			if len(opt.Description) != 0 {
				formatForMan(wr, opt.Description)
				fmt.Fprintln(wr, "")
			}
		}
	})
}

func writeManPageSubcommands(wr io.Writer, name string, root *Command) {
	subs := root.sortedVisibleCommands()

	for _, c := range subs {
		var nn string

		if c.Hidden {
			continue
		}

		if len(name) != 0 {
			nn = name + " " + c.Name
		} else {
			nn = c.Name
		}

		writeManPageCommand(wr, nn, root, c)
	}
}

func writeManPageCommand(wr io.Writer, name string, root *Command, command *Command) {
	fmt.Fprintf(wr, ".SS %s\n", name)
	fmt.Fprintln(wr, command.ShortDescription)

	if len(command.LongDescription) > 0 {
		fmt.Fprintln(wr, "")

		cmdstart := fmt.Sprintf("The %s command", manQuote(command.Name))

		if strings.HasPrefix(command.LongDescription, cmdstart) {
			fmt.Fprintf(wr, "The \\fI%s\\fP command", manQuote(command.Name))

			formatForMan(wr, command.LongDescription[len(cmdstart):])
			fmt.Fprintln(wr, "")
		} else {
			formatForMan(wr, command.LongDescription)
			fmt.Fprintln(wr, "")
		}
	}

	var usage string
	if us, ok := command.data.(Usage); ok {
		usage = us.Usage()
	} else if command.hasHelpOptions() {
		usage = fmt.Sprintf("[%s-OPTIONS]", command.Name)
	}

	// TODO: Start by doing this backwards, just to show we can traverse what we need to
	var pre2 strings.Builder
	var r1 = command
	for {
		if r2, ok := r1.parent.(*Command); !ok {
			break
		} else {
			r1 = r2
		}

		pre2.WriteString(r1.Name)
		pre2.WriteString(" ")
	}
	//if r , ok := command.parent.(*Command); ok {
	//	for ok {
	//		pre2.WriteString(r.Name)
	//		pre2.WriteString(" ")
	//		r, ok = r.parent.(*Command)
	//	}
	//}
	pre2.WriteString(command.Name)


	//var pre strings.Builder
	//for i := 0; i < len(root); i++ {
	//	var p = root[i]
	//	pre.WriteString(p.Name)
	//	pre.WriteString(" ")
	//	if p.hasHelpOptions() {
	//		if i == 0 {
	//			pre.WriteString("[OPTIONS] ")
	//		} else {
	//			pre.WriteString("[")
	//			pre.WriteString(p.Name)
	//			pre.WriteString("-OPTIONS] ")
	//		}
	//	}
	//}
	//pre.WriteString(command.Name)

	if len(usage) > 0 {
		fmt.Fprintf(wr, "\n\\fBUsage\\fP: %s %s\n.TP\n", manQuote(pre2.String()), manQuote(usage))
	}

	if len(command.Aliases) > 0 {
		fmt.Fprintf(wr, "\n\\fBAliases\\fP: %s\n\n", manQuote(strings.Join(command.Aliases, ", ")))
	}

	writeManPageOptions(wr, command.Group)
	writeManPageSubcommands(wr, name, command)
}

// WriteManPage writes a basic man page in groff format to the specified
// writer.
//
// TODO: Bug with writing usage for subcommands
func (p *Parser) WriteManPage(wr io.Writer) {
	t := time.Now()
	source_date_epoch := os.Getenv("SOURCE_DATE_EPOCH")
	if source_date_epoch != "" {
		sde, err := strconv.ParseInt(source_date_epoch, 10, 64)
		if err != nil {
			panic(fmt.Sprintf("Invalid SOURCE_DATE_EPOCH: %s", err))
		}
		t = time.Unix(sde, 0)
	}

	fmt.Fprintf(wr, ".TH %s 1 \"%s\"\n", manQuote(p.Name), t.Format("2 January 2006"))
	fmt.Fprintln(wr, ".SH NAME")
	fmt.Fprintf(wr, "%s \\- %s\n", manQuote(p.Name), manQuote(p.ShortDescription))
	fmt.Fprintln(wr, ".SH SYNOPSIS")

	usage := p.Usage

	if len(usage) == 0 {
		usage = "[OPTIONS]"
	}

	fmt.Fprintf(wr, "\\fB%s\\fP %s\n", manQuote(p.Name), manQuote(usage))
	fmt.Fprintln(wr, ".SH DESCRIPTION")

	formatForMan(wr, p.LongDescription)
	fmt.Fprintln(wr, "")

	fmt.Fprintln(wr, ".SH OPTIONS")

	writeManPageOptions(wr, p.Command.Group)

	if len(p.visibleCommands()) > 0 {
		fmt.Fprintln(wr, ".SH COMMANDS")

		writeManPageSubcommands(wr, "", p.Command)
	}
}
