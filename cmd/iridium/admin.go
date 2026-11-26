package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kumoSleeping/iridium/internal/api"
	"github.com/kumoSleeping/iridium/internal/config"
	"github.com/kumoSleeping/iridium/internal/hub"
	"github.com/kumoSleeping/iridium/internal/install"
	"github.com/kumoSleeping/iridium/internal/launchd"
	"github.com/kumoSleeping/iridium/internal/node"
	"github.com/kumoSleeping/iridium/internal/update"
	"github.com/kumoSleeping/iridium/internal/version"
)

// installCmd is `iridium install [options] [-public <host>]`: the user unit iridium.service (on
// macOS the LaunchAgent ~/Library/LaunchAgents/iridium.plist, on Windows a logon task) running
// `iridium serve` with the options given, the config and the state directory; started at once
// (-now=false: at the next login). -public then puts this computer's Iridium on the internet
// (ir public --host): Cloudflare asks the person to log in once, the rest is done, and the first
// invite is printed.
func installCmd(args []string) int {
	fs := flag.NewFlagSet("install", flag.ExitOnError)
	var rf runFlags
	rf.register(fs)
	pub := fs.String("public", "", "this computer's Iridium on the internet at this address, through Cloudflare")
	now := fs.Bool("now", true, "start the service now too")
	listen := fs.String("listen", "", "the address a config made now listens on (default 127.0.0.1:7810)")
	reload := fs.Bool("reload", true, "run systemctl --user daemon-reload")
	unit := fs.String("unit", "iridium", "the unit's name (tests: iridium-test-*)")
	prefix, bin := placeFlags(fs)
	var env envFlag
	fs.Var(&env, "env", "NAME=VALUE the service runs with too (repeatable)")
	fs.Parse(args)
	if fs.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "iridium install: %q is not an option\n", fs.Arg(0))
		return 2
	}
	if _, err := rf.terms(); err != nil {
		fmt.Fprintf(os.Stderr, "iridium install: %v\n", err)
		return 2
	}
	if rf.noServer && len(rf.joins) == 0 {
		fmt.Fprintln(os.Stderr, "iridium install: nothing to run: --no-server with no --join")
		return 2
	}
	if rf.noServer && *pub != "" {
		fmt.Fprintln(os.Stderr, "iridium install: -public needs this computer's Iridium (no --no-server)")
		return 2
	}
	s := service{config: rf.config, unit: *unit, prefix: *prefix, bin: *bin, env: env, now: *now, reload: *reload, listen: *listen, args: rf.serveArgs()}
	exe, err := s.register()
	if err != nil {
		log.Print(err)
		return 1
	}
	if *pub == "" {
		return 0
	}
	if !*now {
		fmt.Fprintln(os.Stderr, "iridium install: -public needs it started (no -now=false)")
		return 2
	}
	cfgPath, _ := filepath.Abs(rf.config)
	return goPublic(exe, cfgPath, *pub)
}

// placeFlags are where the binary is kept and where ir goes (install, ir start).
func placeFlags(fs *flag.FlagSet) (prefix, bin *string) {
	home, _ := os.UserHomeDir()
	defPrefix, defBin := filepath.Join(home, ".local", "opt", "iridium"), filepath.Join(home, ".local", "bin")
	if runtime.GOOS == "windows" {
		// %LOCALAPPDATA%\Iridium: the binary and ir.cmd, that folder put on the user's Path.
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			defPrefix = filepath.Join(d, "Iridium")
		}
		defBin = defPrefix
	}
	prefix = fs.String("prefix", defPrefix, "where the binary is kept (copied there when it runs from elsewhere)")
	bin = fs.String("bin", defBin, `where the ir link goes; "" none`)
	return prefix, bin
}

// service is the user's service of iridium's: the unit (systemd), LaunchAgent (macOS) or logon
// task (Windows) that runs command (serve, or ir start's run) with args.
type service struct {
	config, unit, prefix, bin string
	env                       []string
	now, reload, restart      bool
	listen, command           string
	args                      []string
}

