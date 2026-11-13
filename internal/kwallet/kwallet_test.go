package kwallet

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func testStore(run runner, tools ...string) *Store {
	available := make(map[string]bool, len(tools))
	for _, name := range tools {
		available[name] = true
	}
	return newStore(run, func(name string) (string, error) {
		if available[name] {
			return "/usr/bin/" + name, nil
		}
		return "", errors.New("not found")
	})
}

func hasArg(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}

func TestGetReadsKeyFromWallet(t *testing.T) {
	store := testStore(func(_ context.Context, _ string, name string, args ...string) (string, error) {
		switch name {
		case "/usr/bin/qdbus6":
			return "kdewallet\n", nil
		case "/usr/bin/kwallet-query":
			want := []string{"-f", folder, "-r", entry, "kdewallet"}
			if len(args) != len(want) {
				t.Fatalf("kwallet-query args = %q, want %q", args, want)
			}
			for i := range want {
				if args[i] != want[i] {
					t.Fatalf("kwallet-query args = %q, want %q", args, want)
				}
			}
			return "secret-key\n", nil
		}
		return "", errors.New("unexpected command: " + name)
	}, "qdbus6", "kwallet-query")

	key, err := store.Get(context.Background())
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if key != "secret-key" {
		t.Fatalf("Get() = %q, want %q", key, "secret-key")
	}
}

func TestGetReturnsNotFound(t *testing.T) {
	store := testStore(func(_ context.Context, _ string, name string, _ ...string) (string, error) {
		if name == "/usr/bin/qdbus6" {
			return "kdewallet\n", nil
		}
		return "", errors.New("entry does not exist")
	}, "qdbus6", "kwallet-query")

	_, err := store.Get(context.Background())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
}

func TestSetKeepsKeyOutOfArguments(t *testing.T) {
	var stdin string
	var args []string

	store := testStore(func(_ context.Context, input, name string, cmdArgs ...string) (string, error) {
		switch name {
		case "/usr/bin/qdbus6":
			return "kdewallet\n", nil
		case "/usr/bin/python3":
			stdin, args = input, cmdArgs
			return "", nil
		}
		return "", errors.New("unexpected command: " + name)
	}, "qdbus6", "python3")

	if err := store.Set(context.Background(), "secret-key"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !strings.Contains(stdin, "secret-key") {
		t.Fatalf("key was not passed on stdin, stdin = %q", stdin)
	}
	for _, arg := range args {
		if strings.Contains(arg, "secret-key") {
			t.Fatalf("key leaked into process arguments: %q", args)
		}
	}
}

func TestSetRejectsMultilineKey(t *testing.T) {
	store := testStore(func(_ context.Context, _ string, _ string, _ ...string) (string, error) {
		return "", nil
	})

	if err := store.Set(context.Background(), "first\nsecond"); err == nil {
		t.Fatal("Set() accepted a multiline key")
	}
}

func TestDeleteUsesOpenHandle(t *testing.T) {
	var removeArgs []string

	store := testStore(func(_ context.Context, _ string, _ string, args ...string) (string, error) {
		switch {
		case hasArg(args, "org.kde.KWallet.networkWallet"):
			return "kdewallet\n", nil
		case hasArg(args, "org.kde.KWallet.open"):
			return "42\n", nil
		case hasArg(args, "org.kde.KWallet.removeEntry"):
			removeArgs = args
			return "0\n", nil
		}
		return "", errors.New("unexpected call")
	}, "qdbus6")

	if err := store.Delete(context.Background()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if !hasArg(removeArgs, "42") {
		t.Fatalf("removeEntry was not called with the open handle: %q", removeArgs)
	}
	if hasArg(removeArgs, "kdewallet") {
		t.Fatalf("removeEntry got a wallet name instead of a handle: %q", removeArgs)
	}
}
