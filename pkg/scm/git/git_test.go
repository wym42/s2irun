package git

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	testcmd "github.com/kubesphere/s2irun/pkg/test/cmd"
	testfs "github.com/kubesphere/s2irun/pkg/test/fs"
	"github.com/kubesphere/s2irun/pkg/utils/fs"
)

func TestIsValidGitRepository(t *testing.T) {
	fileSystem := fs.NewFileSystem()

	// a local git repo with a commit
	d, err := CreateLocalGitDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	ok, err := IsLocalNonBareGitRepository(fileSystem, d)
	if !ok || err != nil {
		t.Errorf("IsLocalNonBareGitRepository returned %v, %v", ok, err)
	}
	empty, err := LocalNonBareGitRepositoryIsEmpty(fileSystem, d)
	if empty || err != nil {
		t.Errorf("LocalNonBareGitRepositoryIsEmpty returned %v, %v", ok, err)
	}

	// a local git repo with no commit
	d, err = CreateEmptyLocalGitDirectory()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	ok, err = IsLocalNonBareGitRepository(fileSystem, d)
	if !ok || err != nil {
		t.Errorf("IsLocalNonBareGitRepository returned %v, %v", ok, err)
	}
	empty, err = LocalNonBareGitRepositoryIsEmpty(fileSystem, d)
	if !empty || err != nil {
		t.Errorf("LocalNonBareGitRepositoryIsEmpty returned %v, %v", ok, err)
	}

	// a directory which is not a git repo
	d = filepath.Join(d, ".git")

	ok, err = IsLocalNonBareGitRepository(fileSystem, d)
	if ok || err != nil {
		t.Errorf("IsLocalNonBareGitRepository returned %v, %v", ok, err)
	}

	// a submodule git repo with a commit
	d, err = CreateLocalGitDirectoryWithSubmodule()
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	ok, err = IsLocalNonBareGitRepository(fileSystem, filepath.Join(d, "submodule"))
	if !ok || err != nil {
		t.Errorf("IsLocalNonBareGitRepository returned %v, %v", ok, err)
	}
	empty, err = LocalNonBareGitRepositoryIsEmpty(fileSystem, filepath.Join(d, "submodule"))
	if empty || err != nil {
		t.Errorf("LocalNonBareGitRepositoryIsEmpty returned %v, %v", ok, err)
	}
}

func getGit() (Git, *testcmd.FakeCmdRunner) {
	cr := &testcmd.FakeCmdRunner{}
	gh := New(&testfs.FakeFileSystem{}, cr)

	return gh, cr
}

func TestGitClone(t *testing.T) {
	gh, ch := getGit()
	err := gh.Clone(MustParse("source1"), "target1", CloneConfig{Quiet: true, Recursive: true})
	if err != nil {
		t.Errorf("Unexpected error returned from clone: %v", err)
	}
	if ch.Name != "git" {
		t.Errorf("Unexpected command name: %q", ch.Name)
	}
	if !reflect.DeepEqual(ch.Args, []string{"clone", "--quiet", "--recursive", "source1", "target1"}) {
		t.Errorf("Unexpected command arguments: %#v", ch.Args)
	}
}

func TestGitCloneError(t *testing.T) {
	gh, ch := getGit()
	runErr := fmt.Errorf("Run Error")
	ch.Err = runErr
	err := gh.Clone(MustParse("source1"), "target1", CloneConfig{})
	if err != runErr {
		t.Errorf("Unexpected error returned from clone: %v", err)
	}
}

func TestGitCheckout(t *testing.T) {
	gh, ch := getGit()
	err := gh.Checkout("repo1", "ref1")
	if err != nil {
		t.Errorf("Unexpected error returned from checkout: %v", err)
	}
	if ch.Name != "git" {
		t.Errorf("Unexpected command name: %q", ch.Name)
	}
	if !reflect.DeepEqual(ch.Args, []string{"checkout", "ref1"}) {
		t.Errorf("Unexpected command arguments: %#v", ch.Args)
	}
	if ch.Opts.Dir != "repo1" {
		t.Errorf("Unexpected value in exec directory: %q", ch.Opts.Dir)
	}
}

func TestGitCheckoutError(t *testing.T) {
	gh, ch := getGit()
	runErr := fmt.Errorf("Run Error")
	ch.Err = runErr
	err := gh.Checkout("repo1", "ref1")
	if err != runErr {
		t.Errorf("Unexpected error returned from checkout: %v", err)
	}
}
