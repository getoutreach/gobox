// Package sshhelper is a toolkit for common ssh-related operations.
package sshhelper

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/AlecAivazis/survey/v2"
	gogitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/kevinburke/ssh_config"
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
	return agent.NewKeyring()
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
	sshFile := ssh_config.Get(host, "IdentityFile")
	if sshFile == "" {
		return "", fmt.Errorf("failed to find an IdentityFile for host '%s' in ssh config", host)
	}

	sshFile, err := homedir.Expand(sshFile)
	if err != nil {
		return "", errors.Wrapf(err, "failed to expand ~/ in ssh IdentityFile path for host '%s'", host)
	}

	if !filepath.IsAbs(sshFile) {
		return "", fmt.Errorf("returned IdentityFile is not an absolute path for host '%s'", host)
	}

	return sshFile, AddKeyToAgent(sshFile, a, log)
}

// AddKeyToAgent adds a key to the internal ssh-key agent.
func AddKeyToAgent(keyPath string, a agent.Agent, log logrus.FieldLogger) error { // nolint:funlen
	b, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return errors.Wrap(err, "failed to read private key")
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
