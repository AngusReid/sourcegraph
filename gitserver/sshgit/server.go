// Package sshgit provides functionality for a git server over SSH.
package sshgit

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"

	"github.com/AaronO/go-git-http"
	"github.com/flynn/go-shlex"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
	"src.sourcegraph.com/sourcegraph/auth"
	"src.sourcegraph.com/sourcegraph/events"
	"src.sourcegraph.com/sourcegraph/go-sourcegraph/sourcegraph"
	"src.sourcegraph.com/sourcegraph/server/accesscontrol"
	"src.sourcegraph.com/sourcegraph/util/eventsutil"
)

const (
	// gitTransactionTimeout controls a hard deadline on the time to perform a single git transaction.
	gitTransactionTimeout = 30 * time.Minute
)

// Server is SSH git server.
type Server struct {
	listener net.Listener
	ctx      context.Context
	clientID string

	config    *ssh.ServerConfig
	reposRoot string // Path to repository storage directory.
}

// ListenAndStart listens on the TCP network address addr and starts the server.
func (s *Server) ListenAndStart(ctx context.Context, addr string, privateSigner ssh.Signer, clientID string) error {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = l
	s.ctx = ctx
	s.clientID = clientID

	s.config = &ssh.ServerConfig{
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if c.User() != "git" {
				return nil, fmt.Errorf(`unsupported SSH user %q; use "git" for SSH git access`, c.User())
			}

			cl, err := sourcegraph.NewClientFromContext(s.ctx)
			if err != nil {
				return nil, err
			}
			userSpec, err := cl.UserKeys.LookupUser(s.ctx, &sourcegraph.SSHPublicKey{Key: key.Marshal()})
			if err != nil {
				return nil, err
			}
			user, err := cl.Users.Get(s.ctx, userSpec)
			if err != nil {
				return nil, err
			}

			return &ssh.Permissions{
				CriticalOptions: map[string]string{
					uidKey:       fmt.Sprint(user.UID),
					userLoginKey: user.Login,
				},
			}, nil
		},
	}
	s.config.AddHostKey(privateSigner)

	s.reposRoot = filepath.Join(os.Getenv("SGPATH"), "repos")

	go s.run()
	return nil
}

func (s *Server) run() {
	for {
		tcpConn, err := s.listener.Accept()
		if err != nil {
			log.Printf("failed to accept incoming connection: %v\n", err)
			continue
		}
		tcpConn.SetDeadline(time.Now().Add(gitTransactionTimeout))
		go s.handleConn(tcpConn)
	}
}

func (s *Server) handleConn(tcpConn net.Conn) {
	sshConns.Inc()
	defer sshConns.Dec()

	defer tcpConn.Close()
	sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, s.config)
	if err != nil {
		log.Printf("failed to ssh handshake: %v\n", err)
		return
	}
	go ssh.DiscardRequests(reqs)
	for ch := range chans {
		if t := ch.ChannelType(); t != "session" {
			ch.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %v", t))
			continue
		}
		go s.handleChannel(sshConn, ch)
	}
}

