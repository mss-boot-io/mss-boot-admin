package pkg

import (
	"testing"

	git "github.com/go-git/go-git/v5"
	gitHTTP "github.com/go-git/go-git/v5/plumbing/transport/http"
)

func TestGeneratorGitOptionsNeverRecurseIntoSubmodules(t *testing.T) {
	auth := &gitHTTP.BasicAuth{Username: "x-access-token", Password: "provider-secret"}
	clone := gitCloneOptions("https://github.com/example/template.git", "main", false, auth)
	if clone.RecurseSubmodules != git.NoRecurseSubmodules {
		t.Fatalf("clone recursion = %v, want NoRecurseSubmodules", clone.RecurseSubmodules)
	}
	if clone.Auth != auth {
		t.Fatal("clone did not retain the provider auth for the canonical top-level repository")
	}

	pull := gitPullOptions("main", auth)
	if pull.RecurseSubmodules != git.NoRecurseSubmodules {
		t.Fatalf("pull recursion = %v, want NoRecurseSubmodules", pull.RecurseSubmodules)
	}
	if pull.Auth != auth {
		t.Fatal("pull did not retain the provider auth for the canonical top-level repository")
	}
}

func TestPublicGeneratorGitOptionsDoNotSendEmptyBasicAuth(t *testing.T) {
	if auth := gitCloneOptions("https://github.com/example/public.git", "", false, nil).Auth; auth != nil {
		t.Fatalf("public clone auth = %#v, want nil", auth)
	}
	if auth := gitPullOptions("", nil).Auth; auth != nil {
		t.Fatalf("public pull auth = %#v, want nil", auth)
	}
}
