//go:build linux

package mtavalidation

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func Supported() bool { return runtime.GOARCH == "amd64" }

// HelperMain is an early, argument-free CLI mode. No GUI is constructed and no
// game path, shell command, output destination or password is accepted.
func HelperMain(args []string) (bool, error) {
	if len(args) == 0 || args[0] != helperFlag {
		return false, nil
	}
	if len(args) != 1 {
		return true, fmt.Errorf("the MTA validation helper takes no arguments")
	}
	if os.Geteuid() != 0 {
		return true, fmt.Errorf("MTA validation requires explicit local pkexec authorization")
	}
	if !Supported() {
		return true, fmt.Errorf("the pinned official helper requires Linux x86-64")
	}
	return true, privileged(os.Stdin, os.Stdout)
}

// Run belongs in the downgrader executable, whose main must dispatch HelperMain
// before constructing the GUI. Only that restricted CLI mode is authorized.
func Run(ctx context.Context, p *Packages, progress Progress) (Result, error) {
	return run(ctx, p, progress, "/usr/bin/pkexec")
}

// The authorizer is fixed by Run; the private seam permits disposable IPC checks
// without prompting for privileges or replacing any system executable.
func run(ctx context.Context, p *Packages, progress Progress, authorizer string) (Result, error) {
	var result Result
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if !Supported() {
		return result, fmt.Errorf("MTA serial validation helper requires Linux x86-64")
	}
	if os.Geteuid() == 0 {
		return result, fmt.Errorf("run the application as your normal desktop user; only the restricted helper mode is elevated")
	}
	if p == nil {
		return result, fmt.Errorf("verified helper packages are missing")
	}
	// Verify again immediately before crossing the privilege boundary.
	if _, err := unpack(p.server, serverPackage); err != nil {
		return result, err
	}
	if _, err := unpack(p.config, configPackage); err != nil {
		return result, err
	}
	// Snapshot the running image, not a possibly replaced executable path. The
	// private regular copy also works when the GUI lives on a user-only AppImage
	// FUSE mount that root cannot traverse. It is never a general root GUI launch.
	executable, cleanup, err := invoker(ctx)
	if err != nil {
		return result, err
	}
	defer cleanup()
	if _, err := os.Stat(authorizer); err != nil {
		return result, fmt.Errorf("local authorization requires pkexec and a running desktop authentication agent")
	}
	input, writer, err := os.Pipe()
	if err != nil {
		return result, err
	}
	defer input.Close()
	defer writer.Close()
	callerContext := ctx
	ctx, cancel := context.WithTimeout(ctx, Lifetime+2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, authorizer, executable, helperFlag)
	command.Stdin = input
	// Closing the pipe is the authenticated helper's stop signal. Killing a root
	// process from this UI is neither assumed to work nor needed for cancellation.
	command.Cancel = func() error {
		_ = writer.Close()
		err := command.Process.Signal(syscall.SIGTERM)
		if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return err
	}
	command.WaitDelay = 10 * time.Second
	output, err := command.StdoutPipe()
	if err != nil {
		return result, err
	}
	// Do not expose the server's password, detected address or raw diagnostics.
	command.Stderr = io.Discard
	if progress != nil {
		progress(Event{Stage: "authorize", Message: "Approve the local authorization dialog within 90 seconds. The launcher itself stays unprivileged."})
	}
	if err := command.Start(); err != nil {
		return result, fmt.Errorf("start local authorization: %w", err)
	}
	_ = input.Close() // The child owns its duplicated read end; don't keep a failed authorization writer blocked.
	var authorizationExpired atomic.Bool
	authorizationTimer := time.AfterFunc(90*time.Second, func() { authorizationExpired.Store(true); cancel() })
	defer authorizationTimer.Stop()
	sent := make(chan error, 1)
	go func() {
		var header bytes.Buffer
		header.WriteString(protocol)
		_ = binary.Write(&header, binary.BigEndian, uint32(len(p.server)))
		_ = binary.Write(&header, binary.BigEndian, uint32(len(p.config)))
		_, err := writer.Write(header.Bytes())
		if err == nil {
			_, err = writer.Write(p.server)
		}
		if err == nil {
			_, err = writer.Write(p.config)
		}
		sent <- err
	}()
	scanner := bufio.NewScanner(io.LimitReader(output, 1<<20))
	scanner.Buffer(make([]byte, 4096), 16384)
	var helperError, cleanupError string
	workspaceCreated := false
	for scanner.Scan() {
		var event Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			helperError = "invalid response from the local validation helper"
			cancel()
			break
		}
		switch event.Stage {
		case "verified":
			authorizationTimer.Stop()
		case "ready":
			result.MasterReady = true
		case "workspace":
			workspaceCreated = true
		case "cleaned":
			result.Cleaned = true
		case "starting", "stopped":
		case "cleanup-error":
			cleanupError = event.Message
		case "error":
			helperError = event.Message
		default:
			helperError = "unexpected response from the local validation helper"
			cancel()
		}
		if progress != nil {
			progress(event)
		}
	}
	scanErr := scanner.Err()
	if scanErr != nil {
		cancel()
	}
	waitErr := command.Wait()
	_ = writer.Close()
	<-sent
	if cleanupError != "" {
		return result, fmt.Errorf("%s", cleanupError)
	}
	if workspaceCreated && !result.Cleaned {
		return result, fmt.Errorf("helper cleanup was not confirmed; root-private temporary files may remain. No client success is recorded")
	}
	if callerContext.Err() != nil {
		return result, callerContext.Err()
	}
	if helperError != "" {
		return result, fmt.Errorf("%s", helperError)
	}
	if scanErr != nil {
		return result, scanErr
	}
	if authorizationExpired.Load() {
		return result, fmt.Errorf("local authorization timed out after 90 seconds; no client validation was confirmed")
	}
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	if waitErr != nil {
		var exit *exec.ExitError
		if errors.As(waitErr, &exit) && (exit.ExitCode() == 126 || exit.ExitCode() == 127) {
			return result, fmt.Errorf("local authorization was cancelled, denied or unavailable; no validation was confirmed")
		}
		return result, fmt.Errorf("local validation helper exited unsuccessfully; no client success was confirmed")
	}
	if !result.MasterReady {
		return result, fmt.Errorf("helper stopped without confirmed MTA master-server contact")
	}
	return result, nil
}
func invoker(ctx context.Context) (string, func(), error) {
	source, err := os.Open("/proc/self/exe")
	if err != nil {
		return "", nil, err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return "", nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > 2<<30 {
		return "", nil, fmt.Errorf("running executable cannot be staged for the restricted validation helper")
	}
	work, err := os.MkdirTemp("/tmp", "gtasa-mta-validation-invoker-")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(work) }
	destination := filepath.Join(work, "gtasa-mta-validation-helper")
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		cleanup()
		return "", nil, err
	}
	var copied int64
	buffer := make([]byte, 128<<10)
	for err == nil {
		if err = ctx.Err(); err != nil {
			break
		}
		var n int
		n, err = source.Read(buffer)
		if n > 0 {
			var written int
			var writeErr error
			written, writeErr = file.Write(buffer[:n])
			copied += int64(written)
			if writeErr != nil {
				err = writeErr
				break
			}
		}
		if err == io.EOF {
			err = nil
			break
		}
	}
	if err == nil && copied != info.Size() {
		err = fmt.Errorf("running executable changed while staging the helper")
	}
	if err == nil {
		err = file.Sync()
	}
	if err == nil {
		err = file.Chmod(0500)
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return destination, cleanup, nil
}
func privileged(input *os.File, output io.Writer) (err error) {
	emit := func(stage, message string) { _ = json.NewEncoder(output).Encode(Event{Stage: stage, Message: message}) }
	defer func() {
		if err != nil {
			emit("error", err.Error())
		}
	}()
	// Apply before any payload or random password can enter a core dump, and
	// make files created by the native server private by default as well.
	if err = syscall.Setrlimit(syscall.RLIMIT_CORE, &syscall.Rlimit{}); err != nil {
		return err
	}
	syscall.Umask(0077)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, Lifetime)
	defer cancel()
	closeInput := context.AfterFunc(ctx, func() { _ = input.Close() })
	defer closeInput()
	// Never wait indefinitely for an unprivileged producer to supply the payload.
	readTimer := time.AfterFunc(15*time.Second, func() { _ = input.Close() })
	defer readTimer.Stop()
	magic := make([]byte, len(protocol))
	if _, err = io.ReadFull(input, magic); err != nil {
		return err
	}
	if string(magic) != protocol {
		return fmt.Errorf("invalid validation helper protocol")
	}
	var serverLen, configLen uint32
	if err = binary.Read(input, binary.BigEndian, &serverLen); err != nil {
		return err
	}
	if err = binary.Read(input, binary.BigEndian, &configLen); err != nil {
		return err
	}
	if serverLen == 0 || int64(serverLen) > serverPackage.Limit || configLen == 0 || int64(configLen) > configPackage.Limit {
		return fmt.Errorf("validation payload exceeds its limits")
	}
	server := make([]byte, serverLen)
	config := make([]byte, configLen)
	if _, err = io.ReadFull(input, server); err != nil {
		return err
	}
	if _, err = io.ReadFull(input, config); err != nil {
		return err
	}
	readTimer.Stop()
	serverFiles, err := unpack(server, serverPackage)
	if err != nil {
		return err
	}
	configFiles, err := unpack(config, configPackage)
	if err != nil {
		return err
	}
	// An EOF, malformed control byte, parent crash or cancellation all stop work.
	go func() { var b [1]byte; _, _ = input.Read(b[:]); cancel() }()
	if err = ctx.Err(); err != nil {
		return err
	}
	emit("verified", "Official helper archives reverified before privileged extraction.")
	if err = portsFree(); err != nil {
		return err
	}
	work, err := os.MkdirTemp("/tmp", "gtasa-mta-validation-root-")
	if err != nil {
		return err
	}
	defer func() {
		if cleanupErr := os.RemoveAll(work); cleanupErr != nil {
			emit("cleanup-error", "The helper stopped, but its root-private temporary files could not be removed. No game or prefix files were changed.")
			err = errors.Join(err, cleanupErr)
		} else {
			emit("cleaned", "Temporary helper files removed.")
		}
	}()
	emit("workspace", "Preparing verified files in a root-private temporary workspace.")
	if err = extract(work, serverFiles); err != nil {
		return err
	}
	deathmatch := filepath.Join(work, "mods", "deathmatch")
	if err = extract(deathmatch, configFiles); err != nil {
		return err
	}
	random := make([]byte, 8)
	if _, err = rand.Read(random); err != nil {
		return err
	}
	// A fresh valid 16-character password; no resources, HTTP, LAN broadcast,
	// voice, account changes or crash uploads. ASE must remain enabled: its
	// outgoing master query is necessary for the proven recovery workflow.
	configuration := fmt.Sprintf(`<config>
<servername>Temporary MTA serial validation</servername>
<serverip>auto</serverip><serverport>%d</serverport>
<maxplayers>1</maxplayers><password>%s</password>
<httpserver>0</httpserver><httpport>22005</httpport>
<ase>1</ase><donotbroadcastlan>1</donotbroadcastlan>
<voice>0</voice><crash_dump_upload>0</crash_dump_upload>
<minclientversion_auto_update>0</minclientversion_auto_update>
<logfile>logs/server.log</logfile><authfile>logs/server_auth.log</authfile>
<dbfile>logs/db.log</dbfile><scriptdebuglogfile>logs/scripts.log</scriptdebuglogfile>
<acl>acl.xml</acl>
</config>
`, ServerPort, hex.EncodeToString(random))
	if err = os.WriteFile(filepath.Join(deathmatch, "mtaserver.conf"), []byte(configuration), 0600); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(deathmatch, "logs"), 0700); err != nil {
		return err
	}
	if err = os.Mkdir(filepath.Join(work, "tmp"), 0700); err != nil {
		return err
	}
	if err = os.Chmod(filepath.Join(work, "mta-server64"), 0700); err != nil {
		return err
	}
	// Do not inherit LD_*, Proton, credentials, desktop variables or a
	// user-controlled working directory.
	if err = ctx.Err(); err != nil {
		return err
	}
	command := exec.Command(filepath.Join(work, "mta-server64"), "-s", "-D", work, "--port", fmt.Sprint(ServerPort), "--maxplayers", "1", "--novoice")
	command.Dir = work
	command.Env = []string{"PATH=/usr/bin:/bin", "HOME=" + work, "TMPDIR=" + filepath.Join(work, "tmp"), "LANG=C", "LC_ALL=C"}
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	if err = command.Start(); err != nil {
		return fmt.Errorf("start verified official MTA server: %w", err)
	}
	defer func() {
		// WNOWAIT keeps the leader unreaped until every group signal is sent. Its
		// numeric PID/PGID cannot be recycled into an unrelated process meanwhile.
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGTERM)
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if stopped, _ := childExited(command.Process.Pid); stopped {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Wait()
		emit("stopped", "Temporary helper stopped and its process group was reaped.")
	}()
	emit("starting", "Helper running; waiting for successful MTA master-server contact.")
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	queryDeadline := time.NewTimer(90 * time.Second)
	defer queryDeadline.Stop()
	ready := false
	logPath := filepath.Join(deathmatch, "logs", "server.log")
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-queryDeadline.C:
			if !ready {
				return fmt.Errorf("MTA master server did not report success within 90 seconds; check networking, runtime libraries and UDP port availability")
			}
		case <-ticker.C:
			stopped, waitErr := childExited(command.Process.Pid)
			if waitErr != nil {
				return waitErr
			}
			if stopped {
				return fmt.Errorf("official MTA server stopped unexpectedly; check runtime libraries and the official guide")
			}
			f, e := os.Open(logPath)
			if errors.Is(e, os.ErrNotExist) {
				continue
			}
			if e != nil {
				return e
			}
			info, e := f.Stat()
			if e != nil {
				f.Close()
				return e
			}
			if !info.Mode().IsRegular() || info.Size() > 4<<20 {
				f.Close()
				return fmt.Errorf("unexpected helper log file")
			}
			data, e := io.ReadAll(io.LimitReader(f, 4<<20+1))
			f.Close()
			if e != nil {
				return e
			}
			text := string(data)
			if strings.Contains(text, "Querying MTA master server... failed!") {
				return fmt.Errorf("official MTA helper could not contact its master server; startup alone does not confirm serial validation")
			}
			if !ready && strings.Contains(text, "Querying MTA master server... success!") {
				ready = true
				queryDeadline.Stop()
				emit("ready", "MTA master-server contact succeeded. Retry the client now; client success is still unconfirmed.")
			}
		}
	}
}
func childExited(pid int) (bool, error) {
	var info unix.Siginfo
	err := unix.Waitid(unix.P_PID, pid, &info, unix.WEXITED|unix.WNOHANG|unix.WNOWAIT, nil)
	return info.Signo != 0, err
}
func portsFree() error {
	var listeners []net.PacketConn
	defer func() {
		for _, l := range listeners {
			_ = l.Close()
		}
	}()
	for _, port := range []int{ServerPort, ASEPort} {
		l, err := net.ListenPacket("udp4", fmt.Sprintf("0.0.0.0:%d", port))
		if err != nil {
			return fmt.Errorf("UDP port %d is unavailable; another server may already be running. It will not be stopped", port)
		}
		listeners = append(listeners, l)
	}
	return nil
}
func extract(root string, files []archiveFile) error {
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	for _, item := range files {
		destination := filepath.Join(root, filepath.FromSlash(item.Name))
		if item.Directory {
			if err := os.MkdirAll(destination, 0700); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return err
		}
		file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		_, writeErr := file.Write(item.Data)
		if err := errors.Join(writeErr, file.Close()); err != nil {
			return err
		}
	}
	return nil
}