// register places the binary (internal/install.Place) and writes the service, started with now
// (again, with restart, if it runs). It gives where the binary is.
func (s service) register() (string, error) {
	home, _ := os.UserHomeDir()
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err == nil {
		exe, err = install.Place(exe, s.prefix, s.bin)
	}
	if err != nil {
		return "", err
	}
	cfgPath, _ := filepath.Abs(s.config)
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		unitDir = filepath.Join(d, "systemd", "user")
	}
	o := install.Options{Exe: exe, ConfigPath: cfgPath, Listen: s.listen, UnitDir: unitDir, Unit: s.unit, Path: os.Getenv("PATH"), Env: s.env,
		Now: s.now, Restart: s.restart, Out: os.Stdout, Command: s.command, Args: s.args}
	if runtime.GOOS == "windows" {
		// A logon task of Task Scheduler, run as this user.
		o.Task = true
		if u, err := user.Current(); err == nil {
			o.User = u.Username
		}
		o.Schtasks = func(args ...string) error {
			cmd := exec.Command("schtasks", args...)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			return cmd.Run()
		}
	} else if runtime.GOOS == "darwin" {
		// A LaunchAgent; launchd gives it a bare PATH, so the agents' is written into it.
		o.Launchd, o.UnitDir, o.Log = true, launchd.Dir(home), launchd.LogFile(home, s.unit)
		o.Launchctl, o.Domain = launchd.Run, launchd.Domain(context.Background())
	} else if s.reload || s.now {
		o.Systemctl = func(args ...string) error {
			cmd := exec.Command("systemctl", append([]string{"--user"}, args...)...)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			return cmd.Run()
		}
	}
	return exe, install.Install(o)
}

// goPublic waits for the service just started to answer, then runs ir public --host host at this
// terminal: Cloudflare's login is the person's to do; the rest is done and the first invite
// printed.
func goPublic(exe, cfgPath, host string) int {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Print(err)
		return 1
	}
	fmt.Println("等待服务启动……")
	end := time.Now().Add(30 * time.Second)
	for {
		if c, err := net.DialTimeout("unix", cfg.SocketPath(), time.Second); err == nil {
			c.Close()
			break
		}
		if time.Now().After(end) {
			fmt.Fprintln(os.Stderr, "iridium install: 服务 30 秒内没有启动；之后可再运行 ir public --host "+host)
			return 1
		}
		time.Sleep(500 * time.Millisecond)
	}
	cmd := exec.Command(exe, "ir", "public", "--host", host)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "IRIDIUM_CONFIG="+cfgPath)
	if err := cmd.Run(); err != nil {
		if x, ok := err.(*exec.ExitError); ok {
			return x.ExitCode()
		}
		log.Print(err)
		return 1
	}
	return 0
}

// pushCmd is `iridium push [-i key] [-listen addr] <user@host>`.
func pushCmd(args []string) int {
	fs := flag.NewFlagSet("push", flag.ExitOnError)
	key := fs.String("i", "", "the ssh identity")
	listen := fs.String("listen", "", "the address a first install there listens on")
	fs.Parse(args)
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: iridium push [-i <key>] [-listen <addr>] <user@host>")
		return 2
	}
	if err := node.Push(context.Background(), node.PushOptions{Target: fs.Arg(0), Key: *key, Listen: *listen, Out: os.Stdout}); err != nil {
		log.Print(err)
		return 1
	}
	return 0
}

// nodeCmd is `iridium node new <account> [-listen addr]`.
func nodeCmd(args []string) int {
	if len(args) < 2 || args[0] != "new" {
		fmt.Fprintln(os.Stderr, "usage: iridium node new <account> [-listen <addr>]")
		return 2
	}
	fs := flag.NewFlagSet("node new", flag.ExitOnError)
	listen := fs.String("listen", "", "its address (default the first free 127.0.0.1 port from 7811)")
	fs.Parse(args[2:])
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: iridium node new <account> [-listen <addr>]")
		return 2
	}
	if err := node.New(context.Background(), node.NewOptions{User: args[1], Listen: *listen, Out: os.Stdout}); err != nil {
		log.Print(err)
		return 1
	}
	return 0
}

// envFlag is -env NAME=VALUE, once for each.
type envFlag []string

func (e *envFlag) String() string     { return strings.Join(*e, " ") }
func (e *envFlag) Set(v string) error { *e = append(*e, v); return nil }

// newUpdater looks for new versions where the config says, carrying its state on h.
func newUpdater(cfg config.Config, h *hub.Hub) *update.Updater {
	return update.New(update.Options{
		Source:  cfg.Update.Source(update.DefaultSource),
		Current: version.Version,
		Publish: func(s api.UpdateState) { h.Set("update", s) },
		Restart: func() error {
			// After the reply that says so has gone out.
			time.AfterFunc(500*time.Millisecond, func() {
				if err := restart(); err != nil {
					log.Printf("update: restart: %v", err)
				}
			})
			return nil
		},
		Logf: log.Printf,
	})
}

func versionCmd() int {
	fmt.Println("iridium", version.Version)
	return 0
}
