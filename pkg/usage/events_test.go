package usage

import (
	"testing"

	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
)

func TestRegistryPullEmitsEvent(t *testing.T) {
	logrus.SetLevel(logrus.InfoLevel)
	hook := logrustest.NewGlobal()
	defer hook.Reset()

	RegistryPull("manifest", "qnib/ftest", "latest", "sha256:abc", "", "", 200)

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("no log entry emitted")
	}
	if e.Data["event"] != EventPull {
		t.Errorf("event = %v, want %v", e.Data["event"], EventPull)
	}
	if e.Data["repo"] != "qnib/ftest" || e.Data["kind"] != "manifest" {
		t.Errorf("unexpected repo/kind: %+v", e.Data)
	}
	if e.Data["profile"] != "none" {
		t.Errorf("empty profile should normalise to none, got %v", e.Data["profile"])
	}
	if e.Data["user"] != "anonymous" {
		t.Errorf("empty user should normalise to anonymous, got %v", e.Data["user"])
	}
}

func TestAuthEmitsEvent(t *testing.T) {
	logrus.SetLevel(logrus.InfoLevel)
	hook := logrustest.NewGlobal()
	defer hook.Reset()

	Auth("alice", "zen2", "oidc", ResultSuccess)

	e := hook.LastEntry()
	if e == nil {
		t.Fatal("no log entry emitted")
	}
	if e.Data["event"] != EventAuth {
		t.Errorf("event = %v, want %v", e.Data["event"], EventAuth)
	}
	if e.Data["user"] != "alice" || e.Data["profile"] != "zen2" ||
		e.Data["method"] != "oidc" || e.Data["result"] != ResultSuccess {
		t.Errorf("unexpected auth fields: %+v", e.Data)
	}
}
