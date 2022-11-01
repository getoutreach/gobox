// Copyright 2022 Outreach Corporation. All Rights Reserved.

// Description: Package sshhelper is a toolkit for common ssh-related operations.

package sshhelper

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	"github.com/getoutreach/gobox/pkg/sshconfig"
	gogitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/mitchellh/go-homedir"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// GetSSHAgent creates a new ssh-agent, originally it would return one only
// if it was needed, but it's probably better to always use our own.
func GetSSHAgent() agent.Agent {
	a := agent.NewKeyring()
	if addr, ok := os.LookupEnv("SSH_AUTH_SOCK"); ok {
		agentConn, err := (&net.Dialer{}).Dial("unix", addr)
		if err == nil {
			a = agent.NewClient(agentConn)
		}
	}

	return a
}

// GetPasswordInput retrieves input that doesn't echo back to the user as they type it
func GetPasswordInput() (string, error) {
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", err
	}

	return string(b), nil
}

// LoadDefaultKey loads the default for an alias/host and puts it into the keyring
// if a default key is not found, the user is prompted to provide the path
func LoadDefaultKey(host string, a agent.Agent, log logrus.FieldLogger) (string, error) {
	sshFile, err := sshconfig.Get(context.TODO(), host, "IdentityFile")
	if sshFile == "" || err != nil {
		return "", errors.Wrapf(err, "failed to find an IdentityFile for host %q in ssh config", host)
	}

	sshFile, err = homedir.Expand(sshFile)
	if err != nil {
		return "", errors.Wrapf(err, "failed to expand ~/ in ssh IdentityFile path for host %q", host)
	}

	if !filepath.IsAbs(sshFile) {
		return "", fmt.Errorf("returned IdentityFile is not an absolute path for host %q", host)
	}

	return sshFile, AddKeyToAgent(sshFile, a, log)
}

func pubKeyInAgent(a agent.Agent, pubByts []byte) (bool, error) {
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey(pubByts) //nolint:dogsled
	if err != nil {
		return false, errors.Wrap(err, "failed to parse public key")
	}

	keys, err := a.List()
	if err != nil {
		return false, errors.Wrap(err, "failed to list existing agent keys")
	}
	for _, k := range keys {
		if ssh.FingerprintSHA256(pubKey) == ssh.FingerprintSHA256(k) {
			// key already in agent
			return true, nil
		}
	}
	return false, nil
}

// AddKeyToAgent adds a key to the internal ssh-key agent. If a public key can
// be found at the same path as the private key, it will first check the agent
// and return nil if the key is already present
func AddKeyToAgent(keyPath string, a agent.Agent, log logrus.FieldLogger) error { // nolint:funlen
	b, err := os.ReadFile(keyPath)
	if err != nil {
		return errors.Wrap(err, "failed to read private key")
	}
	// If a public key exists use it to check if the key-to-add already
	// exists in the agent. If we can't find a public key we'll go ahead
	// and move along as if it's not in the agent.
	pubByts, err := os.ReadFile(keyPath + ".pub")
	if err == nil {
		exists, err2 := pubKeyInAgent(a, pubByts)
		if err2 != nil {
			return errors.Wrap(err2, "failed checking agent for existing key")
		}
		if exists {
			return nil
		}
	}

	pk, err := ssh.ParseRawPrivateKey(b)
	if err != nil {
		serviceName := "outreach-ssh"
		user := "default"

		pass := ""

		for {
			if pass, err = keyring.Get(serviceName, user); err != nil {
				err = survey.AskOne(&survey.Password{
					Message: "Please enter your SSH Key Password:",
					Help:    fmt.Sprintf("SSH Key: %s", keyPath),
				}, &pass, survey.WithValidator(survey.Required))
				if err != nil {
					return err
				}
				fmt.Println("")

				// nolint:govet
				if err := keyring.Set(serviceName, user, pass); err != nil {
					log.WithError(err).Warn("Failed to save key in keyring, will have to type this again.")
				}
			}

			pk, err = ssh.ParseRawPrivateKeyWithPassphrase(b, []byte(pass))
			if err != nil {
				// Delete the passphrase from the keyring.
				if _, err2 := keyring.Get(serviceName, user); err2 != nil {
					keyring.Delete(serviceName, user) //nolint:errcheck
				}

				log.WithError(err).Error("Failed to decrypt private key with provided passphrase")
				continue
			}

			break
		}
	}

	return a.Add(agent.AddedKey{
		PrivateKey:       pk,
		LifetimeSecs:     uint32(60 * 60), // 1 hour
		ConfirmBeforeUse: false,
	})
}

// ExistingSSHAgentCallback is based on gogit's transport ssh public key callback, but allows
// for using an existing ssh-agent
type ExistingSSHAgentCallback struct {
	User     string
	Callback func() (signers []ssh.Signer, err error)
	gogitssh.HostKeyCallbackHelper
}

func NewExistingSSHAgentCallback(a agent.Agent) *ExistingSSHAgentCallback {
	return &ExistingSSHAgentCallback{
		User:     "git",
		Callback: a.Signers,
	}
}

func (a *ExistingSSHAgentCallback) Name() string {
	return gogitssh.PublicKeysCallbackName
}

func (a *ExistingSSHAgentCallback) String() string {
	return fmt.Sprintf("user: %s, name: %s", a.User, a.Name())
}

func (a *ExistingSSHAgentCallback) ClientConfig() (*ssh.ClientConfig, error) {
	return a.SetHostKeyCallback(&ssh.ClientConfig{
		User: a.User,
		Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(a.Callback)},
	})
}
