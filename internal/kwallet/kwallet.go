// Package kwallet stores the Gemini API key in KDE Wallet.
//
// The key is never read from environment variables and never written to the
// config file: kwallet-query reads it, and a small D-Bus client writes it.
package kwallet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	folder = "ask"
	entry  = "gemini-api-key"
	appID  = "ask"
)

// ErrNotFound means the wallet is reachable but has no key for ask.
var ErrNotFound = errors.New("Gemini API key is not stored in KWallet")

type runner func(ctx context.Context, stdin, name string, args ...string) (string, error)

// Store reads and writes the key in the default KDE wallet.
type Store struct {
	run  runner
	look func(string) (string, error)
}

// New returns a store backed by the system tools.
func New() *Store {
	return &Store{run: execRunner, look: exec.LookPath}
}

func newStore(run runner, look func(string) (string, error)) *Store {
	return &Store{run: run, look: look}
}

// Get returns the Gemini API key from the wallet.
func (s *Store) Get(ctx context.Context) (string, error) {
	query, err := s.look("kwallet-query")
	if err != nil {
		return "", errors.New("kwallet-query not found: install the kwallet6 package")
	}

	wallet := s.walletName(ctx)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	out, err := s.run(ctx, "", query, "-f", folder, "-r", entry, wallet)
	if err != nil {
		if ctx.Err() != nil {
			return "", errors.New("timed out waiting for KWallet to unlock")
		}
		return "", fmt.Errorf("%w: %v", ErrNotFound, err)
	}

	key := strings.TrimSpace(out)
	if key == "" {
		return "", ErrNotFound
	}
	return key, nil
}

// Set writes the key to the wallet, creating the ask folder if needed.
func (s *Store) Set(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("API key is empty")
	}
	if len(key) >= 512 || strings.ContainsAny(key, "\r\n") {
		return errors.New("API key must be a single line shorter than 512 characters")
	}

	wallet := s.walletName(ctx)

	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	// python3-dbus keeps the secret on stdin instead of the process argument
	// list. Exit code 3 means dbus-python is unavailable; fall back to qdbus6.
	if python, err := s.look("python3"); err == nil {
		_, err = s.run(ctx, key+"\n", python, "-c", pythonScript, wallet, folder, entry)
		if err == nil {
			return nil
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 3 {
			return fmt.Errorf("could not save key to KWallet (is the wallet unlocked?): %w", err)
		}
	}

	if err := s.setQDBus(ctx, wallet, key); err != nil {
		return fmt.Errorf("could not save key to KWallet: %w", err)
	}
	return nil
}

// Delete removes the key from the wallet. A missing entry is not an error.
func (s *Store) Delete(ctx context.Context) error {
	qdbus, err := s.look("qdbus6")
	if err != nil {
		return errors.New("qdbus6 not found: install the qdbus-qt6 package")
	}

	wallet := s.walletName(ctx)

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	call := func(method string, args ...string) (string, error) {
		out, err := s.run(ctx, "", qdbus,
			append([]string{"org.kde.kwalletd6", "/modules/kwalletd6", method}, args...)...)
		return strings.TrimSpace(out), err
	}

	handle, err := call("org.kde.KWallet.open", wallet, "0", appID)
	if err != nil || handle == "" || handle == "-1" {
		return errors.New("could not open the wallet")
	}

	result, err := call("org.kde.KWallet.removeEntry", handle, folder, entry, appID)
	if err != nil {
		return fmt.Errorf("could not remove key from KWallet: %w", err)
	}
	if result != "0" {
		return errors.New("could not remove key from KWallet")
	}
	return nil
}

func (s *Store) walletName(ctx context.Context) string {
	qdbus, err := s.look("qdbus6")
	if err != nil {
		return "kdewallet"
	}

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	out, err := s.run(ctx, "", qdbus,
		"org.kde.kwalletd6", "/modules/kwalletd6",
		"org.kde.KWallet.networkWallet",
	)
	if name := strings.TrimSpace(out); err == nil && name != "" {
		return name
	}
	return "kdewallet"
}

func (s *Store) setQDBus(ctx context.Context, wallet, key string) error {
	tool, err := s.look("qdbus6")
	service, path := "org.kde.kwalletd6", "/modules/kwalletd6"
	if err != nil {
		tool, err = s.look("qdbus")
		service, path = "org.kde.kwalletd5", "/modules/kwalletd5"
		if err != nil {
			return errors.New("neither python3-dbus nor qdbus6 is available")
		}
	}

	call := func(method string, args ...string) (string, error) {
		out, err := s.run(ctx, "", tool, append([]string{service, path, method}, args...)...)
		return strings.TrimSpace(out), err
	}

	handle, err := call("org.kde.KWallet.open", wallet, "0", appID)
	if err != nil || handle == "" || handle == "-1" {
		return errors.New("could not open the wallet")
	}

	// The folder may already exist; KWallet reports that as an error.
	_, _ = call("org.kde.KWallet.createFolder", handle, folder, appID)

	// This fallback necessarily passes the secret as an argument. It is used
	// only when dbus-python is missing.
	result, err := call("org.kde.KWallet.writePassword", handle, folder, entry, key, appID)
	if err != nil || result != "0" {
		return errors.New("could not write the key")
	}
	return nil
}

func execRunner(ctx context.Context, stdin, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("%s: %w: %s", name, err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

const pythonScript = `import sys
try:
    import dbus
except Exception:
    sys.exit(3)
wallet, folder, entry = sys.argv[1], sys.argv[2], sys.argv[3]
key = sys.stdin.readline().rstrip("\n")
if not key:
    sys.exit(1)
try:
    bus = dbus.SessionBus()
    obj = bus.get_object("org.kde.kwalletd6", "/modules/kwalletd6")
    iface = dbus.Interface(obj, "org.kde.KWallet")
    handle = iface.open(wallet, dbus.UInt32(0), "ask")
    if int(handle) < 0:
        sys.exit(1)
    try:
        iface.createFolder(handle, folder, "ask")
    except Exception:
        pass
    if int(iface.writePassword(handle, folder, entry, key, "ask")) != 0:
        sys.exit(1)
except Exception:
    sys.exit(1)
`