func (s *Server) handleChannel(sshConn *ssh.ServerConn, newChan ssh.NewChannel) {
	ch, reqs, err := newChan.Accept()
	if err != nil {
		return
	}
	defer ch.Close()
	for req := range reqs {
		switch req.Type {
		case "shell":
			if req.WantReply {
				req.Reply(true, nil)
			}
			fmt.Fprintf(ch, "Hi %q, you've successfully authenticated, but we don't provide shell access.\n", sshConn.Permissions.CriticalOptions[userLoginKey])
			ch.SendRequest("exit-status", false, ssh.Marshal(exitStatusMsg{0}))
			return
		case "exec":
			if req.WantReply {
				req.Reply(true, nil)
			}
			err := s.execGitCommand(sshConn, ch, req)
			if err != nil {
				log.Println(err)
			}
			return
		case "env":
			if req.WantReply {
				req.Reply(true, nil)
			}
			// Do nothing.
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
}

func (s *Server) execGitCommand(sshConn *ssh.ServerConn, ch ssh.Channel, req *ssh.Request) error {
	if len(req.Payload) < 4 {
		return fmt.Errorf("invalid git transport protocol payload (less than 4 bytes): %q", req.Payload)
	}
	command := string(req.Payload[4:]) // E.g., "git-upload-pack '/user/repo'".
	args, err := shlex.Split(command)  // E.g., []string{"git-upload-pack", "/user/repo"}.
	if err != nil || len(args) != 2 {
		return fmt.Errorf("command %q is not a valid git command", command)
	}
	op := args[0]      // E.g., "git-upload-pack".
	repoURI := args[1] // E.g., "/user/repo".
	repoURI = path.Clean(repoURI)
	if path.IsAbs(repoURI) {
		repoURI = repoURI[1:] // Normalize "/user/repo" to "user/repo".
	}
	repoDir := filepath.Join(s.reposRoot, filepath.FromSlash(repoURI))
	if repoURI == "" || !strings.HasPrefix(repoDir, s.reposRoot) {
		fmt.Fprintf(ch.Stderr(), "Specified repo %q lies outside of root.\n\n", repoURI)
		return fmt.Errorf("specified repo %q lies outside of root", repoURI)
	}
	userLogin := sshConn.Permissions.CriticalOptions[userLoginKey]
	uid := uidFromSSHConn(sshConn)

	cl, err := sourcegraph.NewClientFromContext(s.ctx)
	if err != nil {
		return err
	}
	repo, err := cl.Repos.Get(s.ctx, &sourcegraph.RepoSpec{URI: repoURI})
	if err != nil {
		if grpc.Code(err) == codes.NotFound {
			fmt.Fprintf(ch.Stderr(), "Specified repo %q does not exist.\n\n", repoURI)
			return fmt.Errorf("specified repo %q does not exist: %v", repoURI, err)
		}
		fmt.Fprintf(ch.Stderr(), "Error accessing repo %q: %v\n\n", repoURI, err)
		return fmt.Errorf("error accessing repo %q: %v", repoURI, err)
	}

	user, err := cl.Users.Get(s.ctx, &sourcegraph.UserSpec{UID: uid})
	if err != nil {
		fmt.Fprintf(ch.Stderr(), "Could not find user with uid %v.\n\n", uid)
		return fmt.Errorf("could not find user with uid %v: %v", uid, err)
	}
	actor := auth.GetActorFromUser(*user)
	actor.ClientID = s.clientID

	// Check if user has sufficient permissions for git write/read access to this repo.
	switch op {
	case "git-upload-pack":
		// git-upload-pack uploads packs back to client. It happens when the client does
		// git fetch or similar. Check for read access.
		if err := accesscontrol.VerifyActorHasReadAccess(s.ctx, actor, "sshgit.git-upload-pack"); err != nil {
			fmt.Fprintf(ch.Stderr(), "User %q doesn't have read permissions.\n\n", userLogin)
			return fmt.Errorf("user %q doesn't have read permissions: %v", userLogin, err)
		}
	case "git-receive-pack":
		// git-receive-pack receives packs and applies them to the repository. It happens when the client does
		// git push or similar. Check for write access.
		if err := accesscontrol.VerifyActorHasWriteAccess(s.ctx, actor, "sshgit.git-receive-pack"); err != nil {
			fmt.Fprintf(ch.Stderr(), "User %q doesn't have write permissions.\n\n", userLogin)
			return fmt.Errorf("user %q doesn't have write permissions: %v", userLogin, err)
		}
		// Some repos should never be written to by users directly
		if !repo.IsSystemOfRecord() {
			fmt.Fprintf(ch.Stderr(), "Repo %q is not writeable.\n\n", repoURI)
			return fmt.Errorf("repo %q is not a system of record", repoURI)
		}
	default:
		return fmt.Errorf("%q is not a supported git operation", op)
	}
	shortOp := op[4:] // "upload-pack" or "receive-pack".

	// Execute the git operation.
	cmd := exec.Command("git", shortOp, ".")
	cmd.Dir = repoDir
	cmd.Stdout = ch
	cmd.Stderr = ch
	rpcReader := &githttp.RpcReader{
		Reader: ch,
		Rpc:    shortOp,
	}
	cmd.Stdin = rpcReader
	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("could not start command: %v", err)
	}
	err = waitTimeout(cmd, gitTransactionTimeout)
	cmdExitStatus := exitStatus(err)
	if err != nil {
		log.Printf("failed to exit cmd: %v\n", err)
	} else {
		payload := events.GitPayload{
			Actor: sourcegraph.UserSpec{UID: uid},
			Repo:  sourcegraph.RepoSpec{URI: repoURI},
		}
		for _, e := range collapseDuplicateEvents(rpcReader.Events) {
			payload.Event = e
			if e.Last == emptyCommitID {
				events.Publish(events.GitCreateBranchEvent, payload)
			} else if e.Commit == emptyCommitID {
				events.Publish(events.GitDeleteBranchEvent, payload)
			} else if e.Type == githttp.PUSH || e.Type == githttp.PUSH_FORCE || e.Type == githttp.TAG {
				events.Publish(events.GitPushEvent, payload)
			}
		}
	}
	if op == "git-receive-pack" {
		eventsutil.LogSSHGitPush(s.clientID, userLogin)
	}
	_, err = ch.SendRequest("exit-status", false, ssh.Marshal(cmdExitStatus))
	if err != nil {
		return fmt.Errorf("ch.SendRequest: %v", err)
	}
	return nil
}

// waitTimeout waits up to timeout for cmd to finish.
// If it doesn't finish in time, the process will be terminated.
func waitTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	finished := make(chan struct{})
	go func() {
		select {
		case <-time.After(timeout):
			cmd.Process.Kill()
		case <-finished:
			// All okay.
		}
	}()
	err := cmd.Wait()
	close(finished)
	return err
}

type exitStatusMsg struct {
	Status uint32
}

// exitStatus converts the error value from exec.Command.Wait() to an exitStatusMsg.
func exitStatus(err error) exitStatusMsg {
	switch err {
	case nil:
		return exitStatusMsg{0}
	default:
		if exitError, ok := err.(*exec.ExitError); ok {
			if status, ok := exitError.Sys().(syscall.WaitStatus); ok {
				return exitStatusMsg{uint32(status.ExitStatus())}
			}
		}
		return exitStatusMsg{1}
	}
}

const (
	uidKey       = "sourcegraph-uid"
	userLoginKey = "sourcegraph-user-login"
)

func uidFromSSHConn(sshConn *ssh.ServerConn) int32 {
	uid, err := strconv.ParseInt(sshConn.Permissions.CriticalOptions[uidKey], 10, 32)
	if err != nil {
		panic(fmt.Errorf("strconv.ParseInt error shouldn't happen since we encode it ourselves, but it happened: %v", err))
	}
	return int32(uid)
}
